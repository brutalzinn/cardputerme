package server

import (
	"sync"
	"time"
)

type deadline struct {
	mu    sync.Mutex
	timer *time.Timer
}

func (d *deadline) arm(after time.Duration, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	if after <= 0 {
		return
	}
	d.timer = time.AfterFunc(after, fn)
}
