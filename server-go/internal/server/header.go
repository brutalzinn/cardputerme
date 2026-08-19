package server

import (
	"log"

	"cardputerme/internal/screen"
)

// applyWifi takes the fact the device reported. Absent means the firmware
// predates the field, not that the radio is down.
//
// Its PRESENCE is also a free firmware probe: `wifi` arrived in the same build
// that started drawing server-composed header cells, so a device that reports it
// is a device that renders the header. Without this we cannot tell a blank top
// bar caused by old firmware from one caused by a stale server, and asking the
// user what they see is not a diagnosis.
func (s *Server) applyWifi(up *bool) {
	s.noteHeaderCapable(up != nil)
	if up == nil {
		return
	}
	s.mu.Lock()
	s.wifi = *up
	s.mu.Unlock()
}

// deviceKnown exists only so the log fires on a real transition rather than on
// the first report of every reconnect.
func (s *Server) noteHeaderCapable(capable bool) {
	s.mu.Lock()
	known, was := s.deviceKnown, s.deviceHeader
	s.deviceKnown = true
	s.deviceHeader = capable
	s.mu.Unlock()
	if known && was == capable {
		return
	}
	if capable {
		log.Printf("[device] firmware renders server-composed headers")
		return
	}
	log.Printf("[device] firmware PREDATES the composed header — the top bar is drawn on-device and cannot be changed from here; re-flash to fix")
}

func (s *Server) deviceRendersHeader() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deviceHeader
}

func wifiCell(up bool) screen.Cell {
	if up {
		return screen.Cell{Text: "WiFi", Color: screen.Colors.Status}
	}
	return screen.Cell{Text: "NoWiFi", Color: screen.Colors.Ask}
}

func (s *Server) headerCells() []screen.Cell {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headerCellsLocked()
}

// headerCellsLocked composes what the device used to build from local state.
// WiFi is a fact the device REPORTS; whether and how to show it is decided
// here, so it can change without a re-flash (#45). Battery is no longer a
// part of this: the device now reads, derives and draws its own battery and
// charging state directly, so it stays visible with no session and no
// server at all — that's the whole reason it moved off this frame.
func (s *Server) headerCellsLocked() []screen.Cell {
	return s.headerCellsFor()
}

func (s *Server) headerCellsFor() []screen.Cell {
	cells := []screen.Cell{}
	if text := s.alertTextLocked(); text != "" {
		cells = append(cells, screen.Cell{Text: text + " ", Color: screen.Colors.Ask})
	}
	return append(cells, wifiCell(s.wifi))
}
