package server

import (
	"cardputerme/internal/battery"
	"cardputerme/internal/screen"
)

// unknownBattery keeps the slot occupied before the device has ever reported.
const unknownBattery = "--%"

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
	cells := []screen.Cell{}
	if s.alert != "" {
		cells = append(cells, screen.Cell{Text: clip(s.alert, alertWidth) + " ", Color: screen.Colors.Ask})
	}
	cells = append(cells, wifiCell(s.wifi))
	return append(cells, batteryCell(battery.Label(s.gauge.Status())))
}

// batteryCell always occupies the slot: a reading that appears and disappears
// cannot be read at a glance. An unknown one is drawn dim and as a placeholder,
// never as 0% — an unread battery is not a flat one.
func batteryCell(label string) screen.Cell {
	if label == "" {
		return screen.Cell{Text: "  " + unknownBattery, Color: screen.Colors.Dim}
	}
	return screen.Cell{Text: "  " + label, Color: screen.Colors.Status}
}
