package commands

import (
	"sort"
	"strings"

	"cardputerme/internal/input"
)

type Ctx struct {
	Name string
}

type Command struct {
	Help string
	Run  func(ctx Ctx, args string) string
}

var registry = map[string]Command{}

func register(verb string, c Command) {
	registry[verb] = c
}

func List() string {
	verbs := make([]string, 0, len(registry))
	for verb := range registry {
		verbs = append(verbs, verb)
	}
	sort.Strings(verbs)
	lines := make([]string, 0, len(verbs))
	for _, verb := range verbs {
		lines = append(lines, input.CommandPrefix+verb+" - "+registry[verb].Help)
	}
	return strings.Join(lines, "\n")
}

func Run(ctx Ctx, line string) string {
	verb, args, _ := strings.Cut(strings.TrimSpace(line), " ")
	if verb == "" {
		return ""
	}
	c, ok := registry[verb]
	if !ok {
		return "unknown command: " + verb
	}
	return c.Run(ctx, strings.TrimSpace(args))
}
