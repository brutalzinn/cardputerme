package server

import (
	"sync"
	"testing"
	"time"
)

// drop() used to call stop() while holding mu (LIFO defer ordering). stop()
// waits for the subscribe goroutine, whose callback takes mu — deadlock. This
// hangs rather than fails if it regresses, so it runs under a timeout.
func TestDropDoesNotDeadlockAgainstTheSubscribeCallback(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	s.register("a", "a", "/tmp")
	s.register("b", "b", "/tmp")

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.drop("a")
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("drop deadlocked — stop() must not run while holding mu")
	}
	if got := s.sessionNames(); len(got) != 1 || got[0] != "b" {
		t.Fatalf("sessions = %v", got)
	}
}

// One server now serves many sessions, so switching, registering, dropping and
// rendering all race against each other. Guards the s.sess reads that used to
// sit outside the lock.
func TestConcurrentSwitchRegisterAndRender(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, LinesPerCard: 7, MaxCards: 40})
	s.register("a", "a", "/tmp")
	s.register("b", "b", "/tmp")

	// Bounded: switchTo captures a pane, so an unbounded spin would fork tmux
	// thousands of times and starve the box rather than test anything.
	var wg sync.WaitGroup
	for _, name := range []string{"a", "b"} {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			for range 15 {
				s.switchTo(n)
			}
		}(name)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 500 {
			s.pickerItems()
			s.currentName()
			s.sessionNames()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 15 {
			s.register("extra", "extra", "/tmp")
			s.drop("extra")
		}
	}()
	wg.Wait()
}
