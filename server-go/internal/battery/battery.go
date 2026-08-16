package battery

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

type Reading struct {
	Millivolts int
	External   bool
	At         time.Time
}

type Policy struct {
	MinMv       int
	MaxMv       int
	Window      int
	Deadband    int
	SettleAfter time.Duration
	RiseRate    float64
	RisePeriod  time.Duration
}

func (p Policy) percent(mv int) int {
	span := p.MaxMv - p.MinMv
	if span <= 0 {
		return 0
	}
	pct := (mv - p.MinMv) * 100 / span
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

type sample struct {
	mv int
	at time.Time
}

type Gauge struct {
	policy Policy

	mu          sync.Mutex
	samples     []sample
	shown       int
	known       bool
	wired       bool
	rising      bool
	lastDisturb time.Time
}

func NewGauge(p Policy) *Gauge {
	return &Gauge{policy: p}
}

func (g *Gauge) Disturb(now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastDisturb = now
}

func (g *Gauge) Observe(r Reading) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.wired = r.External
	if g.settling(r.At) {
		return
	}
	g.samples = append(g.samples, sample{mv: r.Millivolts, at: r.At})
	if len(g.samples) > g.policy.Window {
		g.samples = g.samples[len(g.samples)-g.policy.Window:]
	}
	g.rising = g.trendRising()
	g.settle(g.policy.percent(median(g.samples)))
}

func (g *Gauge) Percent() (int, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.shown, g.known
}

func (g *Gauge) Charging() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.charging()
}

func (g *Gauge) Status() (int, bool, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.shown, g.known, g.charging()
}

func (g *Gauge) settling(now time.Time) bool {
	if g.lastDisturb.IsZero() {
		return false
	}
	return now.Sub(g.lastDisturb) < g.policy.SettleAfter
}

func (g *Gauge) settle(pct int) {
	if !g.known {
		g.known = true
		g.shown = pct
		return
	}
	if pct > g.shown && !g.charging() {
		return
	}
	if abs(pct-g.shown) < g.policy.Deadband {
		return
	}
	g.shown = pct
}

func (g *Gauge) charging() bool {
	return g.wired || g.rising
}

func (g *Gauge) trendRising() bool {
	if g.policy.RisePeriod <= 0 || g.policy.RiseRate <= 0 {
		return false
	}
	n := len(g.samples)
	if n < 2 {
		return false
	}
	last := g.samples[n-1]
	for i := 0; i < n-1; i++ {
		span := last.at.Sub(g.samples[i].at)
		if span < g.policy.RisePeriod {
			continue
		}
		if float64(last.mv-g.samples[i].mv)/span.Seconds() >= g.policy.RiseRate {
			return true
		}
	}
	return false
}

func Label(pct int, known, charging bool) string {
	if !known {
		return ""
	}
	if charging {
		return "+" + strconv.Itoa(pct) + "%"
	}
	return strconv.Itoa(pct) + "%"
}

func median(s []sample) int {
	mvs := make([]int, len(s))
	for i, v := range s {
		mvs[i] = v.mv
	}
	sort.Ints(mvs)
	return mvs[len(mvs)/2]
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
