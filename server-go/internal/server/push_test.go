package server

import (
	"testing"
	"time"
)

func TestOutputWaitsOnlyTheConfiguredDebounce(t *testing.T) {
	s := New(Config{Name: "test", WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40, PushDebounce: 15 * time.Millisecond})
	if got := s.pushAfter(); got != 15*time.Millisecond {
		t.Fatalf("got %v", got)
	}
}

func TestAnUnsetDebounceStillCoalescesBursts(t *testing.T) {
	s := testServer()
	if got := s.pushAfter(); got <= 0 {
		t.Fatalf("a zero debounce would push once per byte of output, got %v", got)
	}
}
