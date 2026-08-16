package server

import (
	"strings"
	"testing"
)

// #44 exists so a prompt in ANY session reaches you. One alert slot threw that
// away again: two projects finishing meant one was silently lost.
func TestASecondAlertDoesNotDestroyTheFirst(t *testing.T) {
	s, srv := alertServer(t)
	postNotify(t, srv.URL, `{"session":"gitme","text":"tests failed"}`)
	postNotify(t, srv.URL, `{"session":"lara","text":"deploy done"}`)
	if got := s.waiting(); got != 2 {
		t.Fatalf("waiting = %d, want 2", got)
	}
}

// The live server log showed the same alert five times in a row. Without this
// the inbox fills with duplicates in seconds and the count becomes noise.
func TestIdenticalAlertsCollapse(t *testing.T) {
	s, srv := alertServer(t)
	for i := 0; i < 5; i++ {
		postNotify(t, srv.URL, `{"session":"gitme","text":"waiting"}`)
	}
	if got := s.waiting(); got != 1 {
		t.Fatalf("waiting = %d, want 1", got)
	}
}

// Silence must mean "make no noise", never "throw away". `;notify 0` used to
// discard the alert entirely, so a user who silenced the device also lost the
// record of who wanted them.
func TestSilencedAlertsAreStillQueued(t *testing.T) {
	s, srv := alertServer(t)
	s.SetNotify(false)
	_, body := postNotify(t, srv.URL, `{"session":"gitme","text":"tests failed"}`)
	if body["delivered"] != false {
		t.Fatalf("a silenced alert was not delivered, body = %v", body)
	}
	if body["queued"] != true {
		t.Fatalf("but it must still be queued, body = %v", body)
	}
	if got := s.waiting(); got != 1 {
		t.Fatalf("waiting = %d, want 1", got)
	}
	if s.lastLed == ledMessage(ledAttention) {
		t.Fatal("silence still means no LED")
	}
}

func TestViewingASessionClearsOnlyItsAlerts(t *testing.T) {
	s, srv := alertServer(t)
	s.register("gitme", "gitme", "")
	s.register("lara", "lara", "")
	postNotify(t, srv.URL, `{"session":"gitme","text":"tests failed"}`)
	postNotify(t, srv.URL, `{"session":"lara","text":"deploy done"}`)

	s.switchTo("gitme")
	if got := s.waiting(); got != 1 {
		t.Fatalf("waiting = %d, want only lara left", got)
	}
	if strings.Contains(headerText(s.headerCells()), "gitme") {
		t.Fatalf("the session you are looking at has been answered, header = %q", headerText(s.headerCells()))
	}
}

// A keypress means "I am here", not "I have dealt with every project".
func TestAKeypressSilencesButKeepsOtherSessionsWaiting(t *testing.T) {
	s, srv := alertServer(t)
	s.register("gitme", "gitme", "")
	postNotify(t, srv.URL, `{"session":"gitme","text":"tests failed"}`)
	postNotify(t, srv.URL, `{"session":"lara","text":"deploy done"}`)
	s.switchTo("gitme")

	s.dismissAlert()
	if s.lastLed != ledMessage(ledOff) {
		t.Fatalf("a keypress darkens the LED, led = %q", s.lastLed)
	}
	if got := s.waiting(); got != 1 {
		t.Fatalf("waiting = %d — lara never got your attention", got)
	}
}

func TestTheInboxIsBounded(t *testing.T) {
	s, srv := alertServer(t)
	for i := 0; i < maxAlerts+5; i++ {
		postNotify(t, srv.URL, `{"session":"s`+string(rune('a'+i))+`","text":"waiting"}`)
	}
	if got := s.waiting(); got != maxAlerts {
		t.Fatalf("waiting = %d, want the cap %d", got, maxAlerts)
	}
}

// With several waiting, the newest line is shown and the rest are a count —
// "3 waiting" is nine characters the 39-column status bar cannot afford.
func TestSeveralWaitingShowsACount(t *testing.T) {
	s, srv := alertServer(t)
	postNotify(t, srv.URL, `{"session":"gitme","text":"tests failed"}`)
	postNotify(t, srv.URL, `{"session":"lara","text":"deploy done"}`)
	got := headerText(s.headerCells())
	if !strings.Contains(got, "lara") {
		t.Fatalf("the newest alert leads, header = %q", got)
	}
	if !strings.Contains(got, "!2") {
		t.Fatalf("the others must still be counted, header = %q", got)
	}
}

func TestOneWaitingShowsNoCount(t *testing.T) {
	s, srv := alertServer(t)
	postNotify(t, srv.URL, `{"session":"gitme","text":"tests failed"}`)
	if got := headerText(s.headerCells()); strings.Contains(got, "!1") {
		t.Fatalf("a count of one is noise, header = %q", got)
	}
}
