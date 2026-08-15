package screen

import "testing"

func choicesEq(a []Choice, b []Choice) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseChoicesDot(t *testing.T) {
	got := ParseChoices("Pick one:\n1. Yes\n2. No, keep it")
	want := []Choice{{1, "Yes"}, {2, "No, keep it"}}
	if !choicesEq(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseChoicesParenIndent(t *testing.T) {
	got := ParseChoices("  1) Alpha\n  2) Beta")
	want := []Choice{{1, "Alpha"}, {2, "Beta"}}
	if !choicesEq(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseChoicesIgnoresNonOptions(t *testing.T) {
	if got := ParseChoices("3.14 is pi\nno number here\n1.no space"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
