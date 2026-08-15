package input

import "testing"

func interpret(key string) Action {
	return InterpretKey(State{Hist: -1}, key, KeyCtx{}).Action
}

func TestScrollingIsRepeatable(t *testing.T) {
	for _, k := range []string{"up", "down", "left", "right"} {
		if !interpret(k).Repeat {
			t.Fatalf("panning with %q should repeat", k)
		}
	}
}

func TestArrowsSentToTheTerminalAreRepeatable(t *testing.T) {
	for _, k := range []string{"opt+up", "opt+down", "opt+left", "opt+right"} {
		a := interpret(k)
		if a.Kind != "pressKey" || !a.Repeat {
			t.Fatalf("%q got %+v", k, a)
		}
	}
}

func TestTypingIsNotRepeatable(t *testing.T) {
	if interpret("a").Repeat {
		t.Fatal("holding a letter must not spam the input buffer")
	}
}

func TestSubmittingIsNotRepeatable(t *testing.T) {
	a := InterpretKey(State{Input: "ls", Hist: -1}, "enter", KeyCtx{}).Action
	if a.Repeat {
		t.Fatal("holding enter must not resubmit the command")
	}
}

func TestNonArrowKeyPressesAreNotRepeatable(t *testing.T) {
	for _, k := range []string{"enter", "esc", "tab"} {
		if interpret(k).Repeat {
			t.Fatalf("%q should not repeat", k)
		}
	}
}

func TestZoomIsNotRepeatable(t *testing.T) {
	if interpret("ctrl+=").Repeat {
		t.Fatal("holding zoom would run away through the size range")
	}
}
