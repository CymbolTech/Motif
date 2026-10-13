package rewards

import (
	"testing"
	"time"
)

func TestReconciliationBackoffIsBounded(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: time.Minute},
		{attempt: 1, want: 5 * time.Minute},
		{attempt: 2, want: 15 * time.Minute},
		{attempt: 3, want: time.Hour},
		{attempt: 99, want: time.Hour},
	}
	for _, tt := range tests {
		if got := reconciliationBackoff(tt.attempt); got != tt.want {
			t.Errorf("reconciliationBackoff(%d) = %s, want %s", tt.attempt, got, tt.want)
		}
	}
}
