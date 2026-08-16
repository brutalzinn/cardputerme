package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingFileYieldsDefaults(t *testing.T) {
	s := Load(filepath.Join(t.TempDir(), "settings.json"))
	if !s.Notify {
		t.Fatal("a first run has no file; alerts default ON, not off")
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := Load(path)
	s.Notify = false
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	if again := Load(path); again.Notify {
		t.Fatal("`;notify 0` must survive a restart — that is the whole point")
	}
}

func TestCorruptFileFallsBackToDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Load(path)
	if !s.Notify {
		t.Fatal("a corrupt file must not silently disable alerts; fall back to defaults")
	}
}

func TestUnknownKeysAreIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"notify":false,"fromTheFuture":123}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Load(path)
	if s.Notify {
		t.Fatal("known keys must still apply")
	}
}

func TestSaveCreatesTheDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")
	s := Defaults()
	s.Notify = false
	if err := s.Save(path); err != nil {
		t.Fatalf("a first save has no state dir yet: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

// Written via temp+rename, so a crash mid-write can never leave a half file
// that Load would then reject.
func TestSaveIsAtomicAndLeavesNoTempBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	s := Defaults()
	for range 5 {
		if err := s.Save(path); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("temp files were left behind: %v", names)
	}
}
