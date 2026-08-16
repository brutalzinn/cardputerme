package server

import (
	"testing"
	"time"
)

func TestEmitChangedIsDeliveredToTheDispatcher(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	s.emit(sessionEvent{name: "a", kind: evChanged})
	select {
	case ev := <-s.events:
		if ev.name != "a" || ev.kind != evChanged {
			t.Fatalf("got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("event never arrived")
	}
}

// A busy dispatcher must never block a terminal's reader goroutine. Output
// events coalesce — a pending push already covers a later change.
func TestEmitChangedNeverBlocksWhenTheQueueIsFull(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	for range cap(s.events) + 50 {
		s.emit(sessionEvent{name: "a", kind: evChanged})
	}
	if len(s.events) > cap(s.events) {
		t.Fatalf("queue grew past its bound: %d", len(s.events))
	}
}

// A death must NOT be coalesced away — losing it leaks the session forever.
func TestEmitGoneIsNotDroppedWhenTheQueueIsFull(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	for range cap(s.events) {
		s.emit(sessionEvent{name: "a", kind: evChanged})
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.emit(sessionEvent{name: "a", kind: evGone})
	}()
	// drain so the blocked send can land
	go func() {
		for range 5 {
			<-s.events
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a gone event was dropped or blocked forever")
	}
}

func TestEmitDoesNotBlockForeverAfterShutdown(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	for range cap(s.events) {
		s.emit(sessionEvent{name: "a", kind: evChanged})
	}
	s.shutdown()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.emit(sessionEvent{name: "a", kind: evGone})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("emit must give up once the server is shutting down")
	}
}

func TestDispatchDropsASessionOnGone(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	s.register("a", "a", "/tmp")
	s.register("b", "b", "/tmp")
	go s.dispatch()
	defer s.shutdown()

	s.emit(sessionEvent{name: "a", kind: evGone})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.sessionNames()) == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("dispatcher never dropped the dead session: %v", s.sessionNames())
}

func TestDispatchStopsOnShutdown(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	stopped := make(chan struct{})
	go func() {
		s.dispatch()
		close(stopped)
	}()
	s.shutdown()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch must return when the server shuts down")
	}
}
