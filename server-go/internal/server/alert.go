package server

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// defaultAlert is what a caller with nothing to say still gets. Silence would
// make an alert indistinguishable from a bug.
const defaultAlert = "needs you"

// alertWidth keeps the alert from eating the rest of the header. The device
// header holds ~40 characters at text size 1, and WiFi plus the battery take
// about eleven of them.
const alertWidth = 26

// awaitingAlert is what the server says when IT noticed rather than being told.
// Terse on purpose: prefixed with a session name it still has to fit alertWidth,
// and "gitme: waiting for y" is worse than saying less.
const awaitingAlert = "waiting"

type alertRequest struct {
	Session string `json:"session"`
	Text    string `json:"text"`
}

// raiseAlert is the ONE way anything outside the server asks for the human:
// Claude Code, a CI job, a long build, a cron. The server deliberately learns
// nothing about the caller — it is handed a name and a line of text, and the
// policy for what to do with them lives here.
func (s *Server) raiseAlert(session, text string) bool {
	line := clip(alertLine(session, text), alertWidth)
	if !s.NotifyEnabled() {
		log.Printf("[notify] %q silenced by ;notify 0", line)
		return false
	}
	s.mu.Lock()
	s.alert = line
	s.mu.Unlock()
	log.Printf("[notify] %s", line)
	// Waking is part of the alert, not a nicety: the text is only readable on a
	// lit screen, and the ADV cannot light its LED while the backlight is off.
	s.applyPower(s.power.Wake(time.Now()))
	s.signalAttention()
	s.schedulePush()
	return true
}

func (s *Server) clearAlert() {
	s.mu.Lock()
	had := s.alert != ""
	s.alert = ""
	s.mu.Unlock()
	if had {
		s.schedulePush()
	}
}

func alertLine(session, text string) string {
	if text == "" {
		text = defaultAlert
	}
	if session == "" {
		return text
	}
	return session + ": " + text
}

func (s *Server) notifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST only"})
		return
	}
	var req alertRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	delivered := s.raiseAlert(strings.TrimSpace(req.Session), strings.TrimSpace(req.Text))
	// One field, because there is one fact: did the alert go out. Reporting the
	// switch separately said the same thing twice.
	writeJSON(w, http.StatusOK, map[string]any{"delivered": delivered})
}
