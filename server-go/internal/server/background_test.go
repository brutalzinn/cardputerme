package server

import (
	"os/exec"
	"testing"
	"time"

	"cardputerme/internal/quiet"
	"cardputerme/internal/screen"
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

var quietBase = time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

// newQuietSession builds a session with no tmux backend — checkQuiet never
// touches sess.backend, so this exercises the quiet path without needing a
// live terminal. Cleanup disarms any real timer checkQuiet armed for the
// suppressed-and-repoll case, so a stray fire can never later call
// checkSession on this session's nil backend.
func newQuietSession(t *testing.T, s *Server, name string) *session {
	t.Helper()
	sess := newSession(name, nil)
	sess.quiet = quiet.NewTracker(quiet.Policy{After: s.cfg.QuietAfter}, quietBase)
	s.mu.Lock()
	s.sessions[name] = sess
	s.order = append(s.order, name)
	s.mu.Unlock()
	t.Cleanup(func() { sess.quietCheck.arm(0, func() {}) })
	return sess
}

// The ROADMAP's design note from the live device: a spinner ticks the status
// row every second, so quiet must be measured on the GRID, not the whole
// frame, or a stuck task never looks quiet.
func TestGridContentIgnoresTheStatusRow(t *testing.T) {
	pane1 := "hello\nworld\nspinner-tick-1"
	pane2 := "hello\nworld\nspinner-tick-2"
	grid1, _, _ := splitScreen(pane1, nil)
	grid2, _, _ := splitScreen(pane2, nil)
	if gridContent(grid1) != gridContent(grid2) {
		t.Fatal("gridContent must ignore the status row — a ticking spinner must not look like a change")
	}
}

func TestQuietFiresAfterUnchangedGridWithScreenOff(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, Notify: true, QuietAfter: 60 * time.Second})
	sess := newQuietSession(t, s, "q1")
	s.power.Sleep(quietBase)

	notified := make(chan string, 4)
	s.onNotify = func(session string) { notified <- session }

	grid := []screen.Line{{Text: "same output"}}
	s.checkQuiet("q1", sess, grid, quietBase)
	s.checkQuiet("q1", sess, grid, quietBase.Add(60*time.Second))

	select {
	case got := <-notified:
		if got != "q1" {
			t.Fatalf("notified for %q, want q1", got)
		}
	default:
		t.Fatal("unchanged grid for the full quiet window with the screen off must notify")
	}
}

func TestQuietDoesNotFireTwiceForTheSameSilence(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, Notify: true, QuietAfter: 60 * time.Second})
	sess := newQuietSession(t, s, "q2")
	s.power.Sleep(quietBase)
	notified := make(chan string, 4)
	s.onNotify = func(session string) { notified <- session }

	grid := []screen.Line{{Text: "same output"}}
	s.checkQuiet("q2", sess, grid, quietBase)
	s.checkQuiet("q2", sess, grid, quietBase.Add(60*time.Second))
	<-notified // consume the first fire

	s.checkQuiet("q2", sess, grid, quietBase.Add(120*time.Second))
	select {
	case again := <-notified:
		t.Fatalf("fired twice for the same silence (%q)", again)
	default:
	}
}

func TestQuietSuppressedWhileScreenIsOn(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, Notify: true, QuietAfter: 60 * time.Second})
	sess := newQuietSession(t, s, "q3")
	// power starts On by default — do not sleep it yet.
	notified := make(chan string, 4)
	s.onNotify = func(session string) { notified <- session }

	grid := []screen.Line{{Text: "same output"}}
	s.checkQuiet("q3", sess, grid, quietBase)
	s.checkQuiet("q3", sess, grid, quietBase.Add(60*time.Second))
	select {
	case got := <-notified:
		t.Fatalf("must not notify while the screen is on, got %q", got)
	default:
	}

	// Sleeping the screen and re-checking at the same due instant must still
	// fire — suppressed, not lost.
	s.power.Sleep(quietBase.Add(60 * time.Second))
	s.checkQuiet("q3", sess, grid, quietBase.Add(60*time.Second))
	select {
	case got := <-notified:
		if got != "q3" {
			t.Fatalf("notified for %q, want q3", got)
		}
	default:
		t.Fatal("a due-but-suppressed quiet period must fire once the screen goes off")
	}
}

func TestQuietSuppressedWhileNotifySwitchIsOff(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, Notify: false, QuietAfter: 60 * time.Second})
	sess := newQuietSession(t, s, "q4")
	s.power.Sleep(quietBase)
	notified := make(chan string, 4)
	s.onNotify = func(session string) { notified <- session }

	grid := []screen.Line{{Text: "same output"}}
	s.checkQuiet("q4", sess, grid, quietBase)
	s.checkQuiet("q4", sess, grid, quietBase.Add(60*time.Second))
	select {
	case got := <-notified:
		t.Fatalf("must not notify while ;notify is off, got %q", got)
	default:
	}
}

func TestQuietResetsWhenGridChanges(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, Notify: true, QuietAfter: 60 * time.Second})
	sess := newQuietSession(t, s, "q5")
	s.power.Sleep(quietBase)
	notified := make(chan string, 4)
	s.onNotify = func(session string) { notified <- session }

	gridA := []screen.Line{{Text: "A"}}
	gridB := []screen.Line{{Text: "B"}}

	s.checkQuiet("q5", sess, gridA, quietBase)
	s.checkQuiet("q5", sess, gridB, quietBase.Add(30*time.Second)) // changed — resets the clock
	s.checkQuiet("q5", sess, gridB, quietBase.Add(60*time.Second)) // only 30s since reset — not due
	select {
	case got := <-notified:
		t.Fatalf("fired too early after a reset, got %q", got)
	default:
	}

	s.checkQuiet("q5", sess, gridB, quietBase.Add(90*time.Second)) // 60s since reset — due
	select {
	case got := <-notified:
		if got != "q5" {
			t.Fatalf("notified for %q, want q5", got)
		}
	default:
		t.Fatal("60s unchanged since the reset must fire")
	}
}

func TestBackgroundCheckIgnoresAnUnknownSessionForQuietToo(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20, Notify: true, QuietAfter: 60 * time.Second})
	s.checkSession("ghost") // must not panic now that checkSession also touches sess.quiet
}
