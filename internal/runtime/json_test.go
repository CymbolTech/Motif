package runtime

import (
	"strings"
	"testing"
)

func TestDecodeJSONRequiresOneValue(t *testing.T) {
	var payload map[string]string
	if err := DecodeJSON(strings.NewReader(`{"ok":"yes"}`), &payload); err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if payload["ok"] != "yes" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDecodeJSONRejectsTrailingValue(t *testing.T) {
	var payload map[string]string
	if err := DecodeJSON(strings.NewReader(`{"ok":"yes"}{"extra":"value"}`), &payload); err == nil {
		t.Fatal("DecodeJSON() error = nil, want trailing value error")
	}
}
