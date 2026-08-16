package server

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func stressServer() *Server {
	return New(Config{Name: "machine", WrapCols: 20, LinesPerCard: 7, MaxCards: 40, ScrollbackLines: 200})
}

// Many terminals appearing and vanishing at once. register/drop mutate the map
// AND the order slice AND the current pointer, so this is where a torn update
// would show.
func TestStressRegisterAndDropManySessions(t *testing.T) {
	s := stressServer()
	var wg sync.WaitGroup
	for i := range 60 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("s%d", n)
			s.register(name, name, "/tmp")
			s.pickerItems()
			s.drop(name)
		}(i)
	}
	wg.Wait()
	if got := s.sessionNames(); len(got) != 0 {
		t.Fatalf("every session was dropped, registry should be empty: %v", got)
	}
	if s.currentName() != "" {
		t.Fatalf("no sessions left means no current, got %q", s.currentName())
	}
}

// The order slice is reused in place by drop (s.order[:0]) — an aliasing bug
// there would corrupt the list rather than shorten it.
func TestStressOrderStaysConsistentWithTheMap(t *testing.T) {
	s := stressServer()
	for i := range 40 {
		name := fmt.Sprintf("s%d", i)
		s.register(name, name, "/tmp")
	}
	var wg sync.WaitGroup
	for i := range 40 {
		if i%2 != 0 {
			continue
		}
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.drop(fmt.Sprintf("s%d", n))
		}(i)
	}
	wg.Wait()

	names := s.sessionNames()
	if len(names) != 20 {
		t.Fatalf("want 20 survivors, got %d: %v", len(names), names)
	}
	seen := map[string]bool{}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range names {
		if seen[n] {
			t.Fatalf("duplicate %q in order: %v", n, names)
		}
		seen[n] = true
		if _, ok := s.sessions[n]; !ok {
			t.Fatalf("order lists %q but the map does not have it", n)
		}
	}
	if len(s.sessions) != len(names) {
		t.Fatalf("map has %d, order has %d", len(s.sessions), len(names))
	}
}

// Dropping the CURRENT session must always promote a survivor, never leave a
// dangling pointer — hammered while others read it.
func TestStressDropCurrentAlwaysPromotes(t *testing.T) {
	s := stressServer()
	for i := range 30 {
		name := fmt.Sprintf("s%d", i)
		s.register(name, name, "/tmp")
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 400 {
			if cur := s.currentName(); cur == "" && len(s.sessionNames()) > 0 {
				t.Error("current went empty while sessions remained")
				return
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 29 {
			s.drop(fmt.Sprintf("s%d", i))
		}
	}()
	wg.Wait()
}

// The event queue is bounded and coalescing; flooding it must never block a
// producer nor wedge the dispatcher.
func TestStressEventFloodDoesNotBlockOrLose(t *testing.T) {
	s := stressServer()
	s.register("a", "a", "/tmp")
	go s.dispatch()
	defer s.shutdown()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 2000 {
				s.emit(sessionEvent{name: "a", kind: evChanged})
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("producers blocked on a full event queue")
	}

	// a death behind 16k coalesced changes must still be acted on
	s.emit(sessionEvent{name: "a", kind: evGone})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.sessionNames()) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("gone event was lost under flood")
}

// Everything at once: switching, rendering, beacons and led churn.
func TestStressMixedTraffic(t *testing.T) {
	s := stressServer()
	for _, n := range []string{"a", "b", "c"} {
		s.register(n, n, "/tmp")
	}
	go s.dispatch()
	defer s.shutdown()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 300 {
			s.handleBeacons([]beacon{
				{Name: "machine", IP: "10.0.0.1", Port: 8001},
				{Name: fmt.Sprintf("other%d", i%3), IP: "10.0.0.2", Port: 8001},
			})
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 300 {
			s.setLed(led{R: i % 255, Pattern: "solid"})
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 300 {
			s.pickerItems()
			s.currentName()
			s.sessionNames()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 300 {
			s.emit(sessionEvent{name: "a", kind: evChanged})
		}
	}()
	wg.Wait()
}
