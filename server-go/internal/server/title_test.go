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

const framedInput = "real output line\n" +
	"──────────────── my-session ──\n" +
	"❯ Press up to edit queued messages\n" +
	"──────────────────────────────\n" +
	"  esc to interrupt\n"

func TestTheInputWidgetIsNotMirrored(t *testing.T) {
	grid, _, _ := splitScreen(framedInput)
	for _, l := range grid {
		if strings.Contains(l.Text, "Press up to edit") {
			t.Fatalf("the app's own input box is chrome; the server renders its own input line: %q", l.Text)
		}
	}
}

func TestOutputAboveTheFrameSurvives(t *testing.T) {
	grid, _, _ := splitScreen(framedInput)
	found := false
	for _, l := range grid {
		if strings.Contains(l.Text, "real output line") {
			found = true
		}
	}
	if !found {
		t.Fatal("only the framed widget is dropped, never the transcript")
	}
}

func TestALoneRuleDoesNotSwallowTheTranscript(t *testing.T) {
	pane := "──────────────────────────────\n" +
		"keep me\nand me\nand me too\nand also me\nand still me\n" +
		"  esc to interrupt\n"
	grid, _, _ := splitScreen(pane)
	kept := 0
	for _, l := range grid {
		if strings.HasPrefix(l.Text, "keep me") || strings.HasPrefix(l.Text, "and") {
			kept++
		}
	}
	if kept != 5 {
		t.Fatalf("an unpaired rule must not hide everything after it, kept %d/5", kept)
	}
}

func TestAWideGapBetweenRulesIsContent(t *testing.T) {
	pane := "──────────────────────────────\n" +
		"one\ntwo\nthree\nfour\nfive\nsix\n" +
		"──────────────────────────────\n" +
		"  esc to interrupt\n"
	grid, _, _ := splitScreen(pane)
	if len(grid) < 6 {
		t.Fatalf("six rows between rules is a section, not an input box, got %d", len(grid))
	}
}
