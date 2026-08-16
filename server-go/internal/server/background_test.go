package server

import (
	"os/exec"
	"testing"
	"time"
)

func TestFreshQuestionOnlyFiresOnTheTransition(t *testing.T) {
	cases := []struct {
		awaiting, last, notify, want bool
	}{
		{true, false, true, true},   // a question just appeared
		{true, true, true, false},   // still the same question — do not nag
		{false, true, true, false},  // it was answered
		{false, false, true, false}, // nothing happening
		{true, false, false, false}, // notifications disabled
	}
	for _, c := range cases {
		if got := shouldNotify(c.awaiting, c.last, c.notify); got != c.want {
			t.Fatalf("shouldNotify(awaiting=%v last=%v notify=%v) = %v, want %v",
				c.awaiting, c.last, c.notify, got, c.want)
		}
	}
}

// The whole point of one server owning many sessions: a prompt in a project you
// are NOT looking at must still reach you.
func TestBackgroundSessionPromptNotifies(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	fg, bg := "bgtest-front", "bgtest-back"
	for _, n := range []string{fg, bg} {
		exec.Command("tmux", "kill-session", "-t", n).Run()
		if err := exec.Command("tmux", "new-session", "-d", "-s", n, "-c", "/tmp").Run(); err != nil {
			t.Skipf("cannot create tmux session: %v", err)
		}
		defer exec.Command("tmux", "kill-session", "-t", n).Run()
	}

	s := New(Config{Name: "m", WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40, Notify: true})
	s.register(fg, fg, "/tmp")
	s.register(bg, bg, "/tmp")
	if s.currentName() != fg {
		t.Fatalf("precondition: current = %q", s.currentName())
	}

	notified := make(chan string, 4)
	s.onNotify = func(session string) { notified <- session }

	exec.Command("tmux", "send-keys", "-t", bg, "-l", "printf 'Deploy?\\n1. Yes\\n2. No\\n'; read x").Run()
	exec.Command("tmux", "send-keys", "-t", bg, "Enter").Run()
	time.Sleep(500 * time.Millisecond)
	s.checkSession(bg)

	select {
	case got := <-notified:
		if got != bg {
			t.Fatalf("notified for %q, want the background session %q", got, bg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a prompt in a background session never notified — this is the reason one server owns many sessions")
	}

	// and it must not nag while the same prompt is still on screen
	s.checkSession(bg)
	select {
	case again := <-notified:
		t.Fatalf("notified twice for the same unanswered prompt (%q)", again)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestBackgroundCheckIgnoresAnUnknownSession(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, Notify: true})
	s.checkSession("ghost") // must not panic
}
