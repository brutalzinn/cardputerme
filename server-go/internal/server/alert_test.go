package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cardputerme/internal/power"
)

// alertServer matches how the CLI builds one: notifications ON unless NOTIFY=0.
func alertServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s := New(Config{Name: "machine", WrapCols: 20, ScrollbackLines: 200, Notify: true})
	mux := http.NewServeMux()
	s.routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return s, srv
}

func postNotify(t *testing.T, srv string, body string) (int, map[string]any) {
	t.Helper()
	res, err := http.Post(srv+"/notify", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// The whole point: something OUTSIDE the server — Claude Code, a CI job, a
// cron — asks for the human. The server never learns what any of them are.
func TestAnyProgramCanRaiseAnAlert(t *testing.T) {
	s, srv := alertServer(t)
	code, body := postNotify(t, srv.URL, `{"session":"gitme","text":"stuck on tests"}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if body["delivered"] != true {
		t.Fatalf("body = %v", body)
	}
	if got := headerText(s.headerCells()); !strings.Contains(got, "stuck on tests") {
		t.Fatalf("the alert must be visible on the device, header = %q", got)
	}
	if s.lastLed != ledMessage(ledAttention) {
		t.Fatalf("led = %q", s.lastLed)
	}
}

func TestAlertNamesTheSessionThatWantsYou(t *testing.T) {
	s, srv := alertServer(t)
	postNotify(t, srv.URL, `{"session":"gitme","text":"stuck"}`)
	if got := headerText(s.headerCells()); !strings.Contains(got, "gitme") {
		t.Fatalf("with many sessions the alert is useless without its name, header = %q", got)
	}
}

// A caller with nothing to say still gets the human's attention.
func TestAlertWithoutTextStillAlerts(t *testing.T) {
	s, srv := alertServer(t)
	_, body := postNotify(t, srv.URL, `{}`)
	if body["delivered"] != true {
		t.Fatalf("body = %v", body)
	}
	if got := headerText(s.headerCells()); !strings.Contains(got, defaultAlert) {
		t.Fatalf("header = %q", got)
	}
}

// `;notify 0` is ONE switch over every channel (#40). An API that could shout
// past it would make the switch a lie.
func TestNotifyOffSilencesTheApi(t *testing.T) {
	s, srv := alertServer(t)
	s.SetNotify(false)
	code, body := postNotify(t, srv.URL, `{"text":"stuck on tests"}`)
	if code != http.StatusOK {
		t.Fatalf("a silenced alert is still a handled request, got %d", code)
	}
	if body["delivered"] != false {
		t.Fatalf("the caller must be told it was silenced, body = %v", body)
	}
	if got := headerText(s.headerCells()); strings.Contains(got, "stuck on tests") {
		t.Fatalf("header = %q", got)
	}
	if s.lastLed == ledMessage(ledAttention) {
		t.Fatal("silence means the LED too")
	}
}

// An alert nobody can read is not an alert: the screen must come up.
func TestAlertWakesADarkScreen(t *testing.T) {
	s, srv := alertServer(t)
	s.applyPower(s.power.Sleep(time.Now()))
	if s.power.State() != power.Off {
		t.Fatal("precondition: the screen is dark")
	}
	postNotify(t, srv.URL, `{"text":"stuck"}`)
	if s.power.State() != power.On {
		t.Fatalf("power = %v", s.power.State())
	}
}

// "I am here" — the same keypress that stops the sound and darkens the LED
// clears the text, so the header cannot keep nagging after you looked.
func TestAKeypressClearsTheAlert(t *testing.T) {
	s, srv := alertServer(t)
	postNotify(t, srv.URL, `{"text":"stuck on tests"}`)
	s.dismissAlert()
	if got := headerText(s.headerCells()); strings.Contains(got, "stuck on tests") {
		t.Fatalf("header = %q", got)
	}
}

func TestNotifyApiRejectsGarbage(t *testing.T) {
	_, srv := alertServer(t)
	if code, _ := postNotify(t, srv.URL, `not json`); code != http.StatusBadRequest {
		t.Fatalf("status = %d", code)
	}
}

func TestNotifyApiIsPostOnly(t *testing.T) {
	_, srv := alertServer(t)
	res, err := http.Get(srv.URL + "/notify")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

// The header is ~40 characters at text size 1. A caller pasting a stack trace
// must not push the battery off the screen.
func TestALongAlertIsClippedNotAllowedToEatTheHeader(t *testing.T) {
	s, srv := alertServer(t)
	postNotify(t, srv.URL, `{"text":"`+strings.Repeat("x", 400)+`"}`)
	cells := s.headerCells()
	if len(cells[0].Text) > alertWidth+1 {
		t.Fatalf("alert cell is %d chars", len(cells[0].Text))
	}
	if !strings.Contains(headerText(cells), "WiFi") {
		t.Fatalf("the rest of the header must survive, got %q", headerText(cells))
	}
}
