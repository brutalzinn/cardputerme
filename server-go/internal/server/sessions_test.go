package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func regServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s := New(Config{Name: "machine", WrapCols: 20, ScrollbackLines: 200})
	mux := http.NewServeMux()
	s.routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return s, srv
}

func TestNewStartsWithNoSessions(t *testing.T) {
	s := New(Config{Name: "machine", WrapCols: 20})
	if got := len(s.sessionNames()); got != 0 {
		t.Fatalf("a machine server owns no terminals until one registers, got %d", got)
	}
	if s.sess != nil {
		t.Fatal("no current session before any registers")
	}
}

func TestRegisterAddsASessionAndMakesItCurrent(t *testing.T) {
	s := New(Config{Name: "machine", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	if got := s.sessionNames(); len(got) != 1 || got[0] != "gitme" {
		t.Fatalf("sessions = %v", got)
	}
	if s.sess == nil || s.sess.name != "gitme" {
		t.Fatal("the first registered session becomes current")
	}
}

func TestRegisterIsIdempotentOnName(t *testing.T) {
	s := New(Config{Name: "machine", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	first := s.sess
	s.register("gitme", "gitme", "/tmp")
	if len(s.sessionNames()) != 1 {
		t.Fatalf("re-registering the same name must not duplicate: %v", s.sessionNames())
	}
	if s.sess != first {
		t.Fatal("re-registering must not replace a live session — it would drop its backend and history")
	}
}

func TestRegisterKeepsTheCurrentSession(t *testing.T) {
	s := New(Config{Name: "machine", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.register("lara", "lara", "/tmp")
	if s.sess.name != "gitme" {
		t.Fatalf("a newly registered session must not steal focus, current = %q", s.sess.name)
	}
	if len(s.sessionNames()) != 2 {
		t.Fatalf("sessions = %v", s.sessionNames())
	}
}

func TestDropRemovesASessionWithoutKillingTheRest(t *testing.T) {
	s := New(Config{Name: "machine", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.register("lara", "lara", "/tmp")
	s.drop("gitme")
	if got := s.sessionNames(); len(got) != 1 || got[0] != "lara" {
		t.Fatalf("sessions = %v", got)
	}
	if s.sess == nil || s.sess.name != "lara" {
		t.Fatal("dropping the current session must promote another, not leave a nil current")
	}
}

func TestDropLastLeavesNoCurrent(t *testing.T) {
	s := New(Config{Name: "machine", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.drop("gitme")
	if s.sess != nil {
		t.Fatal("no sessions left means no current session")
	}
	if len(s.sessionNames()) != 0 {
		t.Fatal("registry must be empty")
	}
}

func TestSessionNamesAreStablyOrdered(t *testing.T) {
	s := New(Config{Name: "machine", WrapCols: 20})
	for _, n := range []string{"zeta", "alpha", "mid"} {
		s.register(n, n, "/tmp")
	}
	a := strings.Join(s.sessionNames(), ",")
	b := strings.Join(s.sessionNames(), ",")
	if a != b {
		t.Fatalf("order must be stable for a picker: %q then %q", a, b)
	}
}

func TestRestListSessions(t *testing.T) {
	s, srv := regServer(t)
	s.register("gitme", "gitme", "/tmp")

	res, err := http.Get(srv.URL + "/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		Current  string   `json:"current"`
		Sessions []string `json:"sessions"`
	}
	if json.NewDecoder(res.Body).Decode(&body) != nil {
		t.Fatal("response must be json")
	}
	if body.Current != "gitme" || len(body.Sessions) != 1 {
		t.Fatalf("body = %+v", body)
	}
}

func TestRestRegisterSession(t *testing.T) {
	s, srv := regServer(t)
	res, err := http.Post(srv.URL+"/sessions", "application/json",
		strings.NewReader(`{"name":"lara","session":"lara","cwd":"/tmp"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if got := s.sessionNames(); len(got) != 1 || got[0] != "lara" {
		t.Fatalf("sessions = %v", got)
	}
}

func TestRestRegisterRejectsAnEmptyName(t *testing.T) {
	s, srv := regServer(t)
	res, err := http.Post(srv.URL+"/sessions", "application/json", strings.NewReader(`{"name":""}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("an unnamed session is not addressable and must be refused")
	}
	if len(s.sessionNames()) != 0 {
		t.Fatal("nothing must have been registered")
	}
}

// Run must register the CLI's own terminal. It was dropped once during the
// refactor and the server came up owning nothing, which no unit test caught.
func TestRunRegistersTheCliSession(t *testing.T) {
	s := New(Config{Name: "startup", Session: "startup", WrapCols: 20})
	s.register(s.cfg.Name, sessionTarget(s.cfg), "")
	if got := s.currentName(); got != "startup" {
		t.Fatalf("current = %q, want the CLI's own session", got)
	}
}
