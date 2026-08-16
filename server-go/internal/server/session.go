package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"cardputerme/internal/screen"
	"cardputerme/internal/terminal"
)

// session is everything that belongs to ONE mirrored terminal. Device-owned
// state (power, battery, led, repeat, the picker) stays on Server, because it
// belongs to the Cardputer rather than to any terminal.
//
// Still guarded by Server.mu — splitting the lock is a separate step.
type session struct {
	name    string
	cwd     string
	backend *terminal.Backend

	input        string
	cmd          string
	reply        string
	hist         int
	history      []string
	view         screen.View
	cache        *mirrorCache
	lastSig      string
	lastAwaiting bool
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ws", s.wsHandler)
	mux.HandleFunc("/sessions", s.sessionsHandler)
	mux.Handle(soundPrefix, s.soundHandler())
}

type sessionRequest struct {
	Name    string `json:"name"`
	Session string `json:"session"`
	Cwd     string `json:"cwd"`
}

func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.registerHandler(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current":  s.currentName(),
		"sessions": s.sessionNames(),
	})
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
		return
	}
	target := req.Session
	if target == "" {
		target = req.Name
	}
	s.register(req.Name, target, req.Cwd)
	log.Printf("[sessions] registered %q (target %q)", req.Name, target)
	writeJSON(w, http.StatusOK, map[string]any{
		"current":  s.currentName(),
		"sessions": s.sessionNames(),
	})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

// register adds a terminal under name. It is idempotent and never replaces a
// live session: doing so would silently discard its backend and history.
func (s *Server) register(name, target, cwd string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.sessions[name]; ok {
		return existing
	}
	sess := newSession(name, terminal.CreateBackend(target, s.cfg.ScrollbackLines))
	sess.cwd = cwd
	s.sessions[name] = sess
	s.order = append(s.order, name)
	if s.sess == nil {
		s.sess = sess
	}
	return sess
}

// drop removes a session. A dying terminal must never take the others with it,
// so this promotes a survivor rather than exiting.
func (s *Server) drop(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[name]; !ok {
		return
	}
	delete(s.sessions, name)
	kept := s.order[:0]
	for _, n := range s.order {
		if n != name {
			kept = append(kept, n)
		}
	}
	s.order = kept
	if s.sess != nil && s.sess.name != name {
		return
	}
	s.sess = nil
	if len(s.order) > 0 {
		s.sess = s.sessions[s.order[0]]
	}
}

// terminalGone drops one dead terminal. Only the LAST one leaving ends the
// process — a machine server must not die because one project closed.
func (s *Server) terminalGone(name string) {
	s.drop(name)
	left := s.sessionNames()
	log.Printf("[expose] terminal %q is gone — %d session(s) left", name, len(left))
	if len(left) > 0 {
		s.schedulePush()
		return
	}
	log.Printf("[expose] no sessions left — shutting down")
	os.Exit(0)
}

func (s *Server) sessionNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

const noSessions = "No terminals here.\nRun cardputerme in\na project, or press\nfn+esc to switch."

// pushNoSessions renders a machine that owns no terminals. The picker still
// works from here, so the user is never stranded.
func (s *Server) pushNoSessions(force bool) {
	s.mu.Lock()
	st := stateResult{lines: s.screenLines(noSessions), status: "[" + s.cfg.Name + "]  no sessions", size: s.size}
	if s.picking {
		st = s.composeMirror(nil, "", "", false)
	}
	sg := sig(st)
	changed := sg != s.lastNoSig || force
	s.lastNoSig = sg
	msg := displayMessage(st)
	s.mu.Unlock()
	if changed {
		s.hub.broadcastFrame(msg)
	}
}

func (s *Server) currentName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess == nil {
		return ""
	}
	return s.sess.name
}

func newSession(name string, backend *terminal.Backend) *session {
	return &session{
		name:    name,
		backend: backend,
		view:    screen.View{Follow: true, SelRow: -1},
		hist:    -1,
	}
}
