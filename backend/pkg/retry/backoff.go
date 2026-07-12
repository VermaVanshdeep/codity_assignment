// Package retry provides configurable backoff strategies for job retry scheduling.
//
// Three strategies are supported:
//   - Fixed:       constant delay regardless of attempt number
//   - Linear:      delay grows linearly with attempt number
//   - Exponential: delay doubles each attempt, with optional jitter to prevent
//     thundering herd when many jobs fail simultaneously
package retry

import (
	"math"
	"math/rand"
	"time"
)

// Strategy defines how retry delays are computed.
type Strategy string

const (
	StrategyFixed       Strategy = "fixed"
	StrategyLinear      Strategy = "linear"
	StrategyExponential Strategy = "exponential"
)

// Policy describes the full retry configuration for a job.
type Policy struct {
	MaxRetries int
	Strategy   Strategy
	BaseDelay  time.Duration // delay unit (e.g. 60s)
	MaxDelay   time.Duration // cap for exponential (default: 24h)
	Jitter     bool          // add random jitter to exponential
}

// DefaultPolicy returns a sensible default retry policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxRetries: 3,
		Strategy:   StrategyExponential,
		BaseDelay:  60 * time.Second,
		MaxDelay:   24 * time.Hour,
		Jitter:     true,
	}
}

// NextDelay computes the wait duration before the next retry attempt.
// attempt is 1-indexed (first retry = attempt 1).
func NextDelay(p Policy, attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	var delay time.Duration
	switch p.Strategy {
	case StrategyFixed:
		delay = p.BaseDelay

	case StrategyLinear:
		delay = p.BaseDelay * time.Duration(attempt)

	case StrategyExponential:
		// 2^(attempt-1) * baseDelay
		exp := math.Pow(2, float64(attempt-1))
		delay = time.Duration(float64(p.BaseDelay) * exp)

		if p.Jitter && delay > 0 {
			// Add ±25% jitter
			jitter := time.Duration(rand.Int63n(int64(delay) / 2)) //nolint:gosec
			if rand.Intn(2) == 0 {                                 //nolint:gosec
				delay += jitter
			} else {
				delay -= jitter
			}
		}

	default:
		delay = p.BaseDelay
	}

	// Apply max cap
	maxDelay := p.MaxDelay
	if maxDelay == 0 {
		maxDelay = 24 * time.Hour
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	if delay < 0 {
		delay = p.BaseDelay
	}

	return delay
}

// NextRunAt returns the absolute time for the next retry.
func NextRunAt(p Policy, attempt int, now time.Time) time.Time {
	return now.Add(NextDelay(p, attempt))
}
