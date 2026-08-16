package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateDirHonoursTheOverride(t *testing.T) {
	t.Setenv("CARDPUTERME_DIR", "/tmp/cardputerme-test")
	if got := stateDir(); got != "/tmp/cardputerme-test" {
		t.Fatalf("stateDir = %q", got)
	}
}

func TestStateDirDefaultsUnderHome(t *testing.T) {
	t.Setenv("CARDPUTERME_DIR", "")
	got := stateDir()
	if !strings.HasSuffix(got, ".cardputerme") {
		t.Fatalf("stateDir = %q, want ~/.cardputerme", got)
	}
}

// The CLI has to find a running machine server, and shell cannot parse JSON
// without extra tools — so the port is written as a bare number.
func TestPublishPortWritesABareNumber(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARDPUTERME_DIR", dir)
	if err := publishPort(8042); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, portFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != "8042" {
		t.Fatalf("port file = %q, want a bare 8042", raw)
	}
}

func TestPublishPortCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")
	t.Setenv("CARDPUTERME_DIR", dir)
	if err := publishPort(8001); err != nil {
		t.Fatalf("a first run has no state dir yet: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, portFile)); err != nil {
		t.Fatal(err)
	}
}

func TestPublishPortOverwritesAStalePort(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARDPUTERME_DIR", dir)
	if err := publishPort(8001); err != nil {
		t.Fatal(err)
	}
	if err := publishPort(8009); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, portFile))
	if strings.TrimSpace(string(raw)) != "8009" {
		t.Fatalf("a restart on a new port must replace the old one, got %q", raw)
	}
}
