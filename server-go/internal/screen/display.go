package screen

import "strings"

// Colors are the RGB565 values the server assigns to lines; the device draws
// exactly these (no color logic on the device).
var Colors = struct {
	Text, Prompt, Ask, Status, Dim uint16
}{
	Text:   0xFFFF,
	Prompt: 0xFFE0,
	Ask:    0xFD20,
	Status: 0x07FF,
	Dim:    0x8410,
}

// Line is one row of mirrored terminal text with its server-chosen color.
type Line struct {
	Text  string
	Color uint16
}

// Cell is the on-the-wire form of a Line.
type Cell struct {
	Text  string `json:"text"`
	Color uint16 `json:"color"`
}

type DisplayMessage struct {
	Type string `json:"type"`
	Size int    `json:"size"`
	Body []Cell `json:"body"`
	// Badge is a short server-chosen string for the device header — kept
	// generic on purpose, so what it carries can change without a re-flash.
	Badge  string `json:"badge"`
	Status Cell   `json:"status"`
}

func LineColor(text string, awaiting bool) uint16 {
	if awaiting {
		return Colors.Ask
	}
	if strings.HasPrefix(text, "> ") {
		return Colors.Prompt
	}
	return Colors.Text
}

func BuildDisplay(body []Line, statusText string, size int) DisplayMessage {
	return BuildDisplayBadge(body, statusText, "", size)
}

func BuildDisplayBadge(body []Line, statusText, badge string, size int) DisplayMessage {
	cells := make([]Cell, 0, len(body))
	for _, l := range body {
		cells = append(cells, Cell{Text: l.Text, Color: l.Color})
	}
	return DisplayMessage{
		Type:   "display",
		Size:   size,
		Body:   cells,
		Badge:  badge,
		Status: Cell{Text: statusText, Color: Colors.Status},
	}
}
