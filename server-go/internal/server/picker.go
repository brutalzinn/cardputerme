package server

import (
	"log"
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

// pickerItem is one row: either a session on THIS machine (switch internally,
// no reconnect) or another machine (re-point the socket). One list rather than
// a mode hierarchy — the device has one rendering path.
type pickerItem struct {
	session string
	machine beacon
	current bool
}

func (s *Server) pickerItems() []pickerItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.itemsLocked()
}

// itemsLocked is the same list for callers that already hold mu.
func (s *Server) itemsLocked() []pickerItem {
	out := []pickerItem{}
	for _, name := range s.order {
		out = append(out, pickerItem{session: name, current: s.sess != nil && s.sess.name == name})
	}
	for _, b := range s.beacons {
		if b.Name == s.cfg.Name {
			continue
		}
		out = append(out, pickerItem{machine: b})
	}
	return out
}

func (s *Server) switchTo(name string) {
	s.mu.Lock()
	sess, ok := s.sessions[name]
	if ok {
		s.sess = sess
		s.picking = false
	}
	s.mu.Unlock()
	if ok {
		log.Printf("[picker] viewing session %q", name)
		s.pushIfChanged(true)
	}
}

func itemLines(items []pickerItem, pick, rows, cols int) []screen.Line {
	if len(items) == 0 {
		return []screen.Line{
			{Text: clip("no terminals yet", cols), Color: screen.Colors.Dim},
			{Text: clip("run cardputerme", cols), Color: screen.Colors.Dim},
		}
	}
	start := pickerStart(pick, len(items), rows)
	out := []screen.Line{}
	for i := start; i < len(items) && i < start+rows; i++ {
		mark := " "
		color := screen.Colors.Text
		if i == pick {
			mark = ">"
			color = screen.Colors.Prompt
		}
		out = append(out, screen.Line{Text: clip(mark+strconv.Itoa(i-start+1)+". "+itemLabel(items[i]), cols), Color: color})
	}
	return out
}

func itemLabel(it pickerItem) string {
	if it.session == "" {
		return "@" + it.machine.Name
	}
	if it.current {
		return it.session + " *"
	}
	return it.session
}

func itemStatus(items []pickerItem, pick int) string {
	if len(items) == 0 {
		return "servers  listening"
	}
	return "pick  " + strconv.Itoa(pick+1) + "/" + strconv.Itoa(len(items))
}

// resolvePick turns a picker action into either a machine jump (a connect
// frame the device acts on) or a local session switch. Caller must not hold mu.
func (s *Server) resolvePick(a input.Action, items []pickerItem) (connect string, switchTo string) {
	idx := -1
	if a.Kind == "connect" {
		idx = a.Index
	}
	if a.Kind == "connectRow" {
		idx = pickerStart(s.pick, len(items), s.rows()) + a.Index
	}
	if idx < 0 || idx >= len(items) {
		return "", ""
	}
	if items[idx].session != "" {
		return "", items[idx].session
	}
	return connectMessage(items[idx].machine), ""
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
