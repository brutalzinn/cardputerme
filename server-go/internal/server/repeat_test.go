package server

import (
	"testing"

	"cardputerme/internal/input"
)

func TestScrollingRepeats(t *testing.T) {
	if !repeatableAction(input.Action{Kind: "pan", Key: "up"}) {
		t.Fatal("panning the terminal history is the whole point of holding a key")
	}
}

func TestArrowsSentToTheTerminalRepeat(t *testing.T) {
	for _, k := range []string{"up", "down", "left", "right"} {
		if !repeatableAction(input.Action{Kind: "pressKey", Key: k}) {
			t.Fatalf("%q should repeat", k)
		}
	}
}

func TestTypingNeverRepeats(t *testing.T) {
	if repeatableAction(input.Action{Kind: "none"}) {
		t.Fatal("holding a letter must not spam the input buffer")
	}
}

func TestSendingNeverRepeats(t *testing.T) {
	if repeatableAction(input.Action{Kind: "send", Text: "ls"}) {
		t.Fatal("holding enter must not resubmit the command")
	}
}

func TestNonArrowKeyPressesDoNotRepeat(t *testing.T) {
	for _, k := range []string{"enter", "escape", "tab", "ctrl+c"} {
		if repeatableAction(input.Action{Kind: "pressKey", Key: k}) {
			t.Fatalf("%q should not repeat", k)
		}
	}
}

func TestZoomDoesNotRepeat(t *testing.T) {
	if repeatableAction(input.Action{Kind: "zoom", Key: "in"}) {
		t.Fatal("holding zoom would run away through the size range")
	}
}

func TestATapStillWorksWithoutAHoldState(t *testing.T) {
	s := testServer()
	s.keyUp("up")
	if got := s.repeat.Key(); got != "" {
		t.Fatalf("a stray release must leave nothing held, got %q", got)
	}
}
