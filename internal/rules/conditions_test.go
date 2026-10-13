package rules

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMatchConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions string
		payload    string
		want       bool
	}{
		{name: "empty object matches", conditions: `{}`, payload: `{}`, want: true},
		{name: "path equality", conditions: `{"eq":["$.order.status","paid"]}`, payload: `{"order":{"status":"paid"}}`, want: true},
		{name: "numeric comparison", conditions: `{"gte":["$.order.total",100]}`, payload: `{"order":{"total":125}}`, want: true},
		{name: "all and any", conditions: `{"all":[{"exists":"$.user"},{"any":[{"eq":["$.kind","purchase"]},{"eq":["$.kind","refund"]}]}]}`, payload: `{"user":"u-1","kind":"refund"}`, want: true},
		{name: "missing value", conditions: `{"eq":["$.missing",1]}`, payload: `{}`, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matched, err := MatchConditions(json.RawMessage(test.conditions), json.RawMessage(test.payload))
			if err != nil {
				t.Fatalf("MatchConditions() error = %v", err)
			}
			if matched != test.want {
				t.Fatalf("MatchConditions() = %v, want %v", matched, test.want)
			}
		})
	}
}

func TestValidateConditionsRejectsUnsafeOrUnboundedInput(t *testing.T) {
	for _, raw := range []string{
		`{"script":"return true"}`,
		`{"eq":["$order.total",1]}`,
		`{"eq":[{"nested":true},1]}`,
	} {
		if _, err := ValidateConditions(json.RawMessage(raw)); err == nil {
			t.Fatalf("ValidateConditions(%s) returned nil error", raw)
		}
	}
	deep := `{}`
	for index := 0; index < maxDepth+2; index++ {
		deep = fmt.Sprintf(`{"all":[%s]}`, deep)
	}
	if _, err := ValidateConditions(json.RawMessage(deep)); err == nil {
		t.Fatal("ValidateConditions() accepted deeply nested conditions")
	}
}
