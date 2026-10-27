// Package mcpserver exposes MotifEngine to MCP clients for configuring and
// inspecting a workspace. It is a thin client of the admin API: every call
// carries the caller's access key, and the API enforces the key's
// workspace, environment and scopes. Events from a product backend keep
// flowing over REST; this server is never that path.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Options struct {
	APIURL       string
	ConsoleURL   string
	PublicAPIURL string
	Version      string
	Logger       *slog.Logger
	HTTPClient   *http.Client
}

// NewServer builds the MCP server for one session. It only registers the tools
// the key's scopes allow: read inspects, write creates, edit changes what
// exists (including publishing rules), and delete removes.
func NewServer(session *Session, opts Options, cache *mcp.SchemaCache) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "motifengine", Title: "MotifEngine", Version: opts.Version}, &mcp.ServerOptions{
		Instructions: instructions(session),
		Logger:       opts.Logger,
		SchemaCache:  cache,
	})
	registerDocs(server)
	registerInspectTools(server, session)
	registerIntegrationTools(server, session)
	if session.Can(scopeWrite) {
		registerConfigureTools(server, session)
	}
	if session.Can(scopeEdit) {
		registerEditTools(server, session)
	}
	if session.Can(scopeDelete) {
		registerLeaderboardDeleteTool(server, session)
	}
	registerPrompts(server, session)
	return server
}

func instructions(session *Session) string {
	environments := make([]string, 0, len(session.Environments))
	for _, environment := range session.Environments {
		environments = append(environments, environment.ID)
	}
	return fmt.Sprintf(`MotifEngine is an event-driven engagement and loyalty engine. A product backend sends trusted events over REST with an access key; published rules turn them into XP, points and log entries; missions and achievements track progress; leaderboards rank what users earn per round; rewards are redeemed by the product backend.

This server configures and inspects the %q workspace (environments: %s; permissions: %s). Start with get_workspace, and read_docs when unsure. Rules read only the current event payload. Run check_rule with realistic sample payloads before saving a draft, ask the human before publish_rule, and prove changes with send_test_event and get_event_trace. Test events award real points in their environment, so prefer a non-production environment and test user ids. Ask before ending a leaderboard round or deleting a leaderboard; neither can be undone. Never put an access key in client-side code.`,
		session.Workspace, strings.Join(environments, ", "), strings.Join(session.Scopes, ", "))
}

// NewHTTPHandler serves the MCP endpoint over stateless Streamable HTTP with
// JSON responses. Requests need an access key with the MCP scope as a bearer token.
func NewHTTPHandler(opts Options) http.Handler {
	resolver := &Resolver{APIURL: opts.APIURL, ConsoleURL: opts.ConsoleURL, PublicAPIURL: opts.PublicAPIURL, HTTPClient: opts.HTTPClient}
	cache := mcp.NewSchemaCache()
	streamable := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		session := sessionFromTokenInfo(auth.TokenInfoFromContext(r.Context()))
		if session == nil {
			return nil
		}
		return NewServer(session, opts, cache)
	}, &mcp.StreamableHTTPOptions{
		Stateless:             true,
		JSONResponse:          true,
		Logger:                opts.Logger,
		CrossOriginProtection: http.NewCrossOriginProtection(),
	})
	return auth.RequireBearerToken(resolver.Verify, nil)(streamable)
}

// RunStdio serves one session over standard input and output, for clients
// that launch the server as a local process.
func RunStdio(ctx context.Context, opts Options, token string) error {
	resolver := &Resolver{APIURL: opts.APIURL, ConsoleURL: opts.ConsoleURL, PublicAPIURL: opts.PublicAPIURL, HTTPClient: opts.HTTPClient}
	session, err := resolver.Resolve(ctx, token)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			return fmt.Errorf("MOTIF_ACCESS_KEY was rejected: %w", err)
		}
		return err
	}
	return NewServer(session, opts, nil).Run(ctx, &mcp.StdioTransport{})
}
