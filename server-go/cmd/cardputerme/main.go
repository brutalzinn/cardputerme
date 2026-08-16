// Command cardputerme exposes one terminal to an M5Cardputer over WebSocket,
// discoverable on the LAN via a UDP beacon. One process per exposed terminal.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cardputerme/internal/server"
)

func sessionName(args []string, env func(string) string, cwd string) string {
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}
	if v := env("NAME"); v != "" {
		return v
	}
	if cwd != "" {
		return filepath.Base(cwd)
	}
	return "cardputerme"
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envList(key, def string) []string {
	out := []string{}
	for _, p := range strings.Split(envStr(key, def), ",") {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func defaultSoundsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cardputerme", "sounds")
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(0)
	wd, _ := os.Getwd()
	cwd := envStr("SESSION_CWD", wd)
	name := sessionName(os.Args[1:], os.Getenv, cwd)
	cfg := server.Config{
		Name:            name,
		Session:         envStr("SESSION", name),
		SessionCwd:      cwd,
		WrapCols:        envInt("WRAP_COLS", 20),
		LinesPerCard:    envInt("LINES_PER_CARD", 7),
		ScrollbackLines: envInt("SCROLLBACK_LINES", 200),
		MaxCards:        envInt("MAX_CARDS", 40),
		Notify:          os.Getenv("NOTIFY") != "0",
		DimAfter:        time.Duration(envInt("DIM_AFTER_S", 30)) * time.Second,
		OffAfter:        time.Duration(envInt("OFF_AFTER_S", 120)) * time.Second,
		RepeatDelay:     time.Duration(envInt("REPEAT_DELAY_MS", 350)) * time.Millisecond,
		RepeatInterval:  time.Duration(envInt("REPEAT_INTERVAL_MS", 90)) * time.Millisecond,
		PushDebounce:    time.Duration(envInt("PUSH_DEBOUNCE_MS", 15)) * time.Millisecond,
		IdleExit:        time.Duration(envInt("IDLE_EXIT_H", 12)) * time.Hour,
		SoundsDir:       envStr("SOUNDS_DIR", defaultSoundsDir()),
		NotifySound:     envStr("NOTIFY_SOUND", "notify.wav"),
		UsbMilliVolts:   envInt("USB_MV", 4200),
		HidePrefixes:    envList("HIDE_PREFIXES", "Tip:"),
	}
	if err := server.New(cfg).Run(); err != nil {
		log.Fatal(err)
	}
}
