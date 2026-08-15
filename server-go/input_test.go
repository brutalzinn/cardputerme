package main

import "testing"

func mir(input string) State { return State{Input: input, Hist: -1} }

func TestCharAppends(t *testing.T) {
	r := interpretKey(mir(""), "h", KeyCtx{})
	if r.State.Input != "h" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestBackspace(t *testing.T) {
	if r := interpretKey(mir("hi"), "backspace", KeyCtx{}); r.State.Input != "h" {
		t.Fatalf("got %q", r.State.Input)
	}
}

func TestShiftEnter(t *testing.T) {
	r := interpretKey(mir("line1"), "shift+enter", KeyCtx{})
	if r.State.Input != "line1\n" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestEnterSends(t *testing.T) {
	r := interpretKey(mir("git status"), "enter", KeyCtx{})
	if r.Action.Kind != "send" || r.Action.Text != "git status" || r.State.Input != "" {
		t.Fatalf("got %+v", r)
	}
}

func TestEnterEmptyPresses(t *testing.T) {
	r := interpretKey(mir(""), "enter", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "enter" {
		t.Fatalf("got %+v", r)
	}
}

func TestEscWhileTypingClears(t *testing.T) {
	r := interpretKey(mir("half a mess"), "esc", KeyCtx{})
	if r.State.Input != "" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestEscEmptyIsRealEscape(t *testing.T) {
	r := interpretKey(mir(""), "esc", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "escape" {
		t.Fatalf("got %+v", r)
	}
}

func TestDigitAnswersMenu(t *testing.T) {
	r := interpretKey(mir(""), "2", KeyCtx{Awaiting: true})
	if r.Action.Kind != "pressKey" || r.Action.Key != "2" || r.State.Input != "" {
		t.Fatalf("got %+v", r)
	}
}

func TestDigitWhileTypingAppends(t *testing.T) {
	r := interpretKey(mir("port "), "2", KeyCtx{Awaiting: true})
	if r.State.Input != "port 2" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestArrowsPan(t *testing.T) {
	r := interpretKey(mir("typing kept"), "up", KeyCtx{})
	if r.Action.Kind != "pan" || r.Action.Key != "up" || r.State.Input != "typing kept" {
		t.Fatalf("got %+v", r)
	}
}

func TestArrowsPanWhileAwaiting(t *testing.T) {
	if interpretKey(mir(""), "down", KeyCtx{Awaiting: true}).Action.Kind != "pan" {
		t.Fatal("down should pan")
	}
}

func TestTab(t *testing.T) {
	r := interpretKey(mir(""), "tab", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "tab" {
		t.Fatalf("got %+v", r)
	}
}

func TestSuggest(t *testing.T) {
	h := []string{"git status", "git commit", "make build"}
	if suggest("git c", h) != "git commit" {
		t.Fatal("newest matching prefix")
	}
	if suggest("git commit", h) != "" {
		t.Fatal("exact match is not a suggestion")
	}
	if suggest("", h) != "" || suggest("zzz", h) != "" {
		t.Fatal("no match")
	}
}

func TestTabAcceptsSuggestion(t *testing.T) {
	h := []string{"git status", "git commit"}
	r := interpretKey(mir("git c"), "tab", KeyCtx{History: h})
	if r.Action.Kind != "none" || r.State.Input != "git commit" {
		t.Fatalf("accept got %+v", r)
	}
}

func TestTabPassesThroughWithoutSuggestion(t *testing.T) {
	r := interpretKey(mir("xyz"), "tab", KeyCtx{History: []string{"git status"}})
	if r.Action.Kind != "pressKey" || r.Action.Key != "tab" {
		t.Fatalf("passthrough got %+v", r)
	}
}

func TestOptArrows(t *testing.T) {
	r := interpretKey(mir(""), "opt+up", KeyCtx{Awaiting: true})
	if r.Action.Kind != "pressKey" || r.Action.Key != "up" {
		t.Fatalf("got %+v", r)
	}
}

func TestShiftEscInterrupts(t *testing.T) {
	r := interpretKey(mir("my draft"), "shift+esc", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "escape" || r.State.Input != "my draft" {
		t.Fatalf("got %+v", r)
	}
}

func TestCtrlLetter(t *testing.T) {
	r := interpretKey(mir(""), "ctrl+c", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "ctrl+c" {
		t.Fatalf("got %+v", r)
	}
}

var hist = []string{"first cmd", "second cmd", "third cmd"}

func TestCtrlUpRecallsNewest(t *testing.T) {
	r := interpretKey(mir(""), "ctrl+up", KeyCtx{History: hist})
	if r.State.Input != "third cmd" || r.State.Hist != 2 {
		t.Fatalf("got %+v", r)
	}
}

func TestCtrlUpStepsBackAndFloors(t *testing.T) {
	s := interpretKey(mir(""), "ctrl+up", KeyCtx{History: hist}).State
	s = interpretKey(s, "ctrl+up", KeyCtx{History: hist}).State
	if s.Input != "second cmd" {
		t.Fatalf("got %q", s.Input)
	}
	s = interpretKey(s, "ctrl+up", KeyCtx{History: hist}).State
	s = interpretKey(s, "ctrl+up", KeyCtx{History: hist}).State
	if s.Input != "first cmd" {
		t.Fatalf("floor got %q", s.Input)
	}
}

func TestCtrlDownForwardClears(t *testing.T) {
	s := interpretKey(mir(""), "ctrl+up", KeyCtx{History: hist}).State
	s = interpretKey(s, "ctrl+up", KeyCtx{History: hist}).State
	s = interpretKey(s, "ctrl+down", KeyCtx{History: hist}).State
	if s.Input != "third cmd" {
		t.Fatalf("got %q", s.Input)
	}
	s = interpretKey(s, "ctrl+down", KeyCtx{History: hist}).State
	if s.Input != "" || s.Hist != -1 {
		t.Fatalf("got %+v", s)
	}
}

func TestRecalledEditableAndSends(t *testing.T) {
	s := interpretKey(mir(""), "ctrl+up", KeyCtx{History: hist}).State
	s = interpretKey(s, "!", KeyCtx{History: hist}).State
	r := interpretKey(s, "enter", KeyCtx{History: hist})
	if r.Action.Text != "third cmd!" || r.State.Hist != -1 {
		t.Fatalf("got %+v", r)
	}
}

func TestCtrlUpNoHistory(t *testing.T) {
	r := interpretKey(mir(""), "ctrl+up", KeyCtx{History: []string{}})
	if r.State.Input != "" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}
