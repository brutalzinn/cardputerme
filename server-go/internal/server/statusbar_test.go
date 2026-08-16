package server

import (
	"strings"
	"testing"
	"time"

	"cardputerme/internal/battery"
)

// The user cannot read the battery on the device and asked for it plainly.
// The status bar is the one region every firmware ever built has rendered from
// server text, so it is where a value you must be able to trust belongs.
func TestStatusBarAlwaysShowsTheBattery(t *testing.T) {
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	s.gauge.Observe(battery.Reading{Millivolts: 3700, At: time.Now()})
	st := s.composeMirror(nil, "", "", false)
	if !strings.Contains(st.status, "50%") {
		t.Fatalf("status = %q", st.status)
	}
}

func TestStatusBarShowsChargingWithAPlus(t *testing.T) {
	now := time.Now()
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	s.gauge.Observe(battery.Reading{Millivolts: 3900, External: true, At: now})
	st := s.composeMirror(nil, "", "", false)
	if !strings.Contains(st.status, "+75%") {
		t.Fatalf("charging must be visible, status = %q", st.status)
	}
}

// Past 39 characters the device marquees the bar (drawStatusBar), so anything
// beyond that scrolls out of sight. A battery you have to wait for is not one
// you can read at a glance.
func TestStatusBarNeverOutgrowsTheScreen(t *testing.T) {
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	s.gauge.Observe(battery.Reading{Millivolts: 3700, At: time.Now()})
	st := s.composeMirror(nil, strings.Repeat("hint ", 60), strings.Repeat("title ", 40), false)
	if n := len([]rune(st.status)); n > statusMax {
		t.Fatalf("status is %d runes (max %d): %q", n, statusMax, st.status)
	}
	if !strings.Contains(st.status, "50%") {
		t.Fatalf("the battery must survive the truncation, got %q", st.status)
	}
	if !strings.Contains(st.status, "proj") {
		t.Fatalf("the session name must survive too, got %q", st.status)
	}
}

// c0 and z2 are the defaults and say nothing. Spending six characters on them
// is what used to leave no room for the battery.
func TestStatusBarOmitsDefaultColumnAndZoom(t *testing.T) {
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	st := s.composeMirror(nil, "", "", false)
	if strings.Contains(st.status, "c0") {
		t.Fatalf("a zero column is not worth two characters, got %q", st.status)
	}
	if strings.Contains(st.status, "z2") {
		t.Fatalf("the default zoom is not worth two characters, got %q", st.status)
	}
	if !strings.Contains(st.status, "r0/0") {
		t.Fatalf("the row counter is the one the user reads, got %q", st.status)
	}
}

func TestStatusBarShowsColumnAndZoomWhenTheyAreNotDefault(t *testing.T) {
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	s.size = 3
	s.sess.view.Col = 4
	st := s.composeMirror(gridLines([]string{strings.Repeat("x", 200)}), "", "", false)
	if !strings.Contains(st.status, "z3") {
		t.Fatalf("a changed zoom must be visible, got %q", st.status)
	}
	if !strings.Contains(st.status, "c4") {
		t.Fatalf("a scrolled column must be visible, got %q", st.status)
	}
}

// An unread battery is not a flat one, and the slot must never be silently
// missing — that is the bug being fixed.
func TestStatusBarKeepsTheSlotBeforeAnyReading(t *testing.T) {
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	st := s.composeMirror(nil, "", "", false)
	if !strings.Contains(st.status, unknownBattery) {
		t.Fatalf("status = %q", st.status)
	}
}
