package input

import "testing"

func pickerCtx(n int) KeyCtx { return KeyCtx{Beacons: n} }

func TestFnEscOpensThePicker(t *testing.T) {
	res := InterpretKey(State{}, "fn+esc", pickerCtx(3))
	if !res.State.Picking {
		t.Fatal("fn+esc must open the server picker")
	}
	if res.State.Pick != 0 {
		t.Fatalf("opens on the first entry, got %d", res.State.Pick)
	}
}

func TestPickerMovesAndWraps(t *testing.T) {
	cases := []struct {
		from int
		key  string
		want int
	}{
		{0, "down", 1},
		{1, "down", 2},
		{2, "down", 0},
		{0, "up", 2},
		{2, "up", 1},
	}
	for _, c := range cases {
		res := InterpretKey(State{Picking: true, Pick: c.from}, c.key, pickerCtx(3))
		if res.State.Pick != c.want {
			t.Fatalf("%s from %d = %d, want %d", c.key, c.from, res.State.Pick, c.want)
		}
		if !res.State.Picking {
			t.Fatal("moving must not leave the picker")
		}
	}
}

func TestPickerEnterConnectsToTheSelection(t *testing.T) {
	res := InterpretKey(State{Picking: true, Pick: 2}, "enter", pickerCtx(4))
	if res.Action.Kind != "connect" {
		t.Fatalf("enter must connect, got %q", res.Action.Kind)
	}
	if res.Action.Index != 2 {
		t.Fatalf("connect to the selected entry, got %d", res.Action.Index)
	}
	if res.State.Picking {
		t.Fatal("connecting leaves the picker")
	}
}

func TestPickerDigitPicksAVisibleRow(t *testing.T) {
	res := InterpretKey(State{Picking: true, Pick: 0}, "3", pickerCtx(9))
	if res.Action.Kind != "connectRow" {
		t.Fatalf("a digit picks a visible row, got %q", res.Action.Kind)
	}
	if res.Action.Index != 2 {
		t.Fatalf("row 3 is index 2 within the window, got %d", res.Action.Index)
	}
}

func TestPickerEscLeavesWithoutConnecting(t *testing.T) {
	res := InterpretKey(State{Picking: true, Pick: 1}, "esc", pickerCtx(3))
	if res.State.Picking {
		t.Fatal("esc closes the picker")
	}
	if res.Action.Kind != "none" {
		t.Fatalf("esc must not connect, got %q", res.Action.Kind)
	}
}

func TestPickerSwallowsTyping(t *testing.T) {
	res := InterpretKey(State{Picking: true, Pick: 0}, "a", pickerCtx(3))
	if res.State.Input != "" {
		t.Fatalf("letters must not reach the input buffer while picking, got %q", res.State.Input)
	}
	if res.Action.Kind != "none" {
		t.Fatalf("got %q", res.Action.Kind)
	}
}

func TestPickerWithNoBeaconsDoesNotCrashOrMove(t *testing.T) {
	for _, key := range []string{"up", "down", "enter", "1"} {
		res := InterpretKey(State{Picking: true, Pick: 0}, key, pickerCtx(0))
		if res.State.Pick != 0 {
			t.Fatalf("%s moved the selection with an empty list", key)
		}
	}
}

func TestFnEscInsideThePickerClosesIt(t *testing.T) {
	res := InterpretKey(State{Picking: true, Pick: 1}, "fn+esc", pickerCtx(3))
	if res.State.Picking {
		t.Fatal("fn+esc toggles the picker shut")
	}
}

func TestTypingIsUnaffectedWhenNotPicking(t *testing.T) {
	res := InterpretKey(State{Input: "he"}, "y", KeyCtx{})
	if res.State.Input != "hey" {
		t.Fatalf("normal typing must be untouched, got %q", res.State.Input)
	}
}
