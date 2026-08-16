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

func levelOf(name string) level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "info":
		return info
	case "urgent":
		return urgent
	}
	return attention
}

// led picks a colour and pattern the deployed firmware can already draw. Only
// three behaviours exist on the device — off, pulse, and solid (anything else)
// — and the pulse PERIOD is a compile-time constant, so urgency has to be
// carried by colour rather than by blink rate.
func (l level) led() led {
	if l == info {
		return led{R: 0, G: 80, B: 255, Pattern: "solid"}
	}
	if l == urgent {
		return led{R: 255, G: 0, B: 0, Pattern: "solid"}
	}
	return ledAttention
}

// usesWav is true for exactly one level. The device caches ONE wav by URL and
// re-downloads over blocking HTTP whenever the URL changes, so a second wav
// would thrash the cache and stall rendering. Distinction is by tone instead.
func (l level) usesWav() bool { return l == attention }

func (l level) tones() []note {
	if l == info {
		return []note{{freq: 880, ms: 60}}
	}
	if l == urgent {
		return []note{{freq: 1600, ms: 70}, {freq: 1900, ms: 70}, {freq: 2300, ms: 120}}
	}
	return []note{{freq: 1200, ms: 90}}
}
