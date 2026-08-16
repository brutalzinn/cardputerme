package server

import (
	"log"
	"time"
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

	if !fire {
		return
	}
	log.Printf("[notify] %q is waiting for you", name)
	s.hub.broadcast(notifyMessage(name))
	s.signalAttention()
	if s.onNotify != nil {
		s.onNotify(name)
	}
}
