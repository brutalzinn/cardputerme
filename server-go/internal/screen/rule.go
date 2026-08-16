package screen

import "strings"

const minRuleRunes = 8

func isRuleRune(r rune) bool {
	return r == '─' || r == '━' || r == '═'
}

// RuleTitle reports whether a row is a horizontal rule drawn with box characters
// and returns any label embedded in it. Terminal UIs frame their input box that
// way, so the row is chrome and the label belongs in the status bar.
func RuleTitle(raw string) (string, bool) {
	rules := 0
	label := strings.Builder{}
	for _, r := range StripAnsi(raw) {
		if isRuleRune(r) {
			rules++
			continue
		}
		label.WriteRune(r)
	}
	if rules < minRuleRunes {
		return "", false
	}
	return strings.TrimSpace(label.String()), true
}
