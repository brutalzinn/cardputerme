package input

import "testing"

func mir(input string) State { return State{Input: input, Hist: -1} }

func TestCharAppends(t *testing.T) {
	r := InterpretKey(mir(""), "h", KeyCtx{})
	if r.State.Input != "h" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestBackspace(t *testing.T) {
	if r := InterpretKey(mir("hi"), "backspace", KeyCtx{}); r.State.Input != "h" {
		t.Fatalf("got %q", r.State.Input)
	}
}

func TestShiftEnter(t *testing.T) {
	r := InterpretKey(mir("line1"), "shift+enter", KeyCtx{})
	if r.State.Input != "line1\n" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestEnterSends(t *testing.T) {
	r := InterpretKey(mir("git status"), "enter", KeyCtx{})
	if r.Action.Kind != "send" || r.Action.Text != "git status" || r.State.Input != "" {
		t.Fatalf("got %+v", r)
	}
}

func TestEnterEmptyPresses(t *testing.T) {
	r := InterpretKey(mir(""), "enter", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "enter" {
		t.Fatalf("got %+v", r)
	}
}

func TestEscWhileTypingClears(t *testing.T) {
	r := InterpretKey(mir("half a mess"), "esc", KeyCtx{})
	if r.State.Input != "" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestEscEmptyIsRealEscape(t *testing.T) {
	r := InterpretKey(mir(""), "esc", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "escape" {
		t.Fatalf("got %+v", r)
	}
}

func TestDigitAnswersMenu(t *testing.T) {
	r := InterpretKey(mir(""), "2", KeyCtx{Awaiting: true})
	if r.Action.Kind != "pressKey" || r.Action.Key != "2" || r.State.Input != "" {
		t.Fatalf("got %+v", r)
	}
}

func TestDigitWhileTypingAppends(t *testing.T) {
	r := InterpretKey(mir("port "), "2", KeyCtx{Awaiting: true})
	if r.State.Input != "port 2" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestArrowsPan(t *testing.T) {
	r := InterpretKey(mir("typing kept"), "up", KeyCtx{})
	if r.Action.Kind != "pan" || r.Action.Key != "up" || r.State.Input != "typing kept" {
		t.Fatalf("got %+v", r)
	}
}

func TestArrowsPanWhileAwaiting(t *testing.T) {
	if InterpretKey(mir(""), "down", KeyCtx{Awaiting: true}).Action.Kind != "pan" {
		t.Fatal("down should pan")
	}
}

func TestTab(t *testing.T) {
	r := InterpretKey(mir(""), "tab", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "tab" {
		t.Fatalf("got %+v", r)
	}
}

func TestSuggest(t *testing.T) {
	h := []string{"git status", "git commit", "make build"}
	if Suggest("git c", h) != "git commit" {
		t.Fatal("newest matching prefix")
	}
	if Suggest("git commit", h) != "" {
		t.Fatal("exact match is not a suggestion")
	}
	if Suggest("", h) != "" || Suggest("zzz", h) != "" {
		t.Fatal("no match")
	}
}

func TestTabAcceptsSuggestion(t *testing.T) {
	h := []string{"git status", "git commit"}
	r := InterpretKey(mir("git c"), "tab", KeyCtx{History: h})
	if r.Action.Kind != "none" || r.State.Input != "git commit" {
		t.Fatalf("accept got %+v", r)
	}
}

func TestTabPassesThroughWithoutSuggestion(t *testing.T) {
	r := InterpretKey(mir("xyz"), "tab", KeyCtx{History: []string{"git status"}})
	if r.Action.Kind != "pressKey" || r.Action.Key != "tab" {
		t.Fatalf("passthrough got %+v", r)
	}
}

func TestOptArrows(t *testing.T) {
	r := InterpretKey(mir(""), "opt+up", KeyCtx{Awaiting: true})
	if r.Action.Kind != "pressKey" || r.Action.Key != "up" {
		t.Fatalf("got %+v", r)
	}
}

func TestShiftEscInterrupts(t *testing.T) {
	r := InterpretKey(mir("my draft"), "shift+esc", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "escape" || r.State.Input != "my draft" {
		t.Fatalf("got %+v", r)
	}
}

func TestCtrlLetter(t *testing.T) {
	r := InterpretKey(mir(""), "ctrl+c", KeyCtx{})
	if r.Action.Kind != "pressKey" || r.Action.Key != "ctrl+c" {
		t.Fatalf("got %+v", r)
	}
}

func TestZoomChords(t *testing.T) {
	if r := InterpretKey(mir(""), "ctrl+up", KeyCtx{}); r.Action.Kind != "zoom" || r.Action.Key != "in" {
		t.Fatalf("ctrl+up got %+v", r)
	}
	if r := InterpretKey(mir(""), "ctrl+down", KeyCtx{}); r.Action.Kind != "zoom" || r.Action.Key != "out" {
		t.Fatalf("ctrl+down got %+v", r)
	}
	if r := InterpretKey(mir(""), "ctrl+ ", KeyCtx{}); r.Action.Kind != "zoom" || r.Action.Key != "reset" {
		t.Fatalf("ctrl+space got %+v", r)
	}
	if r := InterpretKey(mir(""), "ctrl+a", KeyCtx{}); r.Action.Kind != "pressKey" {
		t.Fatalf("ctrl+a should pass through, got %+v", r)
	}
}

var hist = []string{"first cmd", "second cmd", "third cmd"}

func TestHistoryRecallOnCtrlFnUp(t *testing.T) {
	r := InterpretKey(mir(""), "ctrl+fn+up", KeyCtx{History: hist})
	if r.State.Input != "third cmd" || r.State.Hist != 2 {
		t.Fatalf("got %+v", r)
	}
}

func TestHistoryStepsBackAndFloors(t *testing.T) {
	s := InterpretKey(mir(""), "ctrl+fn+up", KeyCtx{History: hist}).State
	s = InterpretKey(s, "ctrl+fn+up", KeyCtx{History: hist}).State
	if s.Input != "second cmd" {
		t.Fatalf("got %q", s.Input)
	}
	s = InterpretKey(s, "ctrl+fn+up", KeyCtx{History: hist}).State
	s = InterpretKey(s, "ctrl+fn+up", KeyCtx{History: hist}).State
	if s.Input != "first cmd" {
		t.Fatalf("floor got %q", s.Input)
	}
}

func TestHistoryForwardClears(t *testing.T) {
	s := InterpretKey(mir(""), "ctrl+fn+up", KeyCtx{History: hist}).State
	s = InterpretKey(s, "ctrl+fn+up", KeyCtx{History: hist}).State
	s = InterpretKey(s, "ctrl+fn+down", KeyCtx{History: hist}).State
	if s.Input != "third cmd" {
		t.Fatalf("got %q", s.Input)
	}
	s = InterpretKey(s, "ctrl+fn+down", KeyCtx{History: hist}).State
	if s.Input != "" || s.Hist != -1 {
		t.Fatalf("got %+v", s)
	}
}

func TestRecalledEditableAndSends(t *testing.T) {
	s := InterpretKey(mir(""), "ctrl+fn+up", KeyCtx{History: hist}).State
	s = InterpretKey(s, "!", KeyCtx{History: hist}).State
	r := InterpretKey(s, "enter", KeyCtx{History: hist})
	if r.Action.Text != "third cmd!" || r.State.Hist != -1 {
		t.Fatalf("got %+v", r)
	}
}

func TestHistoryNoHistory(t *testing.T) {
	r := InterpretKey(mir(""), "ctrl+fn+up", KeyCtx{History: []string{}})
	if r.State.Input != "" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}
