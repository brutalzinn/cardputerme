package quiet

import (
	"testing"
	"time"
)

var base = time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)

func testPolicy() Policy { return Policy{After: 60 * time.Second} }

func TestZeroPolicyNeverDue(t *testing.T) {
	tr := NewTracker(Policy{After: 0}, base)
	tr.Contact(base, "x")
	if tr.Due(base.Add(365 * 24 * time.Hour)) {
		t.Fatal("After=0 must mean never due, matching DimAfter/OffAfter/IdleExit")
	}
	if tr.Until(base.Add(365 * 24 * time.Hour)) != 0 {
		t.Fatal("a disabled tracker arms nothing")
	}
}

func TestNotDueInsideTheWindow(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base, "x")
	if tr.Due(base.Add(59 * time.Second)) {
		t.Fatal("still inside the window")
	}
	if got := tr.Until(base.Add(59 * time.Second)); got != time.Second {
		t.Fatalf("Until = %v, want 1s", got)
	}
}

func TestDueOnceTheWindowElapses(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base, "x")
	if !tr.Due(base.Add(60 * time.Second)) {
		t.Fatal("unchanged content for the whole window means due")
	}
}

func TestUntilIsZeroOnceDue(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base, "x")
	if got := tr.Until(base.Add(60 * time.Second)); got != 0 {
		t.Fatalf("Until = %v, want 0 once due", got)
	}
}

func TestUnchangedContentDoesNotResetTheClock(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base, "x")
	// Simulates a re-check triggered only by the spinner ticking the status
	// row — the grid content itself did not change, so the clock must not
	// be pushed out to base+30s+60s.
	tr.Contact(base.Add(30*time.Second), "x")
	if !tr.Due(base.Add(60 * time.Second)) {
		t.Fatal("a same-content Contact must not reset the deadline")
	}
}

func TestChangedContentResetsTheClock(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base, "x")
	tr.Contact(base.Add(10*time.Second), "y")
	if tr.Due(base.Add(60 * time.Second)) {
		t.Fatal("only 50s have passed since the reset")
	}
	if !tr.Due(base.Add(70 * time.Second)) {
		t.Fatal("60s since the reset means due")
	}
}

func TestConsumeSuppressesFurtherDueUntilNewContact(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base, "x")
	tr.Consume(base.Add(60 * time.Second))
	if tr.Due(base.Add(60 * time.Second)) {
		t.Fatal("Consume must clear Due for the current quiet period")
	}
	if tr.Due(base.Add(1000 * time.Second)) {
		t.Fatal("without new contact, a consumed period must stay answered forever")
	}
}

func TestNewContentAfterConsumeReArmsFiring(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base, "x")
	tr.Consume(base.Add(60 * time.Second))
	tr.Contact(base.Add(100*time.Second), "y")
	if tr.Due(base.Add(150 * time.Second)) {
		t.Fatal("only 50s have passed since the new contact")
	}
	if !tr.Due(base.Add(160 * time.Second)) {
		t.Fatal("new content after Consume must re-arm firing")
	}
}
