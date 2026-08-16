package screen

import "testing"

func TestAPlainRuleIsChromeWithNoTitle(t *testing.T) {
	title, ok := RuleTitle("──────────────────────────────")
	if !ok || title != "" {
		t.Fatalf("got %q ok=%v", title, ok)
	}
}

func TestATitledRuleYieldsItsTitle(t *testing.T) {
	title, ok := RuleTitle("───────────────── cardputer-protocol-spec ──")
	if !ok || title != "cardputer-protocol-spec" {
		t.Fatalf("got %q ok=%v", title, ok)
	}
}

func TestOrdinaryTextIsNotARule(t *testing.T) {
	if _, ok := RuleTitle("  the quick brown fox jumps over the lazy dog"); ok {
		t.Fatal("real content must never be mistaken for chrome")
	}
}

func TestAStrayBoxCharIsNotARule(t *testing.T) {
	if _, ok := RuleTitle("value ─ other"); ok {
		t.Fatal("a couple of box chars in a sentence is not a separator")
	}
}

func TestAColouredRuleIsStillARule(t *testing.T) {
	title, ok := RuleTitle("\x1b[2m──────────────── notes ──\x1b[0m")
	if !ok || title != "notes" {
		t.Fatalf("got %q ok=%v", title, ok)
	}
}

func TestHeavyAndDoubleRulesCount(t *testing.T) {
	for _, line := range []string{"━━━━━━━━━━━━━━━━ a ━━", "════════════════ b ══"} {
		if _, ok := RuleTitle(line); !ok {
			t.Fatalf("%q should be a rule", line)
		}
	}
}

func TestAnEmptyLineIsNotARule(t *testing.T) {
	if _, ok := RuleTitle("   "); ok {
		t.Fatal("blank is blank")
	}
}

func TestAMultiWordTitleKeepsItsSpaces(t *testing.T) {
	title, ok := RuleTitle("────────── my long session ──")
	if !ok || title != "my long session" {
		t.Fatalf("got %q ok=%v", title, ok)
	}
}
