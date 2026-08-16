package server

import (
	"cardputerme/internal/battery"
	"cardputerme/internal/screen"
)

func (s *Server) setWifi(up bool) {
	s.mu.Lock()
	s.wifi = up
	s.mu.Unlock()
}

// applyWifi takes the fact the device reported. Absent means the firmware
// predates the field, not that the radio is down.
func (s *Server) applyWifi(up *bool) {
	if up == nil {
		return
	}
	s.setWifi(*up)
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
	cells := []screen.Cell{{Text: "WiFi", Color: screen.Colors.Status}}
	if !s.wifi {
		cells = []screen.Cell{{Text: "NoWiFi", Color: screen.Colors.Ask}}
	}
	if bat := battery.Label(s.gauge.Status()); bat != "" {
		cells = append(cells, screen.Cell{Text: "  " + bat, Color: screen.Colors.Status})
	}
	return cells
}
