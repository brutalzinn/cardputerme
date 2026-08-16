package server

import (
	"strings"
	"testing"
)

const framedPane = "hello world\n" +
	"──────────────── my-session ──\n" +
	"❯ type here\n" +
	"──────────────────────────────\n" +
	"  esc to interrupt\n"

func TestTheFramedTitleLeavesTheBody(t *testing.T) {
	grid, _, _ := splitScreen(framedPane)
	for _, l := range grid {
		if strings.Contains(l.Text, "my-session") {
			t.Fatalf("the title must not eat a body row: %q", l.Text)
		}
	}
}

func TestTheFrameItselfLeavesTheBody(t *testing.T) {
	grid, _, _ := splitScreen(framedPane)
	for _, l := range grid {
		if strings.Contains(l.Text, "───") {
			t.Fatalf("a rule is chrome, not content: %q", l.Text)
		}
	}
}

func TestTheTitleIsCaptured(t *testing.T) {
	if _, _, title := splitScreen(framedPane); title != "my-session" {
		t.Fatalf("got %q", title)
	}
}

func TestRealContentSurvives(t *testing.T) {
	grid, _, _ := splitScreen(framedPane)
	found := false
	for _, l := range grid {
		if strings.Contains(l.Text, "hello world") {
			found = true
		}
	}
	if !found {
		t.Fatal("ordinary lines must be untouched")
	}
}

func TestAPaneWithNoFrameHasNoTitle(t *testing.T) {
	if _, _, title := splitScreen("just some output\nand more\n"); title != "" {
		t.Fatalf("got %q", title)
	}
}

func TestTheTitleReachesTheStatusBar(t *testing.T) {
	s := testServer()
	st := s.stateFrom(framedPane, true)
	if !strings.Contains(st.status, "my-session") {
		t.Fatalf("the title belongs in the status bar, got %q", st.status)
	}
}

func TestTheStatusStillNamesTheExposure(t *testing.T) {
	s := testServer()
	st := s.stateFrom(framedPane, true)
	if !strings.Contains(st.status, "test") {
		t.Fatalf("got %q", st.status)
	}
}
