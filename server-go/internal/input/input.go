package input

import "strings"

type State struct {
	Input   string
	Hist    int // -1 = none
	Cmd     string
	Picking bool
	Pick    int
}

type Action struct {
	Kind   string
	Key    string
	Text   string
	Index  int
	Repeat bool // safe to fire again while the key is held
}

type Result struct {
	State  State
	Action Action
}

type KeyCtx struct {
	Awaiting bool
	History  []string
	Beacons  int
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

const CommandPrefix = ";"

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

func commandKey(state State, key string) Result {
	typing := func(cmd string) Result {
		return Result{State{Input: state.Input, Hist: state.Hist, Cmd: cmd}, Action{Kind: "none"}}
	}
	if key == "esc" {
		return typing("")
	}
	if key == "enter" {
		return Result{State{Input: state.Input, Hist: state.Hist}, Action{Kind: "command", Text: strings.TrimPrefix(state.Cmd, CommandPrefix)}}
	}
	if key == "backspace" {
		return typing(state.Cmd[:len(state.Cmd)-1])
	}
	if len(key) == 1 {
		return typing(state.Cmd + key)
	}
	return typing(state.Cmd)
}

const PickerKey = "fn+esc"

func pickerKey(state State, key string, ctx KeyCtx) Result {
	stay := func(pick int) Result {
		return Result{State{Input: state.Input, Hist: state.Hist, Picking: true, Pick: pick}, Action{Kind: "none"}}
	}
	leave := func(a Action) Result {
		return Result{State{Input: state.Input, Hist: state.Hist}, a}
	}
	if key == "esc" || key == PickerKey {
		return leave(Action{Kind: "none"})
	}
	if ctx.Beacons <= 0 {
		return stay(state.Pick)
	}
	if key == "up" || key == "down" {
		delta := 1
		if key == "up" {
			delta = -1
		}
		return stay(((state.Pick+delta)%ctx.Beacons + ctx.Beacons) % ctx.Beacons)
	}
	if key == "enter" {
		return leave(Action{Kind: "connect", Index: state.Pick})
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return leave(Action{Kind: "connectRow", Index: int(key[0] - '1')})
	}
	return stay(state.Pick)
}

func InterpretKey(state State, key string, ctx KeyCtx) Result {
	if state.Picking {
		return pickerKey(state, key, ctx)
	}
	if key == PickerKey {
		return Result{State{Input: state.Input, Hist: state.Hist, Picking: true}, Action{Kind: "none"}}
	}
	if state.Cmd != "" {
		return commandKey(state, key)
	}
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
		if input == "" && key == CommandPrefix {
			return quiet(State{Input: input, Hist: hist, Cmd: CommandPrefix})
		}
		if ctx.Awaiting && input == "" && isDigits(key) {
			return press(state, key)
		}
		return quiet(mirror(input+key, hist))
	}
	return quiet(state)
}
