package main

import "testing"

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
	grid := []Line{
		{Text: "some output", Color: 0},
		{Text: "  1. Alpha", Color: 0},
		{Text: "> 2. Beta", Color: 0},
	}
	if got := findSelectorRow(grid); got != 2 {
		t.Fatalf("got %d", got)
	}
	if got := findSelectorRow([]Line{{Text: "no pointer", Color: 0}}); got != -1 {
		t.Fatalf("want -1 got %d", got)
	}
}

func TestGridLines(t *testing.T) {
	rows := []string{esc + "[38;2;255;0;0mred line", "plain\ttab"}
	g := gridLines(rows)
	if len(g) != 2 {
		t.Fatalf("len %d", len(g))
	}
	if g[0].Text != "red line" || g[0].Color != rgb565(255, 0, 0) {
		t.Fatalf("row0 %+v", g[0])
	}
	if g[1].Text != "plain  tab" {
		t.Fatalf("tab not expanded: %q", g[1].Text)
	}
}

func TestSplitScreen(t *testing.T) {
	pane := "line one\nline two\n\n"
	grid, status := splitScreen(pane)
	if status != "line two" {
		t.Fatalf("status %q", status)
	}
	if len(grid) != 1 || grid[0].Text != "line one" {
		t.Fatalf("grid %+v", grid)
	}
}

func TestSplitScreenPrefersInterruptRow(t *testing.T) {
	pane := "building the thing\n✳ Baking… (esc to interrupt)\n$ "
	_, status := splitScreen(pane)
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
	s := &Session{view: View{Follow: true, SelRow: -1}, hist: -1}
	grid := []Line{}
	for i := range 10 {
		grid = append(grid, Line{Text: "l" + string(rune('0'+i)), Color: colors.Text})
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
	s := &Session{view: View{Follow: true, SelRow: -1}, hist: -1, input: "git status"}
	st := s.composeMirror([]Line{{Text: "prev", Color: colors.Text}}, "", false)
	last := st.lines[len(st.lines)-1]
	if last.Text != "> git status" || last.Color != colors.Prompt {
		t.Fatalf("input line %+v", last)
	}
}
