package server

import (
	"strings"
	"testing"
)

func TestPingAnswersPong(t *testing.T) {
	if got := runCommand(testServer(), ";ping"); got != "Pong!" {
		t.Fatalf("got %q", got)
	}
}

func TestAnUnknownVerbSaysSo(t *testing.T) {
	got := runCommand(testServer(), ";nope")
	if !strings.Contains(got, "nope") {
		t.Fatalf("the reply must name the verb the user typed, got %q", got)
	}
}

func TestArgumentsReachTheCommand(t *testing.T) {
	seen := ""
	commands["echo"] = func(s *Server, args string) string {
		seen = args
		return args
	}
	defer delete(commands, "echo")
	runCommand(testServer(), ";echo hello world")
	if seen != "hello world" {
		t.Fatalf("got %q", seen)
	}
}

func TestAVerbWithNoArgumentsGetsAnEmptyString(t *testing.T) {
	seen := "unset"
	commands["bare"] = func(s *Server, args string) string {
		seen = args
		return ""
	}
	defer delete(commands, "bare")
	runCommand(testServer(), ";bare")
	if seen != "" {
		t.Fatalf("got %q", seen)
	}
}

func TestAddingACommandIsOneMapEntry(t *testing.T) {
	commands["pong"] = func(s *Server, args string) string { return "Ping!" }
	defer delete(commands, "pong")
	if got := runCommand(testServer(), ";pong"); got != "Ping!" {
		t.Fatalf("got %q", got)
	}
}

func TestTypingPingEndToEndShowsPong(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "p", "i", "n", "g", "enter"} {
		s.applyKey(k)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.notice != "Pong!" {
		t.Fatalf("got %q", s.notice)
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

func TestTheReplyReachesTheStatusBar(t *testing.T) {
	s := testServer()
	for _, k := range []string{";", "p", "i", "n", "g", "enter"} {
		s.applyKey(k)
	}
	st := s.composeMirror(nil, "", "", false)
	if !strings.Contains(st.status, "Pong!") {
		t.Fatalf("got %q", st.status)
	}
}

func TestAnEmptyCommandLineSaysNothing(t *testing.T) {
	if got := runCommand(testServer(), ";"); got != "" {
		t.Fatalf("got %q", got)
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
	if s.notice != "" {
		t.Fatalf("a reply must not pin itself to the status bar, got %q", s.notice)
	}
}
