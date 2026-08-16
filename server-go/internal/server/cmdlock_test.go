package server

import (
	"strings"
	"testing"
	"time"
)

// `;notify 0` froze the whole server: commands.Run was called while holding mu,
// and runNotify calls back into SetNotify, which locks the same non-reentrant
// mutex. This hangs rather than fails if it regresses, hence the timeout.
func TestCommandThatCallsBackDoesNotDeadlock(t *testing.T) {
	s := withSession(New(Config{Name: "n", Session: "n", WrapCols: 20, LinesPerCard: 7, MaxCards: 40, Notify: true}))

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, k := range append(strings.Split(";notify 0", ""), "enter") {
			s.applyKey(k)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a command that calls back into the server deadlocked it")
	}
	if s.NotifyEnabled() {
		t.Fatal("`;notify 0` must turn alerts off")
	}

	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		for _, k := range append(strings.Split(";notify 1", ""), "enter") {
			s.applyKey(k)
		}
	}()
	select {
	case <-done2:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlocked on the way back on")
	}
	if !s.NotifyEnabled() {
		t.Fatal("`;notify 1` must turn alerts on")
	}
}

// The server must stay responsive to everything else after a command runs.
func TestServerStaysResponsiveAfterACommand(t *testing.T) {
	s := withSession(New(Config{Name: "n", Session: "n", WrapCols: 20, LinesPerCard: 7, MaxCards: 40, Notify: true}))
	for _, k := range append(strings.Split(";notify 0", ""), "enter") {
		s.applyKey(k)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.currentName()
		s.sessionNames()
		s.pickerItems()
		s.NotifyEnabled()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the mutex was left held after a command")
	}
}
