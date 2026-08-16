package server

import (
	"strings"
	"time"
)

// level is how much of the user's attention an alert is asking for. One sound
// for everything carries no information, so the user learns to ignore it.
type level int

const (
	attention level = iota
	info
	urgent
)

type note struct {
	freq int
	ms   int
}

// burstGap spaces the notes of a multi-note alert. The device has no tone queue
// and tone() restarts on arrival, so the SERVER paces them.
const burstGap = 180 * time.Millisecond

// levels is the whole vocabulary in one place, so adding a level is one row
// rather than four edits nothing would check.
//
// The LED carries urgency by COLOUR because only three behaviours exist on the
// device — off, pulse, and solid (anything else) — and the pulse PERIOD is a
// compile-time constant there.
//
// Exactly one level may use the wav: the device caches ONE by URL and
// re-downloads it over blocking HTTP whenever the URL changes, so a second
// would thrash the cache and stall rendering mid-frame.
var levels = [...]struct {
	name  string
	led   led
	tones []note
	wav   bool
}{
	attention: {"attention", ledAttention, []note{{1200, 90}}, true},
	info:      {"info", ledInfo, []note{{880, 60}}, false},
	urgent:    {"urgent", ledUrgent, []note{{1600, 70}, {1900, 70}, {2300, 120}}, false},
}

// levelOf falls back to attention, which is why it is the zero value: an absent
// or misspelled level must behave exactly as callers did before levels existed.
func levelOf(name string) level {
	want := strings.ToLower(strings.TrimSpace(name))
	for i, l := range levels {
		if l.name == want {
			return level(i)
		}
	}
	return attention
}

func (l level) led() led      { return levels[l].led }
func (l level) tones() []note { return levels[l].tones }
func (l level) usesWav() bool { return levels[l].wav }
