package server

import (
	"strings"
	"time"

	"cardputerme/internal/power"
	"cardputerme/internal/screen"
)

// backgroundCheck debounces prompt detection for sessions nobody is looking at.
// Each check costs a tmux capture, so a chatty project must not spawn one per
// line of output.
const backgroundCheck = 400 * time.Millisecond

func shouldNotify(awaiting, last, notify bool) bool {
	return notify && awaiting && !last
}

// sessionChanged routes a terminal's output. The session being viewed pushes a
// frame; every OTHER session is still watched for prompts, because a question
// in a project you are not looking at is exactly what should reach you.
func (s *Server) sessionChanged(name string) {
	if s.currentName() == name {
		s.schedulePush()
		return
	}
	s.scheduleCheck(name)
}

func (s *Server) scheduleCheck(name string) {
	s.mu.Lock()
	sess, ok := s.sessions[name]
	s.mu.Unlock()
	if !ok {
		return
	}
	sess.check.arm(backgroundCheck, func() { s.checkSession(name) })
}

// checkSession looks for a fresh prompt in a session that is not on screen.
// Capture stays OUT of the lock — it forks tmux.
func (s *Server) checkSession(name string) {
	s.mu.Lock()
	sess, ok := s.sessions[name]
	notify := s.notify
	s.mu.Unlock()
	if !ok {
		return
	}

	pane, live := sess.backend.Capture()
	if !live {
		return
	}
	grid, status, _ := splitScreen(pane, s.cfg.HidePrefixes)
	awaiting := detectPromptAwaiting(gridTail(grid, status))

	s.mu.Lock()
	fire := shouldNotify(awaiting, sess.lastAwaiting, notify)
	sess.lastAwaiting = awaiting
	s.mu.Unlock()

	if fire {
		s.raiseAlert(name, awaitingAlert, attention)
		if s.onNotify != nil {
			s.onNotify(name)
		}
	}

	s.checkQuiet(name, sess, grid, time.Now())
}

// checkQuiet is #38's half: a deadline armed off the last GRID change (not
// the raw frame — the status row's spinner ticks every second even when
// nothing else moves). It rides checkSession deliberately — checkSession
// already runs on every tmux change (frequent while a spinner is live) AND
// is safe to re-run redundantly, which is exactly what re-checking a truly
// silent session later needs. `now` is explicit so tests never sleep.
func (s *Server) checkQuiet(name string, sess *session, grid []screen.Line, now time.Time) {
	sess.quiet.Contact(now, gridContent(grid))

	wait := sess.quiet.Until(now)
	if wait <= 0 && sess.quiet.Due(now) {
		if s.power.State() == power.Off && s.NotifyEnabled() {
			sess.quiet.Consume(now)
			s.raiseAlert(name, quietAlert, attention)
			if s.onNotify != nil {
				s.onNotify(name)
			}
		} else {
			// Suppressed (screen on, or ;notify 0) — don't consume; a human
			// may still be looking. Re-poll after another full window
			// rather than nag every backgroundCheck tick.
			wait = s.cfg.QuietAfter
		}
	}
	sess.quietCheck.arm(wait, func() { s.checkSession(name) })
}

func gridContent(grid []screen.Line) string {
	var sb strings.Builder
	for _, l := range grid {
		sb.WriteString(l.Text)
		sb.WriteByte('\n')
	}
	return sb.String()
}
