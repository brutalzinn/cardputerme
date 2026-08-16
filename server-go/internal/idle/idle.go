package idle

import (
	"sync"
	"time"
)

type Policy struct {
	After time.Duration
}

type Tracker struct {
	policy Policy

	mu      sync.Mutex
	last    time.Time
	clients int
}

func NewTracker(p Policy, now time.Time) *Tracker {
	return &Tracker{policy: p, last: now}
}

func (t *Tracker) Connect(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.clients++
	t.last = now
}

func (t *Tracker) Disconnect(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.clients > 0 {
		t.clients--
	}
	t.last = now
}

func (t *Tracker) Contact(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.last = now
}

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

func (t *Tracker) Expired(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dormant() {
		return false
	}
	return now.Sub(t.last) >= t.policy.After
}

func (t *Tracker) dormant() bool {
	return t.policy.After <= 0 || t.clients > 0
}
