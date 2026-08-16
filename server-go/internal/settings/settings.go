package settings

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// Settings is what the user may change from the device and expects to still be
// true tomorrow. Deliberately small: adding a key later is trivial, removing a
// published one is not.
type Settings struct {
	Notify bool `json:"notify"`
}

func Defaults() Settings {
	return Settings{Notify: true}
}

// Load never fails. A missing file is the normal first run, and a corrupt one
// must not silently disable alerts — both fall back to defaults, the corrupt
// case loudly. Unknown keys are ignored, so an older binary tolerates a newer file.
func Load(path string) Settings {
	s := Defaults()
	raw, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Printf("[settings] %s is not valid json (%v) — using defaults", path, err)
		return Defaults()
	}
	return s
}

// Save writes via temp+rename so a crash mid-write cannot leave a half file
// that the next Load would have to reject.
func (s Settings) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
