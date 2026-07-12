package retry_test

import (
	"testing"
	"time"

	"github.com/your-org/job-scheduler/pkg/retry"
)

func TestNextDelay(t *testing.T) {
	tests := []struct {
		name     string
		policy   retry.Policy
		attempt  int
		expected time.Duration
		jitter   bool
	}{
		{
			name: "fixed strategy",
			policy: retry.Policy{
				Strategy:  retry.StrategyFixed,
				BaseDelay: 5 * time.Second,
			},
			attempt:  3,
			expected: 5 * time.Second,
		},
		{
			name: "linear strategy",
			policy: retry.Policy{
				Strategy:  retry.StrategyLinear,
				BaseDelay: 5 * time.Second,
			},
			attempt:  3,
			expected: 15 * time.Second,
		},
		{
			name: "exponential without jitter",
			policy: retry.Policy{
				Strategy:  retry.StrategyExponential,
				BaseDelay: 2 * time.Second,
				Jitter:    false,
			},
			attempt:  4,
			expected: 16 * time.Second, // 2 * 2^(4-1) = 16
		},
		{
			name: "max delay cap",
			policy: retry.Policy{
				Strategy:  retry.StrategyExponential,
				BaseDelay: 10 * time.Second,
				MaxDelay:  30 * time.Second,
				Jitter:    false,
			},
			attempt:  10,
			expected: 30 * time.Second, // cap applies
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := retry.NextDelay(tt.policy, tt.attempt)
			if tt.jitter {
				// with jitter it's hard to test exact, skip for now in struct slice
			} else {
				if delay != tt.expected {
					t.Errorf("NextDelay() = %v, want %v", delay, tt.expected)
				}
			}
		})
	}
}

func TestNextDelay_Jitter(t *testing.T) {
	policy := retry.Policy{
		Strategy:  retry.StrategyExponential,
		BaseDelay: 10 * time.Second,
		Jitter:    true,
	}

	// 10 * 2^1 = 20s. Jitter is +/- 25% (up to 5s).
	// Range: 15s - 25s
	delay := retry.NextDelay(policy, 2)
	if delay < 15*time.Second || delay > 25*time.Second {
		t.Errorf("NextDelay() with jitter out of bounds: %v (expected 15s-25s)", delay)
	}
}
