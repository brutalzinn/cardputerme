package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cardputerme/internal/battery"
)

func health(t *testing.T, s *Server) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	s.routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return out
}

// `report.wifi` exists only in firmware from 42b545c onward — the same build
// that started drawing server-composed header cells. So its presence is a free,
// exact probe for "can this device render the header", with no flash and no new
// wire field. Without it we are guessing which binary is on the device, and a
// blank top bar is unattributable.
func TestServerLearnsWhetherTheDeviceRendersTheHeader(t *testing.T) {
	s := New(Config{Name: "h", WrapCols: 20})
	if s.deviceRendersHeader() {
		t.Fatal("nothing is known until the device has reported")
	}
	s.applyWifi(nil)
	if s.deviceRendersHeader() {
		t.Fatal("a report with no wifi field is firmware that predates the header")
	}
	up := true
	s.applyWifi(&up)
	if !s.deviceRendersHeader() {
		t.Fatal("a reported wifi fact means the device draws what we compose")
	}
}

// A legacy device must not be mistaken for a disconnected radio: the capability
// is recorded from the field's PRESENCE, the value only from its contents.
func TestLegacyFirmwareIsNotReadAsNoWifi(t *testing.T) {
	s := New(Config{Name: "h", WrapCols: 20})
	s.applyWifi(nil)
	if !s.wifi {
		t.Fatal("an absent fact must leave the wifi value alone")
	}
}

// The pager must be able to report its own liveness without the device in hand.
// Diagnosing a blank screen by asking the user what they see is not a workflow.
func TestHealthReportsWhatTheDeviceIsDoing(t *testing.T) {
	s := New(Config{Name: "h", WrapCols: 20})
	s.gauge.Observe(battery.Reading{Millivolts: 3700, At: time.Now()})
	s.handleReport(false, 3700, 88, time.Now())

	got := health(t, s)
	for _, key := range []string{"battery", "mv", "external", "clients", "renders_header", "device_battery"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("/health is missing %q: %v", key, got)
		}
	}
	if got["battery"] != "50%" {
		t.Fatalf("battery = %v", got["battery"])
	}
	if got["mv"] != float64(3700) {
		t.Fatalf("mv = %v", got["mv"])
	}
	if got["clients"] != float64(0) {
		t.Fatalf("clients = %v", got["clients"])
	}
}

// The device computes its own percentage and we deliberately ignore it (#37).
// Keeping it visible in /health is the cross-check: if ours and theirs disagree
// wildly the gauge is wrong, and if both are blank the device is.
func TestHealthKeepsTheDeviceOwnPercentageAsACrossCheck(t *testing.T) {
	s := New(Config{Name: "h", WrapCols: 20})
	s.handleReport(false, 3700, 88, time.Now())
	if got := health(t, s)["device_battery"]; got != float64(88) {
		t.Fatalf("device_battery = %v", got)
	}
}
