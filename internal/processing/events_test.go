package processing

import "testing"

func TestDecodeActionsSupportsLegacyAndMultiActionDefinitions(t *testing.T) {
	legacy, err := decodeActions([]byte(`{"type":"award_xp","amount":10}`))
	if err != nil {
		t.Fatalf("decodeActions() legacy error = %v", err)
	}
	if len(legacy) != 1 || legacy[0].Type != "award_xp" {
		t.Fatalf("legacy actions = %+v", legacy)
	}

	multi, err := decodeActions([]byte(`[
        {"type":"award_xp","amount":10},
        {"type":"award_points","currency":"coins","amount":5}
    ]`))
	if err != nil {
		t.Fatalf("decodeActions() multi error = %v", err)
	}
	if len(multi) != 2 || multi[0].Type != "award_xp" || multi[1].Type != "award_points" {
		t.Fatalf("multi actions = %+v", multi)
	}
}

func TestDecodeActionsRejectsInvalidDefinitions(t *testing.T) {
	for _, raw := range []string{`[]`, `[{"type":"award_xp","amount":10},]`, `{"type":true}`} {
		if _, err := decodeActions([]byte(raw)); err == nil {
			t.Fatalf("decodeActions(%s) returned nil error", raw)
		}
	}
}

func TestActionIdempotencyKeyIncludesOrdinal(t *testing.T) {
	if got := actionIdempotencyKey(42, "rule:v1", 1); got != "event:42:rule:rule:v1:action:1" {
		t.Fatalf("action idempotency key = %q", got)
	}
}

func TestDecodeActionsPreservesNumericAmounts(t *testing.T) {
	actions, err := decodeActions([]byte(`[{"type":"award_xp","amount":9000000000000000000}]`))
	if err != nil {
		t.Fatalf("decodeActions() error = %v", err)
	}
	if actions[0].Amount.String() != "9000000000000000000" {
		t.Fatalf("amount = %s", actions[0].Amount)
	}
}
