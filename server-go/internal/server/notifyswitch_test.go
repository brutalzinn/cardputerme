package server

import (
	"path/filepath"
	"strings"
	"testing"

	"cardputerme/internal/settings"
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
	s.signalAttention(attention)
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

// Tests must never read or write the developer's real state. They did: a test
// calling SetNotify(false) persisted notify=false into ~/.cardputerme, which
// then leaked into unrelated tests. An empty SettingsPath means no disk at all.
func TestNoPersistenceWithoutAnExplicitPath(t *testing.T) {
	s := withSession(New(Config{Name: "n", Session: "n", WrapCols: 20, Notify: true}))
	if s.cfg.SettingsPath != "" {
		t.Fatal("test servers must not be pointed at a real settings file")
	}
	s.SetNotify(false) // must not touch disk
	if s.NotifyEnabled() {
		t.Fatal("the in-memory switch must still work")
	}
}

func TestPersistenceRoundTripsThroughAnExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := withSession(New(Config{Name: "n", Session: "n", WrapCols: 20, Notify: true, SettingsPath: path}))
	s.SetNotify(false)

	again := New(Config{Name: "n", Session: "n", WrapCols: 20, Notify: true, SettingsPath: path})
	if again.NotifyEnabled() {
		t.Fatal("`;notify 0` must survive a restart")
	}
}

func TestAnExplicitEnvOverrideBeatsTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := (settings.Settings{Notify: false}).Save(path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOTIFY", "1")
	s := New(Config{Name: "n", WrapCols: 20, Notify: true, SettingsPath: path})
	if !s.NotifyEnabled() {
		t.Fatal("an explicitly set env var is a deliberate one-off override for this process")
	}
}
