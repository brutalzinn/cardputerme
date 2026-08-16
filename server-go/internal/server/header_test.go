package server

import (
	"strings"
	"testing"
	"time"

	"cardputerme/internal/battery"
	"cardputerme/internal/screen"
)

func headerText(cells []screen.Cell) string {
	parts := []string{}
	for _, c := range cells {
		parts = append(parts, c.Text)
	}
	return strings.Join(parts, "|")
}

func TestHeaderShowsWifiAndBattery(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	s.gauge.Observe(battery.Reading{Millivolts: 3700, At: time.Now()})
	wifiUp := true
	s.applyWifi(&wifiUp)

	got := headerText(s.headerCells())
	if !strings.Contains(got, "WiFi") {
		t.Fatalf("header = %q", got)
	}
	if !strings.Contains(got, "50%") {
		t.Fatalf("battery must be in the header, got %q", got)
	}
}

func TestHeaderShowsWifiDown(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiDown := false
	s.applyWifi(&wifiDown)
	if got := headerText(s.headerCells()); !strings.Contains(got, "NoWiFi") {
		t.Fatalf("header = %q", got)
	}
}

// The device knows whether WiFi is up; it must REPORT that fact. Whether and
// how to show it is the server's call.
func TestWifiIsAReportedFactNotADeviceDecision(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiDown := false
	s.applyWifi(&wifiDown)
	down := headerText(s.headerCells())
	wifiUp := true
	s.applyWifi(&wifiUp)
	up := headerText(s.headerCells())
	if down == up {
		t.Fatal("the reported fact must change what the server composes")
	}
}

// A report from firmware that predates the wifi field must not read as "the
// radio is down" — an absent fact is unknown, not false.
func TestAbsentWifiFactLeavesTheHeaderAlone(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiUp := true
	s.applyWifi(&wifiUp)
	s.applyWifi(nil)
	if got := headerText(s.headerCells()); strings.Contains(got, "NoWiFi") {
		t.Fatalf("a silent device is not a disconnected one, got %q", got)
	}
	down := false
	s.applyWifi(&down)
	if got := headerText(s.headerCells()); !strings.Contains(got, "NoWiFi") {
		t.Fatalf("a reported false must land, got %q", got)
	}
}

// The battery is ALWAYS on screen (user, 2026-08-16) — a header that shows it
// only sometimes is one you cannot trust at a glance.
func TestBatteryIsAlwaysInTheHeader(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiUp := true
	s.applyWifi(&wifiUp)
	if got := headerText(s.headerCells()); !strings.Contains(got, unknownBattery) {
		t.Fatalf("with no reading yet the slot must still be there, got %q", got)
	}
	s.gauge.Observe(battery.Reading{Millivolts: 3700, At: time.Now()})
	if got := headerText(s.headerCells()); !strings.Contains(got, "50%") {
		t.Fatalf("header = %q", got)
	}
}

// Always showing it must never mean inventing a number: an unread battery is
// not a flat one, and 0% would be a lie the user acts on.
func TestUnknownBatteryIsNotAFakeNumber(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	if got := headerText(s.headerCells()); strings.Contains(got, "0%") {
		t.Fatalf("header = %q", got)
	}
}

// Changing what the header says must be a SERVER change. This is the whole
// acceptance test for #45.
func TestHeaderIsComposedEntirelyServerSide(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiUp := true
	s.applyWifi(&wifiUp)
	s.gauge.Observe(battery.Reading{Millivolts: 3700, At: time.Now()})
	st := s.composeMirror(nil, "status", "", false)
	if len(st.header) == 0 {
		t.Fatal("every frame must carry the header the device is to draw")
	}
}
