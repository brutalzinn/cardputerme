package power

import (
	"sync"
	"time"
)

type State string

const (
	On  State = "on"
	Dim State = "dim"
	Off State = "off"
)

type Policy struct {
	DimAfter time.Duration
	OffAfter time.Duration
}

func (p Policy) stateAt(idle time.Duration) State {
	if p.OffAfter > 0 && idle >= p.OffAfter {
		return Off
	}
	if p.DimAfter > 0 && idle >= p.DimAfter {
		return Dim
	}
	return On
}

func (p Policy) until(idle time.Duration) time.Duration {
	if p.DimAfter > 0 && idle < p.DimAfter {
		return p.DimAfter - idle
	}
	if p.OffAfter > 0 && idle < p.OffAfter {
		return p.OffAfter - idle
	}
	return 0
}

type Tracker struct {
	policy Policy

	mu      sync.Mutex
	since   time.Time
	state   State
	inhibit bool
	forced  bool
}

func NewTracker(p Policy, now time.Time) *Tracker {
	return &Tracker{policy: p, since: now, state: On}
}

func (t *Tracker) settle(want State) (State, bool) {
	changed := want != t.state
	t.state = want
	return want, changed
}

func (t *Tracker) At(now time.Time) (State, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.forced {
		return t.settle(Off)
	}
	if t.inhibit {
		return t.settle(On)
	}
	return t.settle(t.policy.stateAt(now.Sub(t.since)))
}

func (t *Tracker) Wake(now time.Time) (State, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.since = now
	t.forced = false
	return t.settle(On)
}

func (t *Tracker) Sleep(now time.Time) (State, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.since = now
	t.forced = true
	return t.settle(Off)
}

func (t *Tracker) SetInhibit(now time.Time, on bool) (State, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inhibit = on
	t.since = now
	if t.forced {
		return t.settle(Off)
	}
	return t.settle(On)
}

func (t *Tracker) Until(now time.Time) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inhibit || t.forced {
		return 0
	}
	return t.policy.until(now.Sub(t.since))
}

func (t *Tracker) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}
