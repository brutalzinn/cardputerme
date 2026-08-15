package screen

import (
	"strconv"
	"strings"
)

type Choice struct {
	N     int
	Label string
}

func ParseChoices(text string) []Choice {
	out := []Choice{}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		i := 0
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if i == 0 || i >= len(line) {
			continue
		}
		sep := line[i]
		if sep != '.' && sep != ')' {
			continue
		}
		j := i + 1
		if j >= len(line) || line[j] != ' ' {
			continue
		}
		for j < len(line) && line[j] == ' ' {
			j++
		}
		label := strings.TrimSpace(line[j:])
		if label == "" {
			continue
		}
		n, _ := strconv.Atoi(line[:i])
		out = append(out, Choice{N: n, Label: label})
	}
	return out
}
