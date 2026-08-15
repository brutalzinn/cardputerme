package main

import "strings"

var colors = struct {
	Text, Prompt, Ask, Status, Dim uint16
}{
	Text:   0xFFFF,
	Prompt: 0xFFE0,
	Ask:    0xFD20,
	Status: 0x07FF,
	Dim:    0x8410,
}

func lineColor(text string, awaiting bool) uint16 {
	if awaiting {
		return colors.Ask
	}
	if strings.HasPrefix(text, "> ") {
		return colors.Prompt
	}
	return colors.Text
}

type Cell struct {
	Text  string `json:"text"`
	Color uint16 `json:"color"`
}

type DisplayMessage struct {
	Type   string `json:"type"`
	Body   []Cell `json:"body"`
	Status Cell   `json:"status"`
}

func buildDisplay(body []Line, statusText string) DisplayMessage {
	cells := []Cell{}
	for _, l := range body {
		cells = append(cells, Cell{Text: l.Text, Color: l.Color})
	}
	return DisplayMessage{
		Type:   "display",
		Body:   cells,
		Status: Cell{Text: statusText, Color: colors.Status},
	}
}
