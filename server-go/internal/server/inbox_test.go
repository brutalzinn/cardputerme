package server

import (
	"strings"
	"testing"
	"time"

	"cardputerme/internal/battery"
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

// A keypress means "I am here", not "I have dealt with every project" — for
// projects you can actually navigate to.
func TestAKeypressSilencesButKeepsOtherSessionsWaiting(t *testing.T) {
	s, srv := alertServer(t)
	s.register("gitme", "gitme", "")
	s.register("lara", "lara", "")
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

// The whole header is ~40 characters at text size 1. The "!n" suffix was added
// after alertWidth was chosen, so a full inbox pushed the battery off the right
// edge — precisely when the most alerts were waiting, and re-creating the bug
// the status-bar battery was added to fix.
func TestAFullInboxDoesNotPushTheBatteryOffScreen(t *testing.T) {
	s, srv := alertServer(t)
	down := false
	s.applyWifi(&down)
	s.gauge.Observe(battery.Reading{Millivolts: 4200, External: true, At: time.Now()})
	for i := 0; i < maxAlerts; i++ {
		postNotify(t, srv.URL, `{"session":"project`+string(rune('a'+i))+`","text":"a very long alert line indeed"}`)
	}
	width := 0
	for _, c := range s.headerCells() {
		width += len([]rune(c.Text))
	}
	if width > headerCols {
		t.Fatalf("header is %d chars (max %d): %q", width, headerCols, headerText(s.headerCells()))
	}
	if !strings.Contains(headerText(s.headerCells()), "100%") {
		t.Fatalf("the battery must survive a full inbox, got %q", headerText(s.headerCells()))
	}
}

// Clipping at store time destroyed the full text forever, so no wider render
// could ever recover it. Clip where the width is known: at render.
func TestTheInboxKeepsTheFullTextAndClipsAtRender(t *testing.T) {
	s, srv := alertServer(t)
	long := "the deploy pipeline failed on the integration stage"
	postNotify(t, srv.URL, `{"session":"gitme","text":"`+long+`"}`)
	s.mu.Lock()
	stored := s.alerts[0].line
	s.mu.Unlock()
	if !strings.Contains(stored, "integration stage") {
		t.Fatalf("the full alert must be kept, stored = %q", stored)
	}
	if n := len([]rune(s.alertTextLocked())); n > alertWidth {
		t.Fatalf("but the rendered form is %d runes: %q", n, s.alertTextLocked())
	}
}

// An alert naming a session this machine does not have is UNADDRESSABLE: any
// program may post any name, and there is nothing to switch to that would clear
// it. Before this they were immortal — inflating the count and holding a dead
// line in the header until the ring pushed them out.
func TestAKeypressAnswersAlertsThereIsNoSessionToVisit(t *testing.T) {
	s, srv := alertServer(t)
	s.register("gitme", "gitme", "")
	postNotify(t, srv.URL, `{"session":"ci-deploy","text":"build failed"}`)
	postNotify(t, srv.URL, `{"session":"gitme","text":"tests failed"}`)
	s.switchTo("gitme")

	s.dismissAlert()
	if got := s.waiting(); got != 0 {
		t.Fatalf("waiting = %d — an alert you cannot navigate to would never clear", got)
	}
}
