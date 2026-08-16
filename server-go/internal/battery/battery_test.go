package battery

import (
	"testing"
	"time"
)

var base = time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)

func testPolicy() Policy {
	return Policy{
		MinMv:       3300,
		MaxMv:       4100,
		Window:      8,
		Deadband:    3,
		SettleAfter: 2 * time.Second,
		RiseRate:    0.5,
		RisePeriod:  30 * time.Second,
	}
}

func TestPercentMapsTheEffectiveRange(t *testing.T) {
	p := testPolicy()
	cases := []struct {
		mv   int
		want int
	}{
		{3300, 0},
		{3700, 50},
		{4100, 100},
		{3000, 0},
		{4200, 100},
	}
	for _, c := range cases {
		if got := p.percent(c.mv); got != c.want {
			t.Fatalf("percent(%d) = %d, want %d", c.mv, got, c.want)
		}
	}
}

func TestUnknownUntilTheFirstReading(t *testing.T) {
	g := NewGauge(testPolicy())
	if _, known := g.Percent(); known {
		t.Fatal("a gauge with no reading must report unknown, never 0%")
	}
	g.Observe(Reading{Millivolts: 3700, At: base})
	pct, known := g.Percent()
	if !known {
		t.Fatal("after a reading the gauge must be known")
	}
	if pct != 50 {
		t.Fatalf("first reading should show through unsmoothed, got %d", pct)
	}
}

func TestMedianRejectsASpike(t *testing.T) {
	g := NewGauge(testPolicy())
	at := base
	for _, mv := range []int{3700, 3700, 3700, 3700, 3700} {
		g.Observe(Reading{Millivolts: mv, At: at})
		at = at.Add(time.Second)
	}
	g.Observe(Reading{Millivolts: 3300, At: at})
	if pct, _ := g.Percent(); pct != 50 {
		t.Fatalf("one 400mV spike must not move the median, got %d", pct)
	}
}

func TestDeadbandHoldsASmallWobble(t *testing.T) {
	g := NewGauge(testPolicy())
	g.Observe(Reading{Millivolts: 3700, At: base})
	before, _ := g.Percent()
	at := base.Add(time.Second)
	for range 8 {
		g.Observe(Reading{Millivolts: 3684, At: at})
		at = at.Add(time.Second)
	}
	after, _ := g.Percent()
	if before != after {
		t.Fatalf("a 2-point drop is inside the deadband; moved %d -> %d", before, after)
	}
}

func TestDischargingNeverRises(t *testing.T) {
	g := NewGauge(testPolicy())
	g.Observe(Reading{Millivolts: 3700, At: base})
	at := base.Add(time.Second)
	for range 8 {
		g.Observe(Reading{Millivolts: 3900, At: at})
		at = at.Add(time.Second)
	}
	if pct, _ := g.Percent(); pct != 50 {
		t.Fatalf("on battery the shown percent must never climb, got %d", pct)
	}
}

func TestExternalPowerAllowsARise(t *testing.T) {
	g := NewGauge(testPolicy())
	g.Observe(Reading{Millivolts: 3700, At: base})
	at := base.Add(time.Second)
	for range 8 {
		g.Observe(Reading{Millivolts: 3900, External: true, At: at})
		at = at.Add(time.Second)
	}
	if pct, _ := g.Percent(); pct != 75 {
		t.Fatalf("while charging the percent may climb, got %d", pct)
	}
}

func TestReadingsInsideTheWakeWindowAreIgnored(t *testing.T) {
	g := NewGauge(testPolicy())
	g.Observe(Reading{Millivolts: 3700, At: base})
	g.Disturb(base.Add(10 * time.Second))
	g.Observe(Reading{Millivolts: 3300, At: base.Add(10*time.Second + 500*time.Millisecond)})
	if pct, _ := g.Percent(); pct != 50 {
		t.Fatalf("a sag right after a power transition must be discarded, got %d", pct)
	}
}

func TestReadingsAfterTheWakeWindowCount(t *testing.T) {
	g := NewGauge(testPolicy())
	g.Observe(Reading{Millivolts: 3700, At: base})
	g.Disturb(base.Add(10 * time.Second))
	at := base.Add(20 * time.Second)
	for range 8 {
		g.Observe(Reading{Millivolts: 3500, At: at})
		at = at.Add(time.Second)
	}
	if pct, _ := g.Percent(); pct != 25 {
		t.Fatalf("once settled the reading must land, got %d", pct)
	}
}

func TestSustainedRiseCountsAsCharging(t *testing.T) {
	g := NewGauge(testPolicy())
	if g.Charging() {
		t.Fatal("a fresh gauge is not charging")
	}
	at := base
	for _, mv := range []int{3700, 3720, 3740, 3760} {
		g.Observe(Reading{Millivolts: mv, At: at})
		at = at.Add(20 * time.Second)
	}
	if !g.Charging() {
		t.Fatal("a sustained rise over the rise period means charging")
	}
}

func TestSlowDriftIsNotCharging(t *testing.T) {
	g := NewGauge(testPolicy())
	at := base
	for _, mv := range []int{3700, 3702, 3704, 3706} {
		g.Observe(Reading{Millivolts: mv, At: at})
		at = at.Add(60 * time.Second)
	}
	if g.Charging() {
		t.Fatal("drift below the rate threshold is not charging")
	}
}

func TestWiredExternalIsCharging(t *testing.T) {
	g := NewGauge(testPolicy())
	g.Observe(Reading{Millivolts: 3700, External: true, At: base})
	if !g.Charging() {
		t.Fatal("an explicitly external reading is charging")
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		pct      int
		known    bool
		charging bool
		want     string
	}{
		{0, false, false, ""},
		{87, true, false, "87%"},
		{87, true, true, "+87%"},
		{0, true, false, "0%"},
		{100, true, true, "+100%"},
	}
	for _, c := range cases {
		if got := Label(c.pct, c.known, c.charging); got != c.want {
			t.Fatalf("Label(%d,%v,%v) = %q, want %q", c.pct, c.known, c.charging, got, c.want)
		}
	}
}
