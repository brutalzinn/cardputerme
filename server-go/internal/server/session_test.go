package server

import "testing"

func TestTmuxTargetIsTheSessionNotTheLabel(t *testing.T) {
	if got := tmuxTarget(Config{Name: "cardputerme", Session: "claude-3"}); got != "claude-3" {
		t.Fatalf("the backend must attach to the real tmux session, got %q", got)
	}
}

func TestLabelIsTheTargetWhenNoSessionIsGiven(t *testing.T) {
	if got := tmuxTarget(Config{Name: "cardputerme"}); got != "cardputerme" {
		t.Fatalf("got %q", got)
	}
}

func TestTheDeviceSeesTheLabel(t *testing.T) {
	s := New(Config{Name: "cardputerme", Session: "claude-3", WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40})
	if s.cfg.Name != "cardputerme" {
		t.Fatalf("got %q", s.cfg.Name)
	}
}
