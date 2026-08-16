package server

import (
	"os"
	"path/filepath"
	"strconv"
)

const portFile = "server.port"
const settingsFile = "settings.json"

func settingsPath() string { return filepath.Join(stateDir(), settingsFile) }

func stateDir() string {
	if dir := os.Getenv("CARDPUTERME_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cardputerme"
	}
	return filepath.Join(home, ".cardputerme")
}

// publishPort tells the CLI where this machine's server is listening. Written
// as a bare number so the launcher needs no JSON tooling.
func publishPort(port int) error {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, portFile), []byte(strconv.Itoa(port)+"\n"), 0o644)
}
