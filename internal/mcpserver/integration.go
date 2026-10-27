package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type snippetInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	EventType   string `json:"event_type" jsonschema:"The event type to send, ideally with a registered contract"`
	Language    string `json:"language" jsonschema:"The backend language"`
}

var snippetSchema = inputSchema[snippetInput](map[string][]string{"language": {"curl", "typescript", "go", "python"}})

type snippetData struct {
	APIURL      string
	EventType   string
	Function    string
	IDField     string
	PayloadJSON string
	OccurredAt  string
	Example     string
}

func registerIntegrationTools(server *mcp.Server, session *Session) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_integration_snippet",
		Description: "Generate server-side code that sends one event type to MotifEngine from the product's backend: a payload shaped by the registered contract, an event_id derived from the business fact, the access key read from a server-side secret, and retries that keep the same event_id.",
		Annotations: readOnly("Integration code"),
		InputSchema: snippetSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input snippetInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		var contracts []map[string]any
		if err := session.Client.Do(ctx, "GET", "/v1/admin/event-types", session.Scope(environment), nil, &contracts); err != nil {
			return nil, nil, err
		}
		var schema map[string]any
		for _, contract := range contracts {
			if contract["name"] == input.EventType {
				schema, _ = contract["schema"].(map[string]any)
			}
		}
		notes := []string{
			"Send from the server at the moment the fact becomes true (payment captured, lesson graded), never from a browser or mobile app.",
			"Set MOTIF_API_URL and MOTIF_API_KEY in the server environment. MOTIF_API_KEY is an access key of the " + environment + " environment with the write scope, kept in the secret store; this server cannot read it.",
			"The event_id must identify the business fact so a retry reuses it. Adjust the derivation if the chosen field is not unique per fact.",
			"Retry 5xx, 429 and network errors with the same event_id. Stop on 409 (event_id reused with different content) and 422 (payload breaks the contract).",
		}
		if schema == nil {
			notes = append([]string{fmt.Sprintf("%s has no registered contract in %s, so the payload below is a placeholder. Register one with register_event_type so malformed payloads are rejected.", input.EventType, environment)}, notes...)
			schema = map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}}
		}
		payload, _ := sampleValue(schema, "", 0).(map[string]any)
		code, err := renderSnippet(input.Language, snippetData{
			APIURL:     session.PublicAPIURL,
			EventType:  input.EventType,
			Function:   functionName(input.EventType, input.Language),
			IDField:    idField(schema),
			OccurredAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		}, payload)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "language": input.Language, "code": code, "environment_variables": map[string]string{"MOTIF_API_URL": session.PublicAPIURL, "MOTIF_API_KEY": "an access key of " + environment + " with the write scope"}, "notes": notes}, nil
	})
}

func renderSnippet(language string, data snippetData, payload map[string]any) (string, error) {
	text, ok := snippetTemplates[language]
	if !ok {
		return "", fmt.Errorf("unsupported language %q; use curl, typescript, go or python", language)
	}
	indent := map[string]string{"curl": "  ", "typescript": "  ", "go": "\t", "python": "    "}[language]
	encoded, err := json.MarshalIndent(payload, indent, "  ")
	if err != nil {
		return "", err
	}
	data.PayloadJSON = string(encoded)
	plain, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	switch language {
	case "typescript":
		data.Example = commented("// ", fmt.Sprintf("await %s('user-42', %s);", data.Function, plain))
	case "go":
		data.Example = commented("//\t", fmt.Sprintf("err := %s(ctx, \"user-42\", %s, time.Now())", data.Function, goMapLiteral(payload, 1)))
	case "python":
		data.Example = commented("# ", fmt.Sprintf("%s(\"user-42\", %s)", data.Function, pythonLiteral(payload, 0)))
	}
	var out bytes.Buffer
	if err := template.Must(template.New(language).Parse(text)).Execute(&out, data); err != nil {
		return "", err
	}
	// The event_id expression sits inside the language's own interpolation
	// braces, which would collide with template delimiters, so it is spliced in last.
	return strings.ReplaceAll(out.String(), "__EVENT_KEY__", eventKeyExpression(language, data.IDField)), nil
}

func eventKeyExpression(language, field string) string {
	if field == "" {
		return map[string]string{"typescript": "'<business fact id>'", "python": "'<business fact id>'", "go": `"<business fact id>"`}[language]
	}
	return map[string]string{"typescript": "String(payload." + field + ")", "python": "payload['" + field + "']", "go": `payload["` + field + `"]`}[language]
}

// sampleValue builds a realistic example from the contract's schema subset.
func sampleValue(schema map[string]any, name string, depth int) any {
	if values, ok := schema["enum"].([]any); ok && len(values) > 0 {
		return values[0]
	}
	switch schema["type"] {
	case "object":
		object := map[string]any{}
		properties, _ := schema["properties"].(map[string]any)
		if depth > 6 {
			return object
		}
		for key, value := range properties {
			if child, ok := value.(map[string]any); ok {
				object[key] = sampleValue(child, key, depth+1)
			}
		}
		return object
	case "integer":
		if minimum, ok := number(schema["minimum"]); ok && minimum > 1 {
			return int64(minimum)
		}
		return 1
	case "number":
		if minimum, ok := number(schema["minimum"]); ok && minimum > 99.5 {
			return minimum
		}
		return 99.5
	case "boolean":
		return true
	case "array":
		return []any{}
	default:
		return exampleString(name)
	}
}

func exampleString(name string) string {
	lower := strings.ToLower(name)
	switch {
	case lower == "":
		return "example"
	case strings.HasSuffix(lower, "_id") || lower == "id":
		return strings.ReplaceAll(strings.TrimSuffix(lower, "_id"), "_", "-") + "-1001"
	case strings.Contains(lower, "currency"):
		return "USD"
	case strings.Contains(lower, "email"):
		return "user@example.com"
	case strings.Contains(lower, "status"):
		return "completed"
	case strings.HasSuffix(lower, "_at") || strings.Contains(lower, "date"):
		return "2026-10-16T10:00:00Z"
	default:
		return "example-" + strings.ReplaceAll(lower, "_", "-")
	}
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case float64:
		return typed, true
	}
	return 0, false
}

// idField picks the payload field that most likely identifies the business fact.
func idField(schema map[string]any) string {
	properties, _ := schema["properties"].(map[string]any)
	candidates := []string{}
	for key := range properties {
		if strings.HasSuffix(key, "_id") || key == "id" {
			candidates = append(candidates, key)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if (candidates[i] == "user_id") != (candidates[j] == "user_id") {
			return candidates[j] == "user_id"
		}
		return candidates[i] < candidates[j]
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func functionName(eventType, language string) string {
	words := strings.FieldsFunc(eventType, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if language == "python" {
		return "send_" + strings.ToLower(strings.Join(words, "_"))
	}
	var name strings.Builder
	name.WriteString("send")
	if language == "go" {
		name.Reset()
		name.WriteString("Send")
	}
	for _, word := range words {
		if word == "" {
			continue
		}
		name.WriteString(strings.ToUpper(word[:1]) + strings.ToLower(word[1:]))
	}
	return name.String()
}

// commented prefixes every line of an example so it stays a comment in the generated code.
func commented(prefix, text string) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(prefix+line, " \t")
	}
	return strings.Join(lines, "\n")
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func goMapLiteral(object map[string]any, depth int) string {
	var out strings.Builder
	out.WriteString("map[string]any{\n")
	for _, key := range sortedKeys(object) {
		out.WriteString(strings.Repeat("\t", depth) + fmt.Sprintf("%q: %s,\n", key, goValue(object[key], depth+1)))
	}
	out.WriteString(strings.Repeat("\t", depth-1) + "}")
	return out.String()
}

func goValue(value any, depth int) string {
	switch typed := value.(type) {
	case map[string]any:
		return goMapLiteral(typed, depth)
	case []any:
		return "[]any{}"
	case string:
		return fmt.Sprintf("%q", typed)
	default:
		return fmt.Sprint(typed)
	}
}

func pythonLiteral(object map[string]any, depth int) string {
	var out strings.Builder
	out.WriteString("{\n")
	for _, key := range sortedKeys(object) {
		out.WriteString(strings.Repeat("    ", depth+1) + fmt.Sprintf("%q: %s,\n", key, pythonValue(object[key], depth+1)))
	}
	out.WriteString(strings.Repeat("    ", depth) + "}")
	return out.String()
}

func pythonValue(value any, depth int) string {
	switch typed := value.(type) {
	case map[string]any:
		return pythonLiteral(typed, depth)
	case []any:
		return "[]"
	case string:
		return fmt.Sprintf("%q", typed)
	case bool:
		if typed {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprint(typed)
	}
}

var snippetTemplates = map[string]string{
	"curl": `# Send one {{.EventType}} event. Run server-side; the key never belongs in a client.
# Reuse the same event_id when retrying the same business fact.
curl -X POST "${MOTIF_API_URL:-{{.APIURL}}}/v1/events" \
  -H "X-Motif-API-Key: $MOTIF_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
  "event_id": "{{.EventType}}:{{if .IDField}}<{{.IDField}}>{{else}}<business fact id>{{end}}",
  "event_type": "{{.EventType}}",
  "user_id": "<your stable user id>",
  "occurred_at": "{{.OccurredAt}}",
  "payload": {{.PayloadJSON}}
}'
`,
	"typescript": `// Server-side only: the access key must never reach a browser or mobile app.
const MOTIF_API_URL = process.env.MOTIF_API_URL ?? '{{.APIURL}}';
const MOTIF_API_KEY = process.env.MOTIF_API_KEY;

export async function {{.Function}}(userId: string, payload: Record<string, unknown>, occurredAt = new Date()) {
  if (!MOTIF_API_KEY) throw new Error('MOTIF_API_KEY is not set');
  // One business fact, one event_id: retries reuse it, so the engine never counts twice.
  const eventId = ` + "`" + `{{.EventType}}:${__EVENT_KEY__}` + "`" + `;
  const body = JSON.stringify({ event_id: eventId, event_type: '{{.EventType}}', user_id: userId, occurred_at: occurredAt.toISOString(), payload });

  for (let attempt = 1; ; attempt++) {
    let response: Response | undefined;
    try {
      response = await fetch(` + "`" + `${MOTIF_API_URL}/v1/events` + "`" + `, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Motif-API-Key': MOTIF_API_KEY },
        body,
      });
    } catch (error) {
      if (attempt >= 5) throw error;
    }
    if (response?.status === 202) return response.json();
    const retryable = !response || response.status === 429 || response.status >= 500;
    if (!retryable || attempt >= 5) {
      throw new Error(` + "`" + `MotifEngine rejected ${eventId}: ${response?.status} ${await response?.text()}` + "`" + `);
    }
    await new Promise((resolve) => setTimeout(resolve, 250 * 2 ** attempt));
  }
}

// Example payload from the registered contract:
{{.Example}}
`,
	"go": `package motif

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// {{.Function}} sends one {{.EventType}} event from the server. The integration
// key lives in the server environment and never reaches a client.
func {{.Function}}(ctx context.Context, userID string, payload map[string]any, occurredAt time.Time) error {
	apiURL := os.Getenv("MOTIF_API_URL")
	if apiURL == "" {
		apiURL = "{{.APIURL}}"
	}
	// One business fact, one event_id: retries reuse it, so the engine never counts twice.
	eventID := fmt.Sprintf("{{.EventType}}:%v", __EVENT_KEY__)
	body, err := json.Marshal(map[string]any{
		"event_id":    eventID,
		"event_type":  "{{.EventType}}",
		"user_id":     userID,
		"occurred_at": occurredAt.UTC().Format(time.RFC3339),
		"payload":     payload,
	})
	if err != nil {
		return err
	}
	for attempt := 1; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/v1/events", bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Motif-API-Key", os.Getenv("MOTIF_API_KEY"))
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			detail, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode == http.StatusAccepted {
				return nil
			}
			if response.StatusCode != http.StatusTooManyRequests && response.StatusCode < 500 {
				return fmt.Errorf("MotifEngine rejected %s: %d %s", eventID, response.StatusCode, detail)
			}
			err = fmt.Errorf("MotifEngine returned %d", response.StatusCode)
		}
		if attempt >= 5 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(250<<attempt) * time.Millisecond):
		}
	}
}

// Example payload from the registered contract:
//
{{.Example}}
`,
	"python": `import os
import time
from datetime import datetime, timezone

import requests

MOTIF_API_URL = os.environ.get("MOTIF_API_URL", "{{.APIURL}}")


def {{.Function}}(user_id: str, payload: dict, occurred_at: datetime | None = None) -> dict:
    """Send one {{.EventType}} event from the server; the key never reaches a client."""
    # One business fact, one event_id: retries reuse it, so the engine never counts twice.
    event_id = f"{{.EventType}}:{__EVENT_KEY__}"
    body = {
        "event_id": event_id,
        "event_type": "{{.EventType}}",
        "user_id": user_id,
        "occurred_at": (occurred_at or datetime.now(timezone.utc)).isoformat(),
        "payload": payload,
    }
    for attempt in range(1, 6):
        try:
            response = requests.post(
                f"{MOTIF_API_URL}/v1/events",
                json=body,
                headers={"X-Motif-API-Key": os.environ["MOTIF_API_KEY"]},
                timeout=10,
            )
        except requests.RequestException:
            if attempt == 5:
                raise
        else:
            if response.status_code == 202:
                return response.json()
            if response.status_code != 429 and response.status_code < 500:
                raise RuntimeError(f"MotifEngine rejected {event_id}: {response.status_code} {response.text}")
            if attempt == 5:
                response.raise_for_status()
        time.sleep(0.25 * 2**attempt)
    raise RuntimeError("unreachable")


# Example payload from the registered contract:
{{.Example}}
`,
}
