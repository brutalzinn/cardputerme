package server

import (
	"fmt"
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
	ledWaiting   = led{R: 255, G: 60, B: 0, Pattern: "solid"}
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

func (s *Server) setLed(l led) {
	msg := ledMessage(l)
	s.mu.Lock()
	same := msg == s.lastLed
	s.lastLed = msg
	s.mu.Unlock()
	if same {
		return
	}
	s.hub.broadcast(msg)
}

func (s *Server) playNotifySound() {
	if url := s.soundURL(s.cfg.NotifySound); url != "" {
		s.hub.broadcast(soundMessage(url))
		return
	}
	s.hub.broadcast(toneMessage(1200, 90))
}

func (s *Server) signalAttention() {
	s.setLed(ledAttention)
	s.playNotifySound()
}

func (s *Server) soundHandler() http.Handler {
	if s.cfg.SoundsDir == "" {
		return http.NotFoundHandler()
	}
	fs := http.FileServer(http.Dir(s.cfg.SoundsDir))
	return http.StripPrefix(soundPrefix, fs)
}
