package server

import (
	"log"

	"cardputerme/internal/battery"
	"cardputerme/internal/screen"
)

// unknownBattery keeps the slot occupied before the device has ever reported.
const unknownBattery = "--%"

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
// here, so it can change without a re-flash (#45).
func (s *Server) headerCellsLocked() []screen.Cell {
	return s.headerCellsFor(s.batteryLabelLocked())
}

func (s *Server) headerCellsFor(bat string) []screen.Cell {
	cells := []screen.Cell{}
	if text := s.alertTextLocked(); text != "" {
		cells = append(cells, screen.Cell{Text: text + " ", Color: screen.Colors.Ask})
	}
	cells = append(cells, wifiCell(s.wifi))
	return append(cells, batteryCell(bat))
}

// batteryLabelLocked is the one place the placeholder decision lives. Both the
// header and the status bar go through it, and a frame reads it ONCE — reading
// the gauge twice let a report land in between and put two different
// percentages in the same frame.
func (s *Server) batteryLabelLocked() string {
	if label := battery.Label(s.gauge.Status()); label != "" {
		return label
	}
	return unknownBattery
}

// batteryCell always occupies the slot: a reading that appears and disappears
// cannot be read at a glance. An unknown one is drawn dim and as a placeholder,
// never as 0% — an unread battery is not a flat one.
func batteryCell(label string) screen.Cell {
	if label == unknownBattery {
		return screen.Cell{Text: "  " + label, Color: screen.Colors.Dim}
	}
	return screen.Cell{Text: "  " + label, Color: screen.Colors.Status}
}
