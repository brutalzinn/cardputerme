package idle

import (
	"testing"
	"time"
)

var base = time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)

func testPolicy() Policy { return Policy{After: 12 * time.Hour} }

func TestZeroMeansNeverExit(t *testing.T) {
	tr := NewTracker(Policy{After: 0}, base)
	if tr.Expired(base.Add(30 * 24 * time.Hour)) {
		t.Fatal("After=0 must mean never exit, matching DIM_AFTER_S/OFF_AFTER_S")
	}
	if tr.Until(base) != 0 {
		t.Fatal("a disabled tracker arms nothing")
	}
}

func TestNotExpiredInsideTheWindow(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	if tr.Expired(base.Add(11 * time.Hour)) {
		t.Fatal("still inside the window")
	}
	if got := tr.Until(base.Add(11 * time.Hour)); got != time.Hour {
		t.Fatalf("Until = %v, want 1h", got)
	}
}

func TestExpiresAfterTheWindow(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	if !tr.Expired(base.Add(12 * time.Hour)) {
		t.Fatal("no contact for the whole window means exit")
	}
}

func TestLiveClientSuppressesExit(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Connect(base)
	if tr.Expired(base.Add(30 * 24 * time.Hour)) {
		t.Fatal("an open connection means the exposure is in use, however quiet")
	}
	if tr.Until(base.Add(30*24*time.Hour)) != 0 {
		t.Fatal("nothing to arm while a client is connected")
	}
}

func TestDisconnectRestartsTheClock(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Connect(base)
	left := base.Add(20 * time.Hour)
	tr.Disconnect(left)
	if tr.Expired(left.Add(11 * time.Hour)) {
		t.Fatal("the window restarts when the last client leaves")
	}
	if !tr.Expired(left.Add(12 * time.Hour)) {
		t.Fatal("and expires a full window after that")
	}
}

func TestSecondClientKeepsItAlive(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Connect(base)
	tr.Connect(base)
	tr.Disconnect(base.Add(time.Hour))
	if tr.Expired(base.Add(30 * 24 * time.Hour)) {
		t.Fatal("one client left but another is still connected")
	}
}

func TestContactResetsTheWindow(t *testing.T) {
	tr := NewTracker(testPolicy(), base)
	tr.Contact(base.Add(11 * time.Hour))
	if tr.Expired(base.Add(22 * time.Hour)) {
		t.Fatal("contact restarts the countdown")
	}
	if !tr.Expired(base.Add(23 * time.Hour)) {
		t.Fatal("expires a full window after the last contact")
	}
}
