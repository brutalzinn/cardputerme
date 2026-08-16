package server

import (
	"encoding/json"
	"log"
	"net/http"
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
	stop         func()
	check        deadline
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ws", s.wsHandler)
	mux.HandleFunc("/sessions", s.sessionsHandler)
	mux.HandleFunc("/notify", s.notifyHandler)
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
	if r.Method == http.MethodDelete {
		name := r.URL.Query().Get("name")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
			return
		}
		s.drop(name)
		log.Printf("[sessions] released %q", name)
		writeJSON(w, http.StatusOK, map[string]any{
			"current":  s.currentName(),
			"sessions": s.sessionNames(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"current":  s.currentName(),
		"sessions": s.sessionNames(),
	})
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if !decodeJSON(w, r, &req) {
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

// decodeJSON replies 400 and reports false when the body is not the shape the
// handler asked for, so handlers stay a guard clause rather than a funnel.
func decodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	if json.NewDecoder(r.Body).Decode(into) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

// register ATTACHES to an existing terminal and starts watching it. It must
// never create one — that is the CLI's job, with the user's cwd. Creating here
// conjures empty tmux sessions for any name anyone posts.
//
// register adds a terminal under name and starts WATCHING it. Without the
// subscribe a registered session would render but never push, so a prompt in
// any project but the current one would go unnoticed — which is the whole
// point of one server owning many sessions.
//
// Idempotent, and it never replaces a live session: doing so would silently
// discard its backend and history.
func (s *Server) register(name, target, cwd string) *session {
	s.mu.Lock()
	if existing, ok := s.sessions[name]; ok {
		s.mu.Unlock()
		return existing
	}
	sess := newSession(name, terminal.CreateBackend(target, s.cfg.ScrollbackLines))
	sess.cwd = cwd
	s.sessions[name] = sess
	s.order = append(s.order, name)
	if s.sess == nil {
		s.sess = sess
	}
	s.mu.Unlock()

	sess.stop = sess.backend.Subscribe(
		func() { s.emit(sessionEvent{name: name, kind: evChanged}) },
		func() { s.emit(sessionEvent{name: name, kind: evGone}) },
	)
	return sess
}

// drop removes a session. A dying terminal must never take the others with it,
// so this promotes a survivor rather than exiting.
func (s *Server) drop(name string) {
	s.mu.Lock()
	sess, ok := s.sessions[name]
	if !ok {
		s.mu.Unlock()
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
	if s.sess == nil || s.sess.name == name {
		s.sess = nil
		if len(s.order) > 0 {
			s.sess = s.sessions[s.order[0]]
		}
	}
	s.mu.Unlock()

	// OUTSIDE the lock, always: stop() waits for the subscribe goroutine, and
	// that goroutine's callback takes mu — holding it here deadlocks the server.
	if sess.stop != nil {
		sess.stop()
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
	s.shutdown()
}

// shutdown ends Run. Signalling beats os.Exit here: a library that kills the
// process cannot be tested, and gives callers no chance to clean up.
func (s *Server) shutdown() {
	s.closeOnce.Do(func() { close(s.done) })
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
	st := stateResult{lines: s.screenLines(noSessions), status: "[" + s.cfg.Name + "]  no sessions", header: s.headerCellsLocked(), size: s.size}
	if s.picking {
		st = s.composeMirror(nil, "", "", false)
	}
	msg := displayMessage(st)
	changed := msg != s.lastNoSig || force
	s.lastNoSig = msg
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
