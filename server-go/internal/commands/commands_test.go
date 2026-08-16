package commands

import (
	"strings"
	"testing"
)

func ctx() Ctx { return Ctx{Name: "test"} }

func TestPingAnswersPong(t *testing.T) {
	if got := Run(ctx(), "ping"); got != "Pong!" {
		t.Fatalf("got %q", got)
	}
}

func TestAnUnknownVerbSaysSo(t *testing.T) {
	if got := Run(ctx(), "nope"); !strings.Contains(got, "nope") {
		t.Fatalf("the reply must name the verb the user typed, got %q", got)
	}
}

func TestAnEmptyCommandLineSaysNothing(t *testing.T) {
	if got := Run(ctx(), ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestArgumentsReachTheCommand(t *testing.T) {
	seen := ""
	register("echo", Command{Help: "test only", Run: func(c Ctx, args string) string { seen = args; return args }})
	defer delete(registry, "echo")
	Run(ctx(), "echo hello world")
	if seen != "hello world" {
		t.Fatalf("got %q", seen)
	}
}

func TestAVerbWithNoArgumentsGetsAnEmptyString(t *testing.T) {
	seen := "unset"
	register("bare", Command{Help: "test only", Run: func(c Ctx, args string) string { seen = args; return "" }})
	defer delete(registry, "bare")
	Run(ctx(), "bare")
	if seen != "" {
		t.Fatalf("got %q", seen)
	}
}

func TestTheExposureNameReachesTheCommand(t *testing.T) {
	seen := ""
	register("who", Command{Help: "test only", Run: func(c Ctx, args string) string { seen = c.Name; return "" }})
	defer delete(registry, "who")
	Run(ctx(), "who")
	if seen != "test" {
		t.Fatalf("a command must be able to see the server's state, got %q", seen)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	got := Run(ctx(), "help")
	for verb := range registry {
		if !strings.Contains(got, verb) {
			t.Fatalf("%q is registered but missing from help: %q", verb, got)
		}
	}
}

func TestANewCommandAppearsInHelpOnItsOwn(t *testing.T) {
	register("weather", Command{Help: "test only", Run: func(c Ctx, args string) string { return "" }})
	defer delete(registry, "weather")
	if got := Run(ctx(), "help"); !strings.Contains(got, "weather") {
		t.Fatalf("registering is the only step; got %q", got)
	}
}

func TestHelpShowsWhatEachCommandDoes(t *testing.T) {
	if got := Run(ctx(), "help"); !strings.Contains(got, registry["ping"].Help) {
		t.Fatalf("got %q", got)
	}
}

func TestHelpIsSortedSoItIsStable(t *testing.T) {
	register("aaa", Command{Help: "test only", Run: func(c Ctx, args string) string { return "" }})
	defer delete(registry, "aaa")
	got := Run(ctx(), "help")
	if strings.Index(got, "aaa") > strings.Index(got, "ping") {
		t.Fatalf("map order must not leak onto the screen, got %q", got)
	}
}

func TestHelpPutsEachCommandOnItsOwnLine(t *testing.T) {
	got := Run(ctx(), "help")
	if !strings.Contains(got, "\n") {
		t.Fatalf("entries joined by spaces run together once wrapped on a 20-column screen, got %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.Count(line, ";") != 1 {
			t.Fatalf("one command per line, got %q", line)
		}
	}
}
