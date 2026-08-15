package server

import (
	"strconv"
	"testing"
)

func TestAnEmptyMailboxHasNothingToSend(t *testing.T) {
	var m mailbox
	if got := m.take(); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestOnlyTheNewestFrameSurvives(t *testing.T) {
	var m mailbox
	m.putFrame("old")
	m.putFrame("newer")
	m.putFrame("newest")
	got := m.take()
	if len(got) != 1 || got[0] != "newest" {
		t.Fatalf("a slow device must render the latest state, not a backlog, got %v", got)
	}
}

func TestEventsAreNeverDropped(t *testing.T) {
	var m mailbox
	m.putEvent("power-dim")
	m.putEvent("notify")
	got := m.take()
	if len(got) != 2 || got[0] != "power-dim" || got[1] != "notify" {
		t.Fatalf("got %v", got)
	}
}

func TestEventsGoOutBeforeTheFrame(t *testing.T) {
	var m mailbox
	m.putFrame("frame")
	m.putEvent("power-on")
	got := m.take()
	if len(got) != 2 || got[0] != "power-on" || got[1] != "frame" {
		t.Fatalf("got %v", got)
	}
}

func TestTakingEmptiesTheMailbox(t *testing.T) {
	var m mailbox
	m.putFrame("frame")
	m.putEvent("evt")
	m.take()
	if got := m.take(); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestAFloodOfEventsCannotGrowForever(t *testing.T) {
	var m mailbox
	for i := 0; i < maxPendingEvents*3; i++ {
		m.putEvent(strconv.Itoa(i))
	}
	got := m.take()
	if len(got) > maxPendingEvents {
		t.Fatalf("pending events must stay bounded, got %d", len(got))
	}
	if got[len(got)-1] != strconv.Itoa(maxPendingEvents*3-1) {
		t.Fatalf("the newest event must survive, got %q", got[len(got)-1])
	}
}
