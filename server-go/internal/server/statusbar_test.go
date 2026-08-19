package server

import (
	"strings"
	"testing"
)

// The battery moved entirely off the status bar (user, 2026-08-17): the
// device draws its own battery and charging state directly now, so this
// line must never compose a percentage again — that would be two sources
// of truth disagreeing on the same fact.
func TestStatusBarNoLongerShowsBattery(t *testing.T) {
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	st := s.composeMirror(nil, "", "", false)
	if strings.Contains(st.status, "%") {
		t.Fatalf("status bar must not compose a battery reading anymore, got %q", st.status)
	}
}

// Past 39 characters the device marquees the bar (drawStatusBar), so anything
// beyond that scrolls out of sight.
func TestStatusBarNeverOutgrowsTheScreen(t *testing.T) {
	s := withSession(New(Config{Name: "proj", Session: "proj", WrapCols: 20}))
	st := s.composeMirror(nil, strings.Repeat("hint ", 60), strings.Repeat("title ", 40), false)
	if n := len([]rune(st.status)); n > statusMax {
		t.Fatalf("status is %d runes (max %d): %q", n, statusMax, st.status)
	}
	if !strings.Contains(st.status, "proj") {
		t.Fatalf("the session name must survive truncation, got %q", st.status)
	}
}

// c0 and z2 are the defaults and say nothing.
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
