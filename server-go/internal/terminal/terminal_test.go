package terminal

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func found(string) (string, error) { return "/usr/bin/tmux", nil }

func missing(string) (string, error) { return "", exec.ErrNotFound }

func TestInstalledTmuxPasses(t *testing.T) {
	if err := lookupTmux(found); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestMissingTmuxIsRefused(t *testing.T) {
	err := lookupTmux(missing)
	if !errors.Is(err, ErrNoTmux) {
		t.Fatalf("got %v", err)
	}
}

func TestTheRefusalTellsYouHowToFixIt(t *testing.T) {
	msg := lookupTmux(missing).Error()
	for _, want := range []string{"tmux", "brew install tmux", "apt install tmux"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q is missing %q", msg, want)
		}
	}
}

func TestTheCheckAsksForTmuxByName(t *testing.T) {
	asked := ""
	look := func(name string) (string, error) {
		asked = name
		return "/usr/bin/tmux", nil
	}
	lookupTmux(look)
	if asked != "tmux" {
		t.Fatalf("looked up %q", asked)
	}
}

func TestASingleKeyNeedsNoRepeatCount(t *testing.T) {
	got := sendKeyArgs("sess", "up", 1)
	for _, a := range got {
		if a == "-N" {
			t.Fatalf("got %v", got)
		}
	}
	if got[len(got)-1] != "Up" {
		t.Fatalf("got %v", got)
	}
}

func TestRepeatedKeysGoInOneCommand(t *testing.T) {
	got := sendKeyArgs("sess", "up", 4)
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "-N 4") {
		t.Fatalf("four presses should be one tmux call, got %v", got)
	}
}

func TestLiteralKeysAlsoRepeat(t *testing.T) {
	got := sendKeyArgs("sess", "x", 3)
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "-N 3") || !strings.Contains(joined, "-l x") {
		t.Fatalf("got %v", got)
	}
}
