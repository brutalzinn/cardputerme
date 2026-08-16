package server

import (
	"testing"
	"time"

	"cardputerme/internal/power"
)

func TestHoldingAnArrowKeepsItHeld(t *testing.T) {
	s := testServer()
	s.keyDown("up", time.Now())
	if got := s.repeat.Key(); got != "up" {
		t.Fatalf("got %q", got)
	}
}

func TestTypingHoldsNothing(t *testing.T) {
	s := testServer()
	s.keyDown("a", time.Now())
	if got := s.repeat.Key(); got != "" {
		t.Fatalf("holding a letter must not arm a repeat, got %q", got)
	}
}

func TestReleasingClearsTheHold(t *testing.T) {
	s := testServer()
	s.keyDown("up", time.Now())
	s.keyUp("up")
	if got := s.repeat.Key(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestAStrayReleaseIsHarmless(t *testing.T) {
	s := testServer()
	s.keyUp("up")
	if got := s.repeat.Key(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestAKeyOnADarkScreenOnlyWakesIt(t *testing.T) {
	s := testServer()
	s.power.Sleep(time.Now())
	s.keyDown("a", time.Now())
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess.input != "" {
		t.Fatalf("a keypress on a dark screen must be swallowed, got %q", s.sess.input)
	}
	if got := s.power.State(); got != power.On {
		t.Fatalf("it must still wake the screen, got %q", got)
	}
}

func TestTheNextKeyAfterWakingIsTyped(t *testing.T) {
	s := testServer()
	s.power.Sleep(time.Now())
	s.keyDown("a", time.Now())
	s.keyDown("b", time.Now())
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess.input != "b" {
		t.Fatalf("got %q", s.sess.input)
	}
}

func TestADimScreenStillTakesTheKey(t *testing.T) {
	s := sleepyServer()
	if st, _ := s.power.At(time.Now().Add(45 * time.Second)); st != power.Dim {
		t.Fatalf("setup: expected a dimmed screen, got %q", st)
	}
	s.keyDown("a", time.Now())
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess.input != "a" {
		t.Fatalf("a dim screen is readable, so the key must go through, got %q", s.sess.input)
	}
}

func TestABatchAppliesEveryEventInOrder(t *testing.T) {
	s := testServer()
	now := time.Now()
	for _, e := range []struct{ key, state string }{{"h", "down"}, {"h", "up"}, {"i", "down"}, {"i", "up"}} {
		s.handleKeyEvent(e.key, e.state, now)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess.input != "hi" {
		t.Fatalf("got %q", s.sess.input)
	}
}

func TestABatchCanOpenAHoldAndLeaveItHeld(t *testing.T) {
	s := testServer()
	now := time.Now()
	s.handleKeyEvent("up", "down", now)
	if got := s.repeat.Key(); got != "up" {
		t.Fatalf("got %q", got)
	}
}

func TestAMissingStateIsATap(t *testing.T) {
	s := testServer()
	s.handleKeyEvent("a", "", time.Now())
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess.input != "a" {
		t.Fatalf("old firmware sends no state and must still type, got %q", s.sess.input)
	}
}
