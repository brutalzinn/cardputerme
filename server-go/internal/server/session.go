package server

import (
	"sync"
	"time"

	"cardputerme/internal/screen"
	"cardputerme/internal/terminal"
)

// session is everything that belongs to ONE mirrored terminal. Device-owned
// state (power, battery, led, repeat, the picker) stays on Server, because it
// belongs to the Cardputer rather than to any terminal.
//
// Still guarded by Server.mu — splitting the lock is a separate step.
type session struct {
	name    string
	backend *terminal.Backend

	input        string
	cmd          string
	reply        string
	hist         int
	history      []string
	view         screen.View
	cache        *mirrorCache
	lastSig      string
	lastAwaiting bool

	pushMu    sync.Mutex
	pushTimer *time.Timer
}

func newSession(name string, backend *terminal.Backend) *session {
	return &session{
		name:    name,
		backend: backend,
		view:    screen.View{Follow: true, SelRow: -1},
		hist:    -1,
	}
}
