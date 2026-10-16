package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ClientHeader marks requests from this server so audit entries can name the client.
const ClientHeader = "X-Motif-Client"

// Client calls the MotifEngine admin API with the caller's access token. The
// API authorizes every request itself; this server never widens access.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: httpClient}
}

// APIError is an error response from the API, with a hint for the assistant
// on how to recover.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	text := fmt.Sprintf("MotifEngine API returned %d %s: %s", e.Status, e.Code, e.Message)
	if hint := e.hint(); hint != "" {
		text += ". " + hint
	}
	return text
}

func (e *APIError) hint() string {
	switch {
	case e.Status == http.StatusUnauthorized:
		return "The access token is invalid, expired or revoked; ask a workspace administrator for a new one from Access tokens in the console"
	case e.Status == http.StatusForbidden:
		return "This token may lack the permission (write or publish) or the environment this needs; do not retry with the same token"
	case e.Status == http.StatusTooManyRequests:
		return "Wait a second before retrying"
	case e.Code == "plan_limit_exceeded":
		return "The workspace plan limit is reached; reuse or deactivate an existing resource instead"
	case e.Status >= 500:
		return "The engine had a temporary problem; retrying the same call is safe"
	}
	return ""
}

// Get reads a resource and returns it with workspace identifiers removed.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (any, error) {
	var value any
	if err := c.Do(ctx, http.MethodGet, path, query, nil, &value); err != nil {
		return nil, err
	}
	return Scrub(value), nil
}

// Send writes a resource and returns the response with workspace identifiers removed.
func (c *Client) Send(ctx context.Context, method, path string, query url.Values, body any) (any, error) {
	var value any
	if err := c.Do(ctx, method, path, query, body, &value); err != nil {
		return nil, err
	}
	return Scrub(value), nil
}

// Do performs a request and decodes the JSON response into out.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set(ClientHeader, "mcp")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("MotifEngine API is unreachable: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode >= 400 {
		apiErr := &APIError{Status: response.StatusCode, Code: http.StatusText(response.StatusCode), Message: strings.TrimSpace(string(payload))}
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(payload, &envelope) == nil && envelope.Error.Code != "" {
			apiErr.Code, apiErr.Message = envelope.Error.Code, envelope.Error.Message
		}
		return apiErr
	}
	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	return decoder.Decode(out)
}

// Scrub removes workspace identifiers from API output. An access token is
// bound to one workspace, so the identifier adds nothing for the assistant.
func Scrub(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "tenant_id" || key == "tenant_ids" {
				delete(typed, key)
				continue
			}
			typed[key] = Scrub(item)
		}
		return typed
	case []any:
		for index, item := range typed {
			typed[index] = Scrub(item)
		}
		return typed
	default:
		return value
	}
}
