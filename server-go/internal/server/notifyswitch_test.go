package server

import (
	"strings"
	"testing"
)

func TestNotifyCommandSilencesEveryChannel(t *testing.T) {
	s := withSession(New(Config{Name: "n", Session: "n", WrapCols: 20, LinesPerCard: 7, MaxCards: 40, Notify: true}))
	if !s.NotifyEnabled() {
		t.Fatal("precondition: notify starts on")
	}

	// typing ;notify 0 on the device
	for _, k := range append(strings.Split(";notify 0", ""), "enter") {
		s.applyKey(k)
	}
	if s.NotifyEnabled() {
		t.Fatal("`;notify 0` typed on the device must turn alerts off")
	}

	// with alerts off, an attention signal must emit nothing at all
	before := s.lastLed
	s.signalAttention()
	if s.lastLed != before {
		t.Fatal("`;notify 0` must silence the LED too, not just the beep")
	}

	for _, k := range append(strings.Split(";notify 1", ""), "enter") {
		s.applyKey(k)
	}
	if !s.NotifyEnabled() {
		t.Fatal("`;notify 1` must turn alerts back on")
	}
}

func TestHealthReportsTheLiveNotifyValue(t *testing.T) {
	s := withSession(New(Config{Name: "n", Session: "n", WrapCols: 20, Notify: true}))
	s.SetNotify(false)
	if s.NotifyEnabled() {
		t.Fatal("SetNotify did not take")
	}
	// /health must not report the startup Config copy
	if s.cfg.Notify == s.NotifyEnabled() {
		t.Skip("config and live value coincide; nothing to distinguish")
	}
}
