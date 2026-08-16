package screen

import "strings"

// StartsWithAny reports whether a row's visible text begins with one of the
// configured prefixes, once colour codes and leading tree markers are removed.
// The prefixes are configuration, so this stays free of app knowledge.
func StartsWithAny(raw string, prefixes []string) bool {
	text := strings.TrimSpace(ToAscii(StripAnsi(raw)))
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		if strings.HasPrefix(text, p) {
			return true
		}
	}
	return false
}
