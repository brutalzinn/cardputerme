package server

import (
	"testing"

	"cardputerme/internal/screen"
)

func testServer() *Server {
	return New(Config{Name: "test", WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40, Notify: true})
}

func TestDetectPromptAwaiting(t *testing.T) {
	if !detectPromptAwaiting("Proceed?\n1. Yes\n2. No") {
		t.Fatal("two options should await")
	}
	if detectPromptAwaiting("just text\nnothing here") {
		t.Fatal("plain text should not await")
	}
	if detectPromptAwaiting("Only one:\n1. lonely") {
		t.Fatal("a single option is not a menu")
	}
	if detectPromptAwaiting("") {
		t.Fatal("empty is not awaiting")
	}
}

func TestFindSelectorRow(t *testing.T) {
	grid := []screen.Line{
		{Text: "some output"},
		{Text: "  1. Alpha"},
		{Text: "> 2. Beta"},
	}
	if got := findSelectorRow(grid); got != 2 {
		t.Fatalf("got %d", got)
	}
	if got := findSelectorRow([]screen.Line{{Text: "no pointer"}}); got != -1 {
		t.Fatalf("want -1 got %d", got)
	}
}

func TestGridLines(t *testing.T) {
	esc := "\x1b"
	rows := []string{esc + "[38;2;255;0;0mred line", "plain\ttab"}
	g := gridLines(rows)
	if len(g) != 2 {
		t.Fatalf("len %d", len(g))
	}
	if g[0].Text != "red line" || g[0].Color != screen.RGB565(255, 0, 0) {
		t.Fatalf("row0 %+v", g[0])
	}
	if g[1].Text != "plain  tab" {
		t.Fatalf("tab not expanded: %q", g[1].Text)
	}
}

func TestSplitScreen(t *testing.T) {
	grid, status := splitScreen("line one\nline two\n\n")
	if status != "line two" {
		t.Fatalf("status %q", status)
	}
	if len(grid) != 1 || grid[0].Text != "line one" {
		t.Fatalf("grid %+v", grid)
	}
}

func TestSplitScreenPrefersInterruptRow(t *testing.T) {
	_, status := splitScreen("building the thing\n✳ Baking… (esc to interrupt)\n$ ")
	if status != "Baking... (esc to interrupt)" {
		t.Fatalf("status %q", status)
	}
}

func TestSplitScreenFallsBackToLastRow(t *testing.T) {
	_, status := splitScreen("line one\nline two\n")
	if status != "line two" {
		t.Fatalf("status %q", status)
	}
}

func TestComposeMirrorFollowsBottom(t *testing.T) {
	s := testServer()
	grid := []screen.Line{}
	for i := range 10 {
		grid = append(grid, screen.Line{Text: "l" + string(rune('0'+i)), Color: screen.Colors.Text})
	}
	st := s.composeMirror(grid, "status", false)
	if len(st.lines) != viewRows {
		t.Fatalf("want %d lines got %d", viewRows, len(st.lines))
	}
	if st.lines[0].Text != "l4" || st.lines[len(st.lines)-1].Text != "l9" {
		t.Fatalf("window %q..%q", st.lines[0].Text, st.lines[len(st.lines)-1].Text)
	}
	if !st.sessionExists {
		t.Fatal("sessionExists should be true")
	}
}

func TestComposeMirrorShowsInputBuffer(t *testing.T) {
	s := testServer()
	s.input = "git status"
	st := s.composeMirror([]screen.Line{{Text: "prev", Color: screen.Colors.Text}}, "", false)
	last := st.lines[len(st.lines)-1]
	if last.Text != "> git status" || last.Color != screen.Colors.Prompt {
		t.Fatalf("input line %+v", last)
	}
}

func TestColsRowsScaleWithSize(t *testing.T) {
	s := testServer() // WrapCols 20, viewRows 6, size 2 baseline
	if s.cols() != 20 || s.rows() != 6 {
		t.Fatalf("size2 got cols=%d rows=%d", s.cols(), s.rows())
	}
	s.size = 1
	if s.cols() != 40 || s.rows() != 12 {
		t.Fatalf("size1 got cols=%d rows=%d", s.cols(), s.rows())
	}
	s.size = 3
	if s.cols() != 13 || s.rows() != 4 {
		t.Fatalf("size3 got cols=%d rows=%d", s.cols(), s.rows())
	}
}

func TestZoomInOutResetClamped(t *testing.T) {
	s := testServer()
	s.applyKey("ctrl+=")
	if s.size != 3 {
		t.Fatalf("zoom in got %d", s.size)
	}
	s.applyKey("ctrl+=")
	if s.size != 3 {
		t.Fatalf("zoom in clamp got %d", s.size)
	}
	s.applyKey("ctrl+-")
	s.applyKey("ctrl+_")
	s.applyKey("ctrl+-")
	if s.size != 1 {
		t.Fatalf("zoom out clamp got %d", s.size)
	}
	s.applyKey("ctrl+ ") // reset to baseline
	if s.size != baseSize {
		t.Fatalf("zoom reset got %d", s.size)
	}
}

func TestComposeMirrorCarriesSize(t *testing.T) {
	s := testServer()
	s.size = 3
	st := s.composeMirror([]screen.Line{{Text: "x", Color: screen.Colors.Text}}, "", false)
	if st.size != 3 {
		t.Fatalf("size in stateResult got %d", st.size)
	}
}

func TestComposeMirrorShowsGhostSuggestion(t *testing.T) {
	s := testServer()
	s.input = "git c"
	s.history = []string{"git commit"}
	st := s.composeMirror([]screen.Line{{Text: "prev", Color: screen.Colors.Text}}, "", false)
	last := st.lines[len(st.lines)-1]
	if last.Text != "git commit" || last.Color != screen.Colors.Dim {
		t.Fatalf("ghost line %+v", last)
	}
}
