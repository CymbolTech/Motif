package entitlements

import (
	"errors"
	"testing"
)

func TestDecodeLimitsAndOverrideMerge(t *testing.T) {
	base, err := decodeLimits([]byte(`{"max_rules": 10, "max_rewards": 20}`))
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := decodeLimits([]byte(`{"max_rules": 3}`))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range overrides {
		base[key] = value
	}
	if base["max_rules"] != 3 || base["max_rewards"] != 20 {
		t.Fatalf("unexpected merged limits: %#v", base)
	}
}

func TestDecodeLimitsRejectsNegativeAndNonInteger(t *testing.T) {
	for _, raw := range []string{`{"max_rules": -1}`, `{"max_rules": 1.5}`, `{"max_rules": "10"}`} {
		if _, err := decodeLimits([]byte(raw)); err == nil {
			t.Fatalf("decodeLimits(%s) error = nil", raw)
		}
	}
}

func TestLimitErrorUnwraps(t *testing.T) {
	err := &LimitError{Key: "max_rules", Limit: 2, Current: 2, Want: 1}
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatal("LimitError should unwrap to ErrLimitExceeded")
	}
}

func TestValidateLimitsRejectsUnknownKeys(t *testing.T) {
	if err := ValidateLimits(map[string]int64{"max_rules": 10}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLimits(map[string]int64{"future_feature": 1}); err == nil {
		t.Fatal("ValidateLimits() error = nil, want unsupported key error")
	}
}
