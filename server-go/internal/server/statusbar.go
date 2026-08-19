package server

import (
	"fmt"
	"strings"
)

// statusMax is what the device can show without scrolling. Past this the
// firmware marquees the bar, and anything that has to be waited for is not
// something you can read at a glance — which is the whole point of the battery
// being here. Mirrors maxChars in drawStatusBar.
const statusMax = 39

// statusBar is the one line every firmware ever built renders from server
// text. The battery no longer leads it — the device draws its own battery
// and charging state directly now, so this line is free to spend its whole
// budget on title/hint, deliberately truncated when space runs out.
func statusBar(name, title, hint string, row, maxRow, col, size int) string {
	head := "[" + name + "]"
	tail := counters(row, maxRow, col, size)

	middle := strings.Join(nonEmpty(title, hint), "  ")
	budget := max(0, statusMax-len([]rune(head))-len([]rune(tail))-4)
	return strings.Join(nonEmpty(head, clip(middle, budget), tail), "  ")
}

// counters spends characters only on what is not already obvious: a zero column
// and the default zoom say nothing, and the six characters they cost are what
// used to leave no room for the battery.
func counters(row, maxRow, col, size int) string {
	out := fmt.Sprintf("r%d/%d", row, maxRow)
	if col > 0 {
		out += fmt.Sprintf(" c%d", col)
	}
	if size != baseSize {
		out += fmt.Sprintf(" z%d", size)
	}
	return out
}

func nonEmpty(parts ...string) []string {
	out := []string{}
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
