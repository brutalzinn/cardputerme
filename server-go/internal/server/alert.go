package server

import (
	"log"
	"net/http"
	"strconv"
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

// maxAlerts bounds the inbox. A pager that remembers everything forever is a
// log; what the user needs is the newest line and an honest count.
const maxAlerts = 8

// awaitingAlert is what the server says when IT noticed rather than being told.
// Terse on purpose: prefixed with a session name it still has to fit alertWidth,
// and "gitme: waiting for y" is worse than saying less.
const awaitingAlert = "waiting"

type alertRequest struct {
	Session string `json:"session"`
	Text    string `json:"text"`
	Level   string `json:"level"`
}

// alert is one unanswered request for the human.
type alert struct {
	session string
	line    string
	at      time.Time
}

// raiseAlert is the ONE way anything asks for the human — Claude Code, a CI job,
// a cron, or the server's own prompt detector. The server deliberately learns
// nothing about the caller.
//
// Queuing is unconditional; only the NOISE is switched. `;notify 0` used to
// discard the alert outright, so silencing the device also destroyed the record
// of who wanted you — silence must mean "make no noise", never "throw away".
func (s *Server) raiseAlert(session, text string, l level) bool {
	line := alertLine(session, text)
	s.queueAlert(session, clip(line, alertWidth))
	if !s.NotifyEnabled() {
		log.Printf("[notify] %q queued silently (;notify 0)", line)
		s.schedulePush()
		return false
	}
	log.Printf("[notify] %s", line)
	// Waking is part of the alert, not a nicety: the text is only readable on a
	// lit screen, and the ADV cannot light its LED while the backlight is off.
	s.applyPower(s.power.Wake(time.Now()))
	s.signalAttention(l)
	s.schedulePush()
	return true
}

// queueAlert appends, collapsing an exact repeat of the newest entry. Without
// that a flapping detector — or a caller in a loop — fills the inbox in seconds
// and the count stops meaning anything. The live log showed the same alert five
// times in a row.
func (s *Server) queueAlert(session, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.alerts); n > 0 && s.alerts[n-1].session == session && s.alerts[n-1].line == line {
		s.alerts[n-1].at = time.Now()
		return
	}
	s.alerts = append(s.alerts, alert{session: session, line: line, at: time.Now()})
	if len(s.alerts) > maxAlerts {
		s.alerts = s.alerts[len(s.alerts)-maxAlerts:]
	}
}

func (s *Server) waiting() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.alerts)
}

// clearSession drops one session's alerts. Looking at a session IS the answer to
// it, so arriving there should not leave it nagging — and must not silence the
// projects you have not looked at yet.
func (s *Server) clearSession(name string) {
	s.mu.Lock()
	kept := []alert{}
	for _, a := range s.alerts {
		if a.session != name {
			kept = append(kept, a)
		}
	}
	changed := len(kept) != len(s.alerts)
	s.alerts = kept
	s.mu.Unlock()
	if changed {
		s.schedulePush()
	}
}

// alertTextLocked is what the device shows: the newest line, plus a count of
// everything else still waiting. "3 waiting" is nine characters the 39-column
// status bar cannot afford, so it is "!3".
func (s *Server) alertTextLocked() string {
	n := len(s.alerts)
	if n == 0 {
		return ""
	}
	newest := s.alerts[n-1].line
	if n == 1 {
		return newest
	}
	return newest + " !" + strconv.Itoa(n)
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
	signalled := s.raiseAlert(strings.TrimSpace(req.Session), strings.TrimSpace(req.Text), levelOf(req.Level))
	// Honest: a broadcast into an empty client map is silently discarded, so
	// "the switch is on" was never the same fact as "a device got it". A pager
	// that lies about delivery is worse than one that fails loudly.
	clients := s.hub.count()
	delivered := signalled && clients > 0
	writeJSON(w, http.StatusOK, map[string]any{
		"delivered": delivered,
		"queued":    true,
		"waiting":   s.waiting(),
		"clients":   clients,
		"reason":    alertReason(delivered, s.NotifyEnabled(), clients),
	})
}

func alertReason(delivered, notify bool, clients int) string {
	if delivered {
		return ""
	}
	if !notify {
		return "silenced by ;notify 0"
	}
	if clients == 0 {
		return "no device connected"
	}
	return ""
}
