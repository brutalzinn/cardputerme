package input

import "strings"

type State struct {
	Input string
	Hist  int // -1 = none
}

type Action struct {
	Kind   string // none | send | pressKey | pan
	Key    string
	Text   string
	Repeat bool // safe to fire again while the key is held
}

type Result struct {
	State  State
	Action Action
}

type KeyCtx struct {
	Awaiting bool
	History  []string
}

func isDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

var arrows = map[string]bool{"up": true, "down": true, "left": true, "right": true}

// suggest returns the newest history entry that extends the current input, or
// "" when there is none — the basis for autosuggest + Tab-accept.
func Suggest(input string, history []string) string {
	if input == "" {
		return ""
	}
	for i := len(history) - 1; i >= 0; i-- {
		if history[i] != input && strings.HasPrefix(history[i], input) {
			return history[i]
		}
	}
	return ""
}

func splitMod(k string) (mod, base string) {
	i := strings.IndexByte(k, '+')
	if i <= 0 {
		return "", k
	}
	return k[:i], k[i+1:]
}

func InterpretKey(state State, key string, ctx KeyCtx) Result {
	input := state.Input
	hist := state.Hist
	mirror := func(newInput string, newHist int) State { return State{Input: newInput, Hist: newHist} }
	press := func(s State, k string) Result {
		return Result{s, Action{Kind: "pressKey", Key: k, Repeat: arrows[k]}}
	}
	quiet := func(s State) Result { return Result{s, Action{Kind: "none"}} }

	mod, base := splitMod(key)

	if mod == "shift" {
		if base == "esc" {
			return press(state, "escape")
		}
		if base == "enter" {
			return quiet(mirror(input+"\n", hist))
		}
		return quiet(state)
	}
	if mod == "ctrl" {
		if base == "=" || base == "+" {
			return Result{state, Action{Kind: "zoom", Key: "in"}}
		}
		if base == "-" || base == "_" {
			return Result{state, Action{Kind: "zoom", Key: "out"}}
		}
		if base == " " {
			return Result{state, Action{Kind: "zoom", Key: "reset"}}
		}
		if base == "up" {
			if len(ctx.History) == 0 {
				return quiet(state)
			}
			idx := len(ctx.History) - 1
			if hist != -1 {
				idx = max(hist-1, 0)
			}
			return quiet(mirror(ctx.History[idx], idx))
		}
		if base == "down" {
			if hist == -1 {
				return quiet(state)
			}
			idx := hist + 1
			if idx >= len(ctx.History) {
				return quiet(mirror("", -1))
			}
			return quiet(mirror(ctx.History[idx], idx))
		}
		if len(base) == 1 {
			return press(state, "ctrl+"+base)
		}
		return quiet(state)
	}
	if mod == "opt" {
		if arrows[base] {
			return press(state, base)
		}
		return quiet(state)
	}

	if key == "esc" {
		if len(input) > 0 {
			return quiet(mirror("", -1))
		}
		return press(state, "escape")
	}
	if key == "enter" {
		if len(input) > 0 {
			return Result{mirror("", -1), Action{Kind: "send", Text: input}}
		}
		return press(state, "enter")
	}
	if key == "backspace" {
		if input == "" {
			return quiet(mirror("", hist))
		}
		return quiet(mirror(input[:len(input)-1], hist))
	}
	if key == "tab" {
		if s := Suggest(input, ctx.History); s != "" {
			return quiet(mirror(s, -1))
		}
		return press(state, "tab")
	}
	if arrows[key] {
		return Result{state, Action{Kind: "pan", Key: key, Repeat: true}}
	}
	if len(key) == 1 {
		if ctx.Awaiting && input == "" && isDigits(key) {
			return press(state, key)
		}
		return quiet(mirror(input+key, hist))
	}
	return quiet(state)
}
