package server

import (
	"strings"

	"cardputerme/internal/input"
)

var commands = map[string]func(s *Server, args string) string{
	"ping": func(s *Server, args string) string { return "Pong!" },
}

func runCommand(s *Server, line string) string {
	verb, args, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, input.CommandPrefix)), " ")
	if verb == "" {
		return ""
	}
	run, ok := commands[verb]
	if !ok {
		return "unknown command: " + verb
	}
	return run(s, strings.TrimSpace(args))
}
