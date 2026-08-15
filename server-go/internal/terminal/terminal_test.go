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
