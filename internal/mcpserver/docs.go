package mcpserver

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed content/*.md
var content embed.FS

type docTopic struct {
	Name        string
	Title       string
	Description string
}

var docTopics = []docTopic{
	{"overview", "MotifEngine overview", "What the engine does, its concepts, and who does what"},
	{"quickstart", "Add MotifEngine to an application", "Step by step, naming the tool for each step"},
	{"events", "Events, idempotency and errors", "Sending events, event IDs, retries, error handling and contracts"},
	{"rule-language", "Rule language", "Conditions, actions, versions, publishing and schedule windows"},
	{"progression", "XP, levels, missions, achievements and streaks", "Progression features and how they relate to rules"},
	{"loyalty", "Points, ledger and rewards", "Currencies, the ledger, refunds, conversion, rewards and redemptions"},
	{"leaderboards", "Leaderboards", "Weekly, monthly and hand-run rankings of points or XP earned per round"},
	{"measurement", "Measurement", "Goals, campaigns, metrics, funnels, retention, segments and experiments"},
	{"recipes", "Recipes", "Common product needs mapped to events, rules and catalog"},
	{"boundaries", "Boundaries", "What the engine deliberately does not do"},
	{"launch-checklist", "Launch checklist", "What to verify before relying on an integration"},
	{"integration-api", "Integration API reference", "The REST routes a product backend calls with an access key"},
}

type readDocsInput struct {
	Topic string `json:"topic" jsonschema:"The documentation topic to read"`
}

var readDocsSchema = func() any {
	names := make([]string, len(docTopics))
	for index, topic := range docTopics {
		names[index] = topic.Name
	}
	return inputSchema[readDocsInput](map[string][]string{"topic": names})
}()

func docText(name string) (string, error) {
	text, err := content.ReadFile("content/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("unknown topic %q", name)
	}
	return string(text), nil
}

func docURI(name string) string { return "motif://docs/" + name }

func registerDocs(server *mcp.Server) {
	var index strings.Builder
	for _, topic := range docTopics {
		fmt.Fprintf(&index, "%s (%s); ", topic.Name, topic.Description)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_docs",
		Description: "Read MotifEngine documentation. Topics: " + strings.TrimSuffix(index.String(), "; ") + ".",
		Annotations: readOnly("Read documentation"),
		InputSchema: readDocsSchema,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input readDocsInput) (*mcp.CallToolResult, any, error) {
		text, err := docText(input.Topic)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	for _, topic := range docTopics {
		name := topic.Name
		server.AddResource(&mcp.Resource{
			URI: docURI(name), Name: name, Title: topic.Title, Description: topic.Description, MIMEType: "text/markdown",
		}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			text, err := docText(name)
			if err != nil {
				return nil, mcp.ResourceNotFoundError(request.Params.URI)
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: docURI(name), MIMEType: "text/markdown", Text: text}}}, nil
		})
	}
}
