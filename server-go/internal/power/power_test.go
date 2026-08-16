package power

import (
	"testing"
	"time"
)

func testPolicy() Policy {
	return Policy{DimAfter: 30 * time.Second, OffAfter: 120 * time.Second}
}

var base = time.Now()

func TestLadderClimbsDownWithIdle(t *testing.T) {
	p := testPolicy()
	cases := []struct {
		idle time.Duration
		want State
	}{
		{0, On},
		{29 * time.Second, On},
		{30 * time.Second, Dim},
		{119 * time.Second, Dim},
		{120 * time.Second, Off},
		{10 * time.Minute, Off},
	}
	for _, c := range cases {
		if got := p.stateAt(c.idle); got != c.want {
			t.Fatalf("stateAt(%v) = %q, want %q", c.idle, got, c.want)
		}
	}
}

func TestZeroTimeoutDisablesThatRung(t *testing.T) {
	p := Policy{DimAfter: 0, OffAfter: 60 * time.Second}
	if got := p.stateAt(59 * time.Second); got != On {
		t.Fatalf("with DimAfter=0 the dim rung is skipped, got %q", got)
	}
	if got := p.stateAt(60 * time.Second); got != Off {
		t.Fatalf("off rung still applies, got %q", got)
	}
	never := Policy{}
	if got := never.stateAt(24 * time.Hour); got != On {
		t.Fatalf("an all-zero policy never sleeps, got %q", got)
	}
}

func TestTrackerReportsOnlyTransitions(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)

	if st, changed := tr.At(t0.Add(10 * time.Second)); st != On || changed {
		t.Fatalf("still on = no transition, got %q changed=%v", st, changed)
	}
	if st, changed := tr.At(t0.Add(30 * time.Second)); st != Dim || !changed {
		t.Fatalf("crossing into dim is a transition, got %q changed=%v", st, changed)
	}
	if st, changed := tr.At(t0.Add(40 * time.Second)); st != Dim || changed {
		t.Fatalf("staying dim is not a transition, got %q changed=%v", st, changed)
	}
	if st, changed := tr.At(t0.Add(120 * time.Second)); st != Off || !changed {
		t.Fatalf("crossing into off is a transition, got %q changed=%v", st, changed)
	}
}

func TestWakeRestartsTheLadder(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	tr.At(t0.Add(200 * time.Second))

	st, changed := tr.Wake(t0.Add(200 * time.Second))
	if st != On || !changed {
		t.Fatalf("waking from off turns the screen on, got %q changed=%v", st, changed)
	}
	if st, changed := tr.At(t0.Add(220 * time.Second)); st != On || changed {
		t.Fatalf("idle counts from the wake, got %q changed=%v", st, changed)
	}
	if st, _ := tr.At(t0.Add(230 * time.Second)); st != Dim {
		t.Fatalf("dim lands 30s after the wake, got %q", st)
	}
}

func TestWakeWhileAlreadyOnIsNotATransition(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	if _, changed := tr.Wake(t0.Add(5 * time.Second)); changed {
		t.Fatal("typing while the screen is on must not re-broadcast")
	}
}

func TestInhibitHoldsTheScreenLit(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	tr.At(t0.Add(200 * time.Second))

	st, changed := tr.SetInhibit(t0.Add(200*time.Second), true)
	if st != On || !changed {
		t.Fatalf("a pending question wakes the screen, got %q changed=%v", st, changed)
	}
	if st, changed := tr.At(t0.Add(1 * time.Hour)); st != On || changed {
		t.Fatalf("an unanswered prompt never sleeps, got %q changed=%v", st, changed)
	}
}

func TestClearingInhibitRestartsTheCountdown(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	tr.SetInhibit(t0, true)
	tr.At(t0.Add(1 * time.Hour))

	at := t0.Add(1 * time.Hour)
	if st, changed := tr.SetInhibit(at, false); st != On || changed {
		t.Fatalf("answering a prompt leaves the screen on, got %q changed=%v", st, changed)
	}
	if st, _ := tr.At(at.Add(29 * time.Second)); st != On {
		t.Fatalf("the full dim window starts at the answer, got %q", st)
	}
	if st, _ := tr.At(at.Add(30 * time.Second)); st != Dim {
		t.Fatalf("dim lands 30s after the answer, got %q", st)
	}
}

func TestSleepIsImmediate(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	st, changed := tr.Sleep(t0)
	if st != Off || !changed {
		t.Fatalf("pocketing the device sleeps it at once, got %q changed=%v", st, changed)
	}
	if st, changed := tr.At(t0.Add(1 * time.Hour)); st != Off || changed {
		t.Fatalf("it stays off until woken, got %q changed=%v", st, changed)
	}
}

func TestSleepOverridesAPendingPrompt(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	tr.SetInhibit(t0, true)

	if st, _ := tr.Sleep(t0); st != Off {
		t.Fatalf("a deliberate sleep beats the prompt inhibitor, got %q", st)
	}
	if st, _ := tr.At(t0.Add(1 * time.Minute)); st != Off {
		t.Fatalf("and it stays off while the prompt still waits, got %q", st)
	}
}

func TestWakingClearsAForcedSleep(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	tr.Sleep(t0)

	if st, changed := tr.Wake(t0.Add(1 * time.Minute)); st != On || !changed {
		t.Fatalf("any key wakes it back up, got %q changed=%v", st, changed)
	}
	if st, _ := tr.At(t0.Add(1*time.Minute + 30*time.Second)); st != Dim {
		t.Fatalf("the normal ladder resumes after the wake, got %q", st)
	}
}

func TestForcedSleepArmsNoTimer(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	tr.Sleep(t0)
	if got := tr.Until(t0); got != 0 {
		t.Fatalf("nothing left to schedule once it is off, got %v", got)
	}
}

func TestUntilIsTheNextTransitionDelay(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)

	if got := tr.Until(t0); got != 30*time.Second {
		t.Fatalf("first deadline is the dim one, got %v", got)
	}
	tr.At(t0.Add(30 * time.Second))
	if got := tr.Until(t0.Add(30 * time.Second)); got != 90*time.Second {
		t.Fatalf("next deadline is the off one, got %v", got)
	}
	tr.At(t0.Add(120 * time.Second))
	if got := tr.Until(t0.Add(120 * time.Second)); got != 0 {
		t.Fatalf("the ladder has bottomed out, got %v", got)
	}
}

func TestUntilIsZeroWhileInhibited(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	tr.SetInhibit(t0, true)
	if got := tr.Until(t0); got != 0 {
		t.Fatalf("no timer is armed while a prompt waits, got %v", got)
	}
}

func TestStateReportsTheLastEvaluatedRung(t *testing.T) {
	t0 := time.Unix(0, 0)
	tr := NewTracker(testPolicy(), t0)
	if got := tr.State(); got != On {
		t.Fatalf("a fresh tracker is on, got %q", got)
	}
	tr.At(t0.Add(200 * time.Second))
	if got := tr.State(); got != Off {
		t.Fatalf("after the off deadline the tracker reports off, got %q", got)
	}
}

func TestExternalPowerHoldsTheScreenLit(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.SetExternalPower(base, true)
	if st, _ := tr.At(base.Add(time.Hour)); st != On {
		t.Fatalf("on USB there is no battery to save, got %q", st)
	}
}

func TestExternalPowerArmsNoTimer(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.SetExternalPower(base, true)
	if got := tr.Until(base); got != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestUnpluggingRestartsTheCountdown(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.SetExternalPower(base, true)
	tr.SetExternalPower(base.Add(time.Hour), false)
	if st, _ := tr.At(base.Add(time.Hour).Add(45 * time.Second)); st != Dim {
		t.Fatalf("unplugged, the ladder resumes from the unplug moment, got %q", st)
	}
}

func TestADeliberateSleepStillWorksOnUsb(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.SetExternalPower(base, true)
	if st, _ := tr.Sleep(base); st != Off {
		t.Fatalf("pocketing the device must win over USB, got %q", st)
	}
	if st, _ := tr.At(base.Add(time.Second)); st != Off {
		t.Fatalf("and it must stay off, got %q", st)
	}
}

func TestWakingOnUsbStaysLit(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.SetExternalPower(base, true)
	tr.Sleep(base)
	tr.Wake(base)
	if st, _ := tr.At(base.Add(time.Hour)); st != On {
		t.Fatalf("got %q", st)
	}
}
