package server

import (
	"os/exec"
	"testing"
)

func TestTmuxTargetIsTheSessionNotTheLabel(t *testing.T) {
	if got := sessionTarget(Config{Name: "cardputerme", Session: "claude-3"}); got != "claude-3" {
		t.Fatalf("the backend must attach to the real tmux session, got %q", got)
	}
}

func TestLabelIsTheTargetWhenNoSessionIsGiven(t *testing.T) {
	if got := sessionTarget(Config{Name: "cardputerme"}); got != "cardputerme" {
		t.Fatalf("got %q", got)
	}
}

func TestTheDeviceSeesTheLabel(t *testing.T) {
	s := New(Config{Name: "cardputerme", Session: "claude-3", WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40})
	if s.cfg.Name != "cardputerme" {
		t.Fatalf("got %q", s.cfg.Name)
	}
}

// #46: the server now auto-discovers tmux sessions, so it must survive
// owning zero of them — it stays up, watching, for the next one to appear.
func TestTerminalGoneDoesNotShutDownWhenSessionsReachZero(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	s.register("a", "a", "/tmp")
	s.terminalGone("a")
	select {
	case <-s.done:
		t.Fatal("server must survive owning zero sessions — auto-discovery needs it alive to catch the next one")
	default:
	}
	if len(s.sessionNames()) != 0 {
		t.Fatalf("session should have been dropped, got %v", s.sessionNames())
	}
}

func skipWithoutTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
}

func TestDiscoverExistingBackfillsSessionsNotAlreadyKnown(t *testing.T) {
	skipWithoutTmux(t)
	names := []string{"discover-existing-a", "discover-existing-b"}
	for _, n := range names {
		exec.Command("tmux", "kill-session", "-t", n).Run()
		if err := exec.Command("tmux", "new-session", "-d", "-s", n, "-c", "/tmp").Run(); err != nil {
			t.Skipf("cannot create tmux session: %v", err)
		}
		defer exec.Command("tmux", "kill-session", "-t", n).Run()
	}

	s := New(Config{Name: "m", WrapCols: 20})
	s.discoverExisting("")
	got := map[string]bool{}
	for _, n := range s.sessionNames() {
		got[n] = true
	}
	for _, n := range names {
		if !got[n] {
			t.Fatalf("discoverExisting missed %q, got %v", n, s.sessionNames())
		}
	}
}

func TestDiscoverExistingSkipsTheAlreadyRegisteredTarget(t *testing.T) {
	skipWithoutTmux(t)
	name := "discover-existing-skip"
	exec.Command("tmux", "kill-session", "-t", name).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", "/tmp").Run(); err != nil {
		t.Skipf("cannot create tmux session: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", name).Run()

	s := New(Config{Name: "m", WrapCols: 20})
	s.register("m", name, "/tmp")
	s.discoverExisting(name)
	if len(s.sessionNames()) != 1 {
		t.Fatalf("the already-registered target must not be registered a second time, got %v", s.sessionNames())
	}
}
