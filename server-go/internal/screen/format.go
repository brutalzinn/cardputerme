package screen

import "strings"

var translit = map[rune]string{
	'‘': "'", '’': "'", '‛': "'",
	'“': "\"", '”': "\"",
	'–': "-", '—': "-", '−': "-",
	'…': "...",
	'•': "*", '·': "*",
	'❯': ">", '▶': ">", '›': ">",
	'→': "->", '⇒': "->",
	'←': "<-", '⇐': "<-",
	'✓': "v", '✔': "v",
	' ': " ",
}

func ToAscii(s string) string {
	s = strings.ReplaceAll(s, "\t", "  ")
	var b strings.Builder
	for _, r := range s {
		if rep, ok := translit[r]; ok {
			b.WriteString(rep)
			continue
		}
		if r == '\n' || (r >= 0x20 && r <= 0x7e) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func WrapLine(line string, cols int) []string {
	runes := []rune(line)
	if len(runes) <= cols {
		return []string{line}
	}
	out := []string{}
	cur := ""
	for _, word := range strings.Fields(line) {
		wr := []rune(word)
		for len(wr) > cols {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			out = append(out, string(wr[:cols]))
			wr = wr[cols:]
		}
		word = string(wr)
		if cur == "" {
			cur = word
			continue
		}
		if len([]rune(cur))+1+len(wr) <= cols {
			cur += " " + word
			continue
		}
		out = append(out, cur)
		cur = word
	}
	if cur != "" {
		out = append(out, cur)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func SliceIntoCards(text string, cols, linesPerCard, maxCards int) [][]string {
	cleaned := ToAscii(strings.ReplaceAll(StripAnsi(text), "\r", ""))
	rawLines := strings.Split(cleaned, "\n")
	for len(rawLines) > 0 && strings.TrimSpace(rawLines[len(rawLines)-1]) == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	wrapped := []string{}
	for _, line := range rawLines {
		for _, w := range WrapLine(line, cols) {
			wrapped = append(wrapped, strings.TrimRight(w, " "))
		}
	}

	cards := [][]string{}
	page := max(1, linesPerCard)
	for i := 0; i < len(wrapped); i += page {
		end := min(i+page, len(wrapped))
		cards = append(cards, wrapped[i:end])
	}
	if maxCards > 0 && len(cards) > maxCards {
		return cards[len(cards)-maxCards:]
	}
	return cards
}
