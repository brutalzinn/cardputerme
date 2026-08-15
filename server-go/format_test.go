package main

import (
	"strings"
	"testing"
)

func TestToAsciiTypography(t *testing.T) {
	if got := toAscii("“hi” — it’s a test…"); got != "\"hi\" - it's a test..." {
		t.Fatalf("got %q", got)
	}
}

func TestToAsciiDropsUnicode(t *testing.T) {
	if got := toAscii("Pong! 🏓"); got != "Pong! " {
		t.Fatalf("got %q", got)
	}
	if got := toAscii("╭─ box ─╮"); got != " box " {
		t.Fatalf("got %q", got)
	}
}

func TestToAsciiKeepsNewlines(t *testing.T) {
	if toAscii("a\nb") != "a\nb" {
		t.Fatal("newline")
	}
}

func TestToAsciiExpandsTabs(t *testing.T) {
	if got := toAscii("a\tb"); got != "a  b" {
		t.Fatalf("got %q", got)
	}
}

func strsEq(a, b []string) bool {
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

func TestWrapLineWords(t *testing.T) {
	if got := wrapLine("the quick brown fox", 10); !strsEq(got, []string{"the quick", "brown fox"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestWrapLineHardBreak(t *testing.T) {
	if got := wrapLine("supercalifragilistic", 10); !strsEq(got, []string{"supercalif", "ragilistic"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestSliceIntoCardsOneCard(t *testing.T) {
	cards := sliceIntoCards("one two three four five six", 100, 2, 40)
	if len(cards) != 1 || cards[0][0] != "one two three four five six" {
		t.Fatalf("got %+v", cards)
	}
}

func TestSliceIntoCardsCap(t *testing.T) {
	parts := make([]string, 20)
	for i := range parts {
		parts[i] = "line" + itoaSmall(i)
	}
	cards := sliceIntoCards(strings.Join(parts, "\n"), 20, 5, 3)
	if len(cards) != 3 {
		t.Fatalf("want 3 cards got %d", len(cards))
	}
	last := cards[len(cards)-1]
	if last[len(last)-1] != "line19" {
		t.Fatalf("last line %q", last[len(last)-1])
	}
}

func TestSliceIntoCardsSanitizes(t *testing.T) {
	cards := sliceIntoCards("Pong! 🏓", 20, 5, 40)
	if cards[0][0] != "Pong!" {
		t.Fatalf("got %q", cards[0][0])
	}
}

func itoaSmall(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
