// Package quiet is a pure unit tracking whether a session's GRID content has
// stopped changing — the deadline-driven half of noticing a session that
// finished or got stuck without being told (#38). It mirrors internal/idle
// and internal/power: a Policy plus a mutex-guarded Tracker, every method
// takes `now` explicitly so callers arm real timers via deadline.go and
// tests never sleep.
package quiet

import (
	"sync"
	"time"
)

// Policy configures how long content must stay unchanged before a session
// counts as quiet.
type Policy struct {
	After time.Duration
}

// Tracker is per-session state: when its content last changed, what that
// content was, and whether an alert already fired for the current quiet
// period. Due/Consume are split rather than one Fire() because suppression
// (screen state, ;notify 0) is Server-wide and this package can't see it —
// the caller decides whether a Due tracker actually gets to Consume.
type Tracker struct {
	policy Policy

	mu      sync.Mutex
	last    time.Time
	content string
	fired   bool
}

func NewTracker(p Policy, now time.Time) *Tracker {
	return &Tracker{policy: p, last: now}
}

// Contact records one observation. Only a CHANGE in content resets the clock
// and clears fired — an unchanged observation (e.g. re-checked only because
// the status row's spinner ticked) leaves the clock running toward Due.
func (t *Tracker) Contact(now time.Time, content string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if content == t.content {
		return
	}
	t.content = content
	t.last = now
	t.fired = false
}

func (t *Tracker) dormant() bool {
	return t.policy.After <= 0 || t.fired
}

// Until mirrors idle.Until/power.Until: remaining time before Due, or 0 if
// disabled, already due, or already fired this period.
func (t *Tracker) Until(now time.Time) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dormant() {
		return 0
	}
	left := t.policy.After - now.Sub(t.last)
	if left < 0 {
		return 0
	}
	return left
}

// Due reports whether content has been unchanged for at least Policy.After
// and no alert has fired for this quiet period yet.
func (t *Tracker) Due(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dormant() {
		return false
	}
	return now.Sub(t.last) >= t.policy.After
}

// Consume marks the current quiet period as answered. Due returns false
// again until a subsequent Contact sees genuinely different content.
func (t *Tracker) Consume(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fired = true
}
