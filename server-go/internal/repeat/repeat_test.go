package repeat

import (
	"testing"
	"time"
)

var policy = Policy{Delay: 350 * time.Millisecond, Interval: 90 * time.Millisecond, MaxHold: 10 * time.Second}

func TestNothingHeldRepeatsNothing(t *testing.T) {
	h := NewHolder(policy)
	if _, ok := h.Next(time.Now()); ok {
		t.Fatal("an idle holder must not schedule a repeat")
	}
}

func TestFirstRepeatWaitsForTheDelay(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("fn+up", now)
	d, ok := h.Next(now)
	if !ok || d != policy.Delay {
		t.Fatalf("got %v ok=%v", d, ok)
	}
}

func TestLaterRepeatsUseTheInterval(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("fn+up", now)
	h.Fired(now)
	d, ok := h.Next(now.Add(policy.Delay))
	if !ok || d != policy.Interval {
		t.Fatalf("got %v ok=%v", d, ok)
	}
}

func TestReleasingStopsTheRepeat(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("fn+up", now)
	h.Up("fn+up")
	if _, ok := h.Next(now); ok {
		t.Fatal("a released key must not repeat")
	}
}

func TestReleasingAnotherKeyDoesNotStopTheHeldOne(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("fn+up", now)
	h.Up("fn+down")
	if _, ok := h.Next(now); !ok {
		t.Fatal("an unrelated release must not cancel the held key")
	}
}

func TestANewKeyTakesOverFromTheOldOne(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("fn+up", now)
	h.Fired(now)
	h.Down("fn+down", now)
	if got := h.Key(); got != "fn+down" {
		t.Fatalf("got %q", got)
	}
	d, ok := h.Next(now)
	if !ok || d != policy.Delay {
		t.Fatalf("a fresh key restarts at the delay, got %v ok=%v", d, ok)
	}
}

func TestALostReleaseCannotRepeatForever(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("fn+up", now)
	if _, ok := h.Next(now.Add(policy.MaxHold)); ok {
		t.Fatal("a key held past MaxHold must stop — the release packet was lost")
	}
}

func TestKeyIsEmptyOnceReleased(t *testing.T) {
	h := NewHolder(policy)
	h.Down("fn+up", time.Now())
	h.Up("fn+up")
	if got := h.Key(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestZeroIntervalDisablesRepeating(t *testing.T) {
	h := NewHolder(Policy{Delay: 350 * time.Millisecond})
	h.Down("fn+up", time.Now())
	if _, ok := h.Next(time.Now()); ok {
		t.Fatal("no interval configured means no auto-repeat")
	}
}

func TestHoldingTheSameKeyDoesNotRestartTheDelay(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("up", now)
	h.Fired(now)
	h.Down("up", now)
	d, ok := h.Next(now)
	if !ok || d != policy.Interval {
		t.Fatalf("a repeated down for a held key must not fall back to the delay, got %v ok=%v", d, ok)
	}
}

func TestHoldingTheSameKeyDoesNotExtendMaxHold(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("up", now)
	h.Down("up", now.Add(9*time.Second))
	if _, ok := h.Next(now.Add(policy.MaxHold)); ok {
		t.Fatal("re-reporting a held key must not push the safety cap out forever")
	}
}

func TestOneIntervalIsOnePress(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("up", now)
	h.Fired(now)
	if got := h.Due(now.Add(policy.Interval)); got != 1 {
		t.Fatalf("got %d", got)
	}
}

func TestALateTimerCatchesUpInOneGo(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("up", now)
	h.Fired(now)
	if got := h.Due(now.Add(3 * policy.Interval)); got != 3 {
		t.Fatalf("a stalled timer should catch up, got %d", got)
	}
}

func TestAnEarlyTickStillPressesOnce(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("up", now)
	h.Fired(now)
	if got := h.Due(now); got != 1 {
		t.Fatalf("got %d", got)
	}
}

func TestALongStallCannotAvalanche(t *testing.T) {
	h := NewHolder(policy)
	now := time.Now()
	h.Down("up", now)
	h.Fired(now)
	if got := h.Due(now.Add(time.Hour)); got > maxCatchUp {
		t.Fatalf("a suspended process must not fire an avalanche of presses, got %d", got)
	}
}
