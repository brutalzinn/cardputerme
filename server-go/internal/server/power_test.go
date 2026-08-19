package server

import (
	"testing"
	"time"

	"cardputerme/internal/power"
)

func sleepyServer() *Server {
	return withSession(New(Config{
		Name: "test", WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40, Notify: true,
		DimAfter: 30 * time.Second, OffAfter: 120 * time.Second,
	}))
}

func TestPowerMessageWire(t *testing.T) {
	if got := powerMessage(power.Off); got != `{"type":"power","state":"off"}` {
		t.Fatalf("got %s", got)
	}
	if got := powerMessage(power.On); got != `{"type":"power","state":"on"}` {
		t.Fatalf("got %s", got)
	}
	if got := powerMessage(power.Dim); got != `{"type":"power","state":"dim"}` {
		t.Fatalf("got %s", got)
	}
}

func TestServerStartsAwake(t *testing.T) {
	if got := sleepyServer().power.State(); got != power.On {
		t.Fatalf("a fresh exposure is lit, got %q", got)
	}
}

func TestConfiguredTimeoutsReachTheTracker(t *testing.T) {
	s := sleepyServer()
	if got := s.power.Until(time.Now()); got > 30*time.Second || got < 29*time.Second {
		t.Fatalf("first deadline should be the configured dim one, got %v", got)
	}
}

func TestUnconfiguredExposureNeverSleepsByItself(t *testing.T) {
	s := testServer()
	if got := s.power.Until(time.Now()); got != 0 {
		t.Fatalf("no rung is armed, got %v", got)
	}
	if st, _ := s.power.At(time.Now().Add(24 * time.Hour)); st != power.On {
		t.Fatalf("still lit a day later, got %q", st)
	}
	if st, _ := s.power.Sleep(time.Now()); st != power.Off {
		t.Fatalf("a deliberate press still sleeps it, got %q", st)
	}
}

func TestSleepAndWakeRoundTrip(t *testing.T) {
	s := sleepyServer()
	now := time.Now()
	if st, changed := s.power.Sleep(now); st != power.Off || !changed {
		t.Fatalf("sleep got %q changed=%v", st, changed)
	}
	if st, changed := s.power.Wake(now); st != power.On || !changed {
		t.Fatalf("wake got %q changed=%v", st, changed)
	}
}
