// Package clock provides a testable abstraction over time.Now().
//
// Design rationale: Hard-coding time.Now() in business logic makes it impossible
// to test time-sensitive code (scheduling, TTL checks, retries) deterministically.
// By injecting a Clock interface, tests can use a fixed or mock clock.
package clock

import "time"

// Clock is an interface for time-related operations.
// All services that depend on the current time should accept a Clock.
type Clock interface {
	// Now returns the current time in UTC.
	Now() time.Time
}

// Real is the production Clock backed by the system clock.
type Real struct{}

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// ─── Test Helpers ──────────────────────────────────────────────────────────────

// Fixed is a Clock that always returns the same time. Use in unit tests.
type Fixed struct {
	T time.Time
}

// Now returns the fixed time.
func (f Fixed) Now() time.Time { return f.T }

// NewFixed creates a Fixed clock at the given time.
func NewFixed(t time.Time) Fixed { return Fixed{T: t} }

// ─── Global Default ────────────────────────────────────────────────────────────

// Default is the application-wide real clock. Used as the zero-value default
// when a Clock is not injected.
var Default Clock = Real{}
