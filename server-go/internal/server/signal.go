package server

import (
	"cardputerme/internal/settings"

	"fmt"
	"log"
	"net/http"
	"strconv"
)

const soundPrefix = "/sound/"

type led struct {
	R       int
	G       int
	B       int
	Pattern string
}

var (
	ledOff       = led{}
	ledAttention = led{R: 255, G: 60, B: 0, Pattern: "pulse"}
	ledInfo      = led{R: 0, G: 80, B: 255, Pattern: "solid"}
	ledUrgent    = led{R: 255, G: 0, B: 0, Pattern: "solid"}
)

func ledMessage(l led) string {
	pattern := l.Pattern
	if pattern == "" {
		pattern = "off"
	}
	return fmt.Sprintf(`{"type":"led","r":%d,"g":%d,"b":%d,"pattern":%q}`, l.R, l.G, l.B, pattern)
}

func soundMessage(url string) string {
	return `{"type":"sound","url":` + strconv.Quote(url) + `}`
}

func (s *Server) setSoundBase(hostPort string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.soundBase = hostPort
}

func (s *Server) soundURL(file string) string {
	if s.cfg.SoundsDir == "" {
		return ""
	}
	s.mu.Lock()
	base := s.soundBase
	s.mu.Unlock()
	if base == "" {
		return ""
	}
	return "http://" + base + soundPrefix + file
}

func toneMessage(freq, ms int) string {
	return `{"type":"sound","freq":` + strconv.Itoa(freq) + `,"ms":` + strconv.Itoa(ms) + `}`
}

// NotifyEnabled/SetNotify are the slice of the server that `;notify` may touch.
// Live state, not the startup Config copy — the whole point is changing it from
// the device without restarting the exposure.
func (s *Server) NotifyEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notify
}

func (s *Server) SetNotify(v bool) {
	s.mu.Lock()
	s.notify = v
	s.mu.Unlock()
	log.Printf("[notify] alerts %s", map[bool]string{true: "on", false: "off"}[v])
	if !v {
		s.setLed(ledOff)
	}
	// Persist so the choice survives a restart — the setting is useless if the
	// user must make it again every time the server comes back. An empty path
	// means no persistence, which keeps tests off the real state dir.
	if s.cfg.SettingsPath == "" {
		return
	}
	if err := (settings.Settings{Notify: v}).Save(s.cfg.SettingsPath); err != nil {
		log.Printf("[settings] could not persist notify=%v: %v", v, err)
	}
}

func (s *Server) setLed(l led) {
	msg := ledMessage(l)
	s.mu.Lock()
	same := msg == s.lastLed
	s.lastLed = msg
	s.mu.Unlock()
	if same {
		return
	}
	log.Printf("[led] %s", msg)
	s.hub.broadcast(msg)
}

func (s *Server) resendLed() {
	s.mu.Lock()
	msg := s.lastLed
	s.mu.Unlock()
	if msg == "" {
		msg = ledMessage(ledOff)
	}
	s.hub.broadcast(msg)
}

// playLevel sends the level's sound. A multi-note burst is paced by re-arming a
// deadline from inside its own callback — never a ticker, and `arm(0)` is the
// cancel path a keypress needs.
func (s *Server) playLevel(l level) {
	if l.usesWav() {
		if url := s.soundURL(s.cfg.NotifySound); url != "" {
			s.hub.broadcast(soundMessage(url))
			return
		}
	}
	s.playNotes(l.tones())
}

func (s *Server) playNotes(notes []note) {
	if len(notes) == 0 {
		return
	}
	s.hub.broadcast(toneMessage(notes[0].freq, notes[0].ms))
	rest := notes[1:]
	if len(rest) == 0 {
		return
	}
	s.toneDeadline.arm(burstGap, func() { s.playNotes(rest) })
}

// signalAttention is the ONE place alerts leave the server, so `;notify 0`
// silences every channel — sound and LED — rather than just the beep.
func soundStopMessage() string { return `{"type":"sound","stop":true}` }

// dismissAlert is what "I am here" looks like: a keypress stops the sound and
// darkens the LED. The device decides nothing — it is told to stop.
func (s *Server) dismissAlert() {
	s.mu.Lock()
	lit := s.lastLed != "" && s.lastLed != ledMessage(ledOff)
	s.mu.Unlock()
	if lit {
		s.hub.broadcast(soundStopMessage())
	}
	s.toneDeadline.arm(0, nil)
	s.setLed(ledOff)
	// A keypress means "I am here", not "I have dealt with every project": it
	// answers the session on screen and leaves the others waiting.
	s.clearSession(s.currentName())
}

func (s *Server) signalAttention(l level) {
	if !s.NotifyEnabled() {
		return
	}
	s.setLed(l.led())
	s.playLevel(l)
}

func (s *Server) soundHandler() http.Handler {
	if s.cfg.SoundsDir == "" {
		return http.NotFoundHandler()
	}
	fs := http.FileServer(http.Dir(s.cfg.SoundsDir))
	return http.StripPrefix(soundPrefix, fs)
}
