package repeat

import (
	"sync"
	"time"
)

type Policy struct {
	Delay    time.Duration
	Interval time.Duration
	MaxHold  time.Duration
}

const maxCatchUp = 8

type Holder struct {
	policy Policy

	mu    sync.Mutex
	key   string
	since time.Time
	last  time.Time
	fired bool
}

func NewHolder(p Policy) *Holder {
	return &Holder{policy: p}
}

func (h *Holder) Down(key string, now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if key != "" && key == h.key {
		return
	}
	h.key = key
	h.since = now
	h.fired = false
}

func (h *Holder) Up(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.key != key {
		return false
	}
	h.key = ""
	return true
}

func (h *Holder) Fired(now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fired = true
	h.last = now
}

func (h *Holder) Due(now time.Time) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.fired || h.policy.Interval <= 0 {
		return 1
	}
	n := int(now.Sub(h.last) / h.policy.Interval)
	if n < 1 {
		return 1
	}
	if n > maxCatchUp {
		return maxCatchUp
	}
	return n
}

func (h *Holder) Key() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.key
}

func (h *Holder) Next(now time.Time) (time.Duration, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.key == "" || h.policy.Interval <= 0 {
		return 0, false
	}
	if h.policy.MaxHold > 0 && now.Sub(h.since) >= h.policy.MaxHold {
		h.key = ""
		return 0, false
	}
	if h.fired {
		return h.policy.Interval, true
	}
	return h.policy.Delay, true
}
