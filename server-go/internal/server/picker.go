package server

import (
	"strconv"

	"cardputerme/internal/input"
	"cardputerme/internal/screen"
)

type beacon struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

func connectMessage(b beacon) string {
	return `{"type":"connect","ip":` + strconv.Quote(b.IP) + `,"port":` + strconv.Itoa(b.Port) + `}`
}

func (s *Server) resolveConnect(a input.Action) string {
	idx := -1
	if a.Kind == "connect" {
		idx = a.Index
	}
	if a.Kind == "connectRow" {
		idx = pickerStart(s.pick, len(s.beacons), s.rows()) + a.Index
	}
	if idx < 0 || idx >= len(s.beacons) {
		return ""
	}
	return connectMessage(s.beacons[idx])
}

func (s *Server) handleBeacons(list []beacon) {
	s.mu.Lock()
	changed := len(list) != len(s.beacons)
	for i := range list {
		if !changed && list[i] != s.beacons[i] {
			changed = true
		}
	}
	s.beacons = list
	picking := s.picking
	s.mu.Unlock()
	if changed && picking {
		s.schedulePush()
	}
}

func pickerStart(pick, n, rows int) int {
	if rows < 1 || n <= rows {
		return 0
	}
	start := pick - rows/2
	if start < 0 {
		return 0
	}
	if start > n-rows {
		return n - rows
	}
	return start
}

func clip(text string, cols int) string {
	r := []rune(text)
	if len(r) <= cols {
		return text
	}
	return string(r[:cols])
}

func pickerLines(bs []beacon, pick, rows, cols int) []screen.Line {
	if len(bs) == 0 {
		return []screen.Line{
			{Text: clip("no terminals yet", cols), Color: screen.Colors.Dim},
			{Text: clip("run cardputerme", cols), Color: screen.Colors.Dim},
		}
	}
	start := pickerStart(pick, len(bs), rows)
	out := []screen.Line{}
	for i := start; i < len(bs) && i < start+rows; i++ {
		mark := " "
		color := screen.Colors.Text
		if i == pick {
			mark = ">"
			color = screen.Colors.Prompt
		}
		row := mark + strconv.Itoa(i-start+1) + ". " + bs[i].Name
		out = append(out, screen.Line{Text: clip(row, cols), Color: color})
	}
	return out
}

func pickerStatus(bs []beacon, pick int) string {
	if len(bs) == 0 {
		return "servers  listening"
	}
	return "servers  " + strconv.Itoa(pick+1) + "/" + strconv.Itoa(len(bs))
}
