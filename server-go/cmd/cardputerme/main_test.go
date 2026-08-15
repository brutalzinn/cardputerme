package main

import "testing"

func noEnv(string) string { return "" }

func TestArgumentNamesTheExposure(t *testing.T) {
	if got := sessionName([]string{"myproject"}, noEnv, "/home/rob/work"); got != "myproject" {
		t.Fatalf("got %q", got)
	}
}

func TestArgumentBeatsTheEnvironment(t *testing.T) {
	env := func(k string) string { return "from-env" }
	if got := sessionName([]string{"from-arg"}, env, "/home/rob/work"); got != "from-arg" {
		t.Fatalf("got %q", got)
	}
}

func TestEnvironmentNamesItWhenNoArgument(t *testing.T) {
	env := func(k string) string {
		if k == "NAME" {
			return "from-env"
		}
		return ""
	}
	if got := sessionName(nil, env, "/home/rob/work"); got != "from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestTheTmuxSessionNameNeverBecomesTheLabel(t *testing.T) {
	env := func(k string) string {
		if k == "SESSION" {
			return "claude-3"
		}
		return ""
	}
	if got := sessionName(nil, env, "/home/rob/work/myproject"); got != "myproject" {
		t.Fatalf("the label is the project dir, not the tmux session, got %q", got)
	}
}

func TestBareInvocationTakesTheDirectoryName(t *testing.T) {
	if got := sessionName(nil, noEnv, "/home/rob/work/cardputerme"); got != "cardputerme" {
		t.Fatalf("got %q", got)
	}
}

func TestEmptyArgumentIsIgnored(t *testing.T) {
	if got := sessionName([]string{""}, noEnv, "/home/rob/work"); got != "work" {
		t.Fatalf("got %q", got)
	}
}

func TestUnknownDirectoryStillHasAName(t *testing.T) {
	if got := sessionName(nil, noEnv, ""); got != "cardputerme" {
		t.Fatalf("got %q", got)
	}
}
