package input

import "testing"

func TestSemicolonOnAnEmptyBufferOpensCommandMode(t *testing.T) {
	r := InterpretKey(State{Hist: -1}, ";", KeyCtx{})
	if r.State.Cmd != ";" || r.State.Input != "" {
		t.Fatalf("got %+v", r.State)
	}
}

func TestColonWhileTypingStaysALiteralColon(t *testing.T) {
	r := InterpretKey(mir("git commit -m 'x"), ";", KeyCtx{})
	if r.State.Cmd != "" || r.State.Input != "git commit -m 'x;" {
		t.Fatalf("a colon mid-sentence must reach the terminal, got %+v", r.State)
	}
}

func TestCharactersBuildTheCommandLine(t *testing.T) {
	s := State{Hist: -1}
	for _, k := range []string{";", "p", "i", "n", "g"} {
		s = InterpretKey(s, k, KeyCtx{}).State
	}
	if s.Cmd != ";ping" || s.Input != "" {
		t.Fatalf("got %+v", s)
	}
}

func TestBackspaceDeletesInTheCommandLine(t *testing.T) {
	s := State{Cmd: ";pin", Hist: -1}
	if got := InterpretKey(s, "backspace", KeyCtx{}).State.Cmd; got != ";pi" {
		t.Fatalf("got %q", got)
	}
}

func TestBackspacingTheColonLeavesCommandMode(t *testing.T) {
	if got := InterpretKey(State{Cmd: ";", Hist: -1}, "backspace", KeyCtx{}).State.Cmd; got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestEscCancelsTheCommand(t *testing.T) {
	r := InterpretKey(State{Cmd: ";ping", Hist: -1}, "esc", KeyCtx{})
	if r.State.Cmd != "" || r.Action.Kind != "none" {
		t.Fatalf("got %+v", r)
	}
}

func TestEnterRunsTheCommand(t *testing.T) {
	r := InterpretKey(State{Cmd: ";ping", Hist: -1}, "enter", KeyCtx{})
	if r.Action.Kind != "command" || r.Action.Text != ";ping" {
		t.Fatalf("got %+v", r.Action)
	}
	if r.State.Cmd != "" {
		t.Fatalf("the line must close after running, got %q", r.State.Cmd)
	}
}

func TestAnEmptyCommandClosesTheLine(t *testing.T) {
	r := InterpretKey(State{Cmd: ";", Hist: -1}, "enter", KeyCtx{})
	if r.State.Cmd != "" {
		t.Fatalf("got %+v", r)
	}
}

func TestACommandNeverRepeats(t *testing.T) {
	if InterpretKey(State{Cmd: ";ping", Hist: -1}, "enter", KeyCtx{}).Action.Repeat {
		t.Fatal("holding enter must not run the command twice")
	}
}

func TestCommandModeSwallowsOtherKeys(t *testing.T) {
	for _, k := range []string{"tab", "up", "ctrl+=", "opt+up", "ctrl+up"} {
		r := InterpretKey(State{Cmd: ";pi", Hist: -1}, k, KeyCtx{})
		if r.Action.Kind != "none" || r.State.Cmd != ";pi" {
			t.Fatalf("%q should be inert while typing a command, got %+v", k, r)
		}
	}
}

func TestCommandModeNeverTouchesTheTerminalBuffer(t *testing.T) {
	s := State{Cmd: ";", Input: "", Hist: -1}
	for _, k := range []string{"p", "i", "n", "g"} {
		s = InterpretKey(s, k, KeyCtx{}).State
	}
	if s.Input != "" {
		t.Fatalf("got %q", s.Input)
	}
}

func TestADigitInCommandModeIsNotAMenuAnswer(t *testing.T) {
	r := InterpretKey(State{Cmd: ";", Hist: -1}, "2", KeyCtx{Awaiting: true})
	if r.Action.Kind != "none" || r.State.Cmd != ";2" {
		t.Fatalf("got %+v", r)
	}
}
