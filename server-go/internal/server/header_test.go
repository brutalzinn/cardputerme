package server

import (
	"strings"
	"testing"

	"cardputerme/internal/screen"
)

func headerText(cells []screen.Cell) string {
	parts := []string{}
	for _, c := range cells {
		parts = append(parts, c.Text)
	}
	return strings.Join(parts, "|")
}

func TestHeaderShowsWifi(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiUp := true
	s.applyWifi(&wifiUp)
	if got := headerText(s.headerCells()); !strings.Contains(got, "WiFi") {
		t.Fatalf("header = %q", got)
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

// The battery moved entirely off the header (user, 2026-08-17): the device
// now reads, derives and draws its own battery and charging state directly,
// so it stays visible with no session and no server at all. The header must
// never carry a percentage again — that would be two sources of truth.
func TestHeaderNoLongerComposesBattery(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiUp := true
	s.applyWifi(&wifiUp)
	if got := headerText(s.headerCells()); strings.Contains(got, "%") {
		t.Fatalf("header must not compose a battery cell anymore, got %q", got)
	}
}

// Changing what the header says must be a SERVER change. This is the whole
// acceptance test for #45.
func TestHeaderIsComposedEntirelyServerSide(t *testing.T) {
	s := withSession(New(Config{Name: "h", Session: "h", WrapCols: 20}))
	wifiUp := true
	s.applyWifi(&wifiUp)
	st := s.composeMirror(nil, "status", "", false)
	if len(st.header) == 0 {
		t.Fatal("every frame must carry the header the device is to draw")
	}
}
