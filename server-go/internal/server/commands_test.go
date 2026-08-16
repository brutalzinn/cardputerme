package server

import (
	"strings"
	"testing"
)

func TestTypingPingEndToEndShowsPong(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "p", "i", "n", "g", "enter"} {
		s.applyKey(k)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reply != "Pong!" {
		t.Fatalf("got %q", s.reply)
	}
	if s.input != "" {
		t.Fatalf("a command must never land in the terminal buffer, got %q", s.input)
	}
}

func TestTheCommandLineIsVisibleWhileTyping(t *testing.T) {
	s := testServer()
	s.applyKey(";")
	s.applyKey("p")
	st := s.composeMirror(nil, "", "", false)
	found := false
	for _, l := range st.lines {
		if strings.Contains(l.Text, ";p") {
			found = true
		}
	}
	if !found {
		t.Fatal("the user must see what they are typing")
	}
}

func TestTheReplyIsShownLikeOutput(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "p", "i", "n", "g", "enter"} {
		s.applyKey(k)
	}
	st := s.composeMirror(nil, "", "", false)
	found := false
	for _, l := range st.lines {
		if strings.Contains(l.Text, "Pong!") {
			found = true
		}
	}
	if !found {
		t.Fatal("a command reply is a response, so it belongs on screen with the output")
	}
}

func TestTheReplyStaysOutOfTheStatusBar(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "p", "i", "n", "g", "enter"} {
		s.applyKey(k)
	}
	if st := s.composeMirror(nil, "", "", false); strings.Contains(st.status, "Pong!") {
		t.Fatalf("the bar is for state, not replies, got %q", st.status)
	}
}

func TestACommandNeverReachesTheTerminal(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "p", "i", "n", "g", "enter"} {
		if a := s.applyKey(k); a.Kind == "send" || a.Kind == "pressKey" {
			t.Fatalf("key %q produced %q, which would be typed into the terminal", k, a.Kind)
		}
	}
}

func TestTheNoticeClearsOnTheNextKey(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "p", "i", "n", "g", "enter"} {
		s.applyKey(k)
	}
	s.applyKey("x")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reply != "" {
		t.Fatalf("a reply must not pin itself to the status bar, got %q", s.reply)
	}
}

func TestHelpRendersOneCommandPerRow(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "h", "e", "l", "p", "enter"} {
		s.applyKey(k)
	}
	st := s.composeMirror(nil, "", "", false)
	for _, l := range st.lines {
		if strings.Count(l.Text, ";") > 1 {
			t.Fatalf("two commands crammed into one row: %q", l.Text)
		}
	}
}
