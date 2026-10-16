package mcpserver

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

var orderSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"order_id":       map[string]any{"type": "string"},
		"total":          map[string]any{"type": "number", "minimum": 0},
		"payment_status": map[string]any{"type": "string", "enum": []any{"paid", "pending"}},
		"customer":       map[string]any{"type": "object", "properties": map[string]any{"tier": map[string]any{"type": "string"}}},
	},
}

func TestSampleValueFollowsTheContract(t *testing.T) {
	payload := sampleValue(orderSchema, "", 0).(map[string]any)
	if payload["payment_status"] != "paid" || payload["order_id"] != "order-1001" || payload["total"] != 99.5 {
		t.Fatalf("sampleValue() = %v", payload)
	}
	if _, ok := payload["customer"].(map[string]any); !ok {
		t.Fatalf("nested objects should be sampled: %v", payload)
	}
	if idField(orderSchema) != "order_id" {
		t.Fatalf("idField() = %q, want order_id", idField(orderSchema))
	}
}

func TestSnippetsAreValidAndKeepTheKeyServerSide(t *testing.T) {
	payload := sampleValue(orderSchema, "", 0).(map[string]any)
	data := snippetData{APIURL: "https://console.example/api", EventType: "order.completed", IDField: "order_id", OccurredAt: "2026-10-16T10:00:00Z"}
	for _, language := range []string{"curl", "typescript", "go", "python"} {
		data.Function = functionName(data.EventType, language)
		code, err := renderSnippet(language, data, payload)
		if err != nil {
			t.Fatalf("renderSnippet(%s) error = %v", language, err)
		}
		if !strings.Contains(code, "MOTIF_API_KEY") || !strings.Contains(code, "order.completed:") {
			t.Fatalf("%s snippet must read the key from the environment and derive the event id:\n%s", language, code)
		}
		if language == "go" {
			if _, err := parser.ParseFile(token.NewFileSet(), "snippet.go", code, parser.AllErrors); err != nil {
				t.Fatalf("Go snippet does not parse: %v\n%s", err, code)
			}
		}
		if language == "curl" {
			continue
		}
		example := code[strings.Index(code, "Example payload"):]
		for _, line := range strings.Split(strings.TrimSpace(example), "\n")[1:] {
			if !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") {
				t.Fatalf("%s example line escaped its comment: %q", language, line)
			}
		}
	}
	if functionName("order.completed", "python") != "send_order_completed" || functionName("order.completed", "go") != "SendOrderCompleted" {
		t.Fatal("function names should follow each language's convention")
	}
}
