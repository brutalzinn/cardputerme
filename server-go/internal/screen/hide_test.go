package screen

import "testing"

var hints = []string{"Tip:", "Note:"}

func TestAHintRowIsRecognised(t *testing.T) {
	if !StartsWithAny("  Tip: Use /btw to ask a side question", hints) {
		t.Fatal("should match after the leading markers are trimmed")
	}
}

func TestAnsiAndTreeMarkersDoNotHideAHint(t *testing.T) {
	if !StartsWithAny("\x1b[2m  ⎿  Tip: something\x1b[0m", hints) {
		t.Fatal("colour codes and the tree marker must be stripped first")
	}
}

func TestOrdinaryContentIsKept(t *testing.T) {
	if StartsWithAny("  the tip of the iceberg", hints) {
		t.Fatal("a word in a sentence is not a hint row")
	}
}

func TestMatchingIsCaseSensitive(t *testing.T) {
	if StartsWithAny("tip: lowercase", hints) {
		t.Fatal("only the exact prefix counts, so ordinary prose survives")
	}
}

func TestNoPrefixesHidesNothing(t *testing.T) {
	if StartsWithAny("Tip: anything", nil) {
		t.Fatal("an empty config must hide nothing")
	}
}

func TestAnEmptyPrefixIsIgnored(t *testing.T) {
	if StartsWithAny("anything at all", []string{""}) {
		t.Fatal("an empty prefix would hide every row")
	}
}
