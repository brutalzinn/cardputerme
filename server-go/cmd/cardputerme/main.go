// Command cardputerme exposes one terminal to an M5Cardputer over WebSocket,
// discoverable on the LAN via a UDP beacon. One process per exposed terminal.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
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

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(0)
	cwd, _ := os.Getwd()
	name := sessionName(os.Args[1:], os.Getenv, envStr("SESSION_CWD", cwd))
	cfg := server.Config{
		Name:            name,
		Session:         envStr("TMUX_SESSION", envStr("SESSION", name)),
		SessionCwd:      envStr("SESSION_CWD", cwd),
		WrapCols:        envInt("WRAP_COLS", 20),
		LinesPerCard:    envInt("LINES_PER_CARD", 7),
		ScrollbackLines: envInt("SCROLLBACK_LINES", 200),
		MaxCards:        envInt("MAX_CARDS", 40),
		Notify:          os.Getenv("NOTIFY") != "0",
		DimAfter:        time.Duration(envInt("DIM_AFTER_S", 30)) * time.Second,
		OffAfter:        time.Duration(envInt("OFF_AFTER_S", 120)) * time.Second,
	}
	if err := server.New(cfg).Run(); err != nil {
		log.Fatal(err)
	}
}
