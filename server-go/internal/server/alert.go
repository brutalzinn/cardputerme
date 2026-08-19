package server

import (
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"cardputerme/internal/screen"
)

// defaultAlert is what a caller with nothing to say still gets. Silence would
// make an alert indistinguishable from a bug.
const defaultAlert = "needs you"

// headerCols is what the device header holds at text size 1: 240px over a 6px
// font. A server-side constant standing in for a device fact — the device should
// report it, the way it reports usb/mv/wifi. Until it does, changing the board
// or the font silently mis-clips.
const headerCols = 40

// alertWidth is what is left for the alert once the widest possible WiFi cell
// ("NoWiFi", 6) plus a separating space are taken out of headerCols. The
// battery no longer shares this header at all — the device draws its own —
// so this budget grew by the 8 characters that cell used to cost. The "!n"
// count lives INSIDE this budget — it was added afterwards and once pushed
// the battery off the screen, back when the battery was still here.
const alertWidth = headerCols - 6 - 1

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

// alert is one unanswered request for the human. The line is kept in FULL:
// clipping at store time destroyed the text forever, so no wider render — or
// /health — could ever recover it. Width is a render-time concern.
type alert struct {
	session string
	line    string
}

// raiseAlert is the ONE way anything asks for the human — Claude Code, a CI job,
// a cron, or the server's own prompt detector. The server deliberately learns
// nothing about the caller.
//
// Queuing is unconditional; only the NOISE is switched. `;notify 0` used to
// discard the alert outright, so silencing the device also destroyed the record
// of who wanted you — silence must mean "make no noise", never "throw away".
func (s *Server) raiseAlert(session, text string, l level) bool {
	line := screen.ToAscii(alertLine(session, text))
	s.queueAlert(session, line)
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
		return
	}
	s.alerts = append(s.alerts, alert{session: session, line: line})
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
	before := len(s.alerts)
	s.alerts = slices.DeleteFunc(s.alerts, func(a alert) bool { return s.answeredLocked(a, name) })
	changed := len(s.alerts) != before
	s.mu.Unlock()
	if changed {
		s.schedulePush()
	}
}

// answeredLocked decides what a keypress just answered. The session on screen,
// obviously — but also any alert naming a session this machine does not have.
// Those are UNADDRESSABLE: raiseAlert accepts any name, so a CI job or a cron
// can post one, and there is no session to switch to and clear it. Without this
// they were immortal, inflating the count and holding a dead line in the header
// forever.
func (s *Server) answeredLocked(a alert, viewing string) bool {
	if a.session == viewing {
		return true
	}
	_, known := s.sessions[a.session]
	return !known
}

// alertTextLocked is what the device shows: the newest line, plus a count of
// everything else still waiting. "3 waiting" is nine characters the header
// cannot afford, so it is "!3" — and the count is inside the width budget, not
// added on top of it.
func (s *Server) alertTextLocked() string {
	n := len(s.alerts)
	if n == 0 {
		return ""
	}
	suffix := ""
	if n > 1 {
		suffix = " !" + strconv.Itoa(n)
	}
	return clip(s.alerts[n-1].line, alertWidth-len([]rune(suffix))) + suffix
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
