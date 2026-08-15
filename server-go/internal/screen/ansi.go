package screen

import "strconv"

func RGB565(r, g, b int) uint16 {
	return uint16(((r & 0xf8) << 8) | ((g & 0xfc) << 3) | (b >> 3))
}

var cube = [6]int{0, 95, 135, 175, 215, 255}

var sys16 = func() [16]uint16 {
	src := [16][3]int{
		{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
		{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	var out [16]uint16
	for i, t := range src {
		out[i] = RGB565(t[0], t[1], t[2])
	}
	return out
}()

func Xterm256(n int) uint16 {
	if n < 16 {
		return sys16[n]
	}
	if n < 232 {
		i := n - 16
		return RGB565(cube[(i/36)%6], cube[(i/6)%6], cube[i%6])
	}
	level := 8 + (n-232)*10
	return RGB565(level, level, level)
}

func atoiDefault(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func partAt(parts []string, i int) int {
	if i < 0 || i >= len(parts) {
		return 0
	}
	return atoiDefault(parts[i])
}

// applySGR folds one escape's SGR params into the running (fg, bold) state.
// Bold brightens the 8 basic colors (30-37 -> 90-97), matching how most
// terminals render bold+color, so the device mirrors what you actually see.
func applySGR(params string, curFg uint16, curBold bool, def uint16) (uint16, bool) {
	parts := splitByte(params, ';')
	if params == "" {
		parts = []string{"0"}
	}
	fg := curFg
	bold := curBold
	k := 0
	for k < len(parts) {
		n := partAt(parts, k)
		if n == 0 {
			fg = def
			bold = false
			k++
			continue
		}
		if n == 1 {
			bold = true
			k++
			continue
		}
		if n == 22 {
			bold = false
			k++
			continue
		}
		if n == 39 {
			fg = def
			k++
			continue
		}
		if n >= 30 && n <= 37 {
			if bold {
				fg = sys16[n-30+8]
			} else {
				fg = sys16[n-30]
			}
			k++
			continue
		}
		if n >= 90 && n <= 97 {
			fg = sys16[n-90+8]
			k++
			continue
		}
		if n == 38 {
			mode := partAt(parts, k+1)
			if mode == 5 {
				fg = Xterm256(partAt(parts, k+2))
				k += 3
				continue
			}
			if mode == 2 {
				fg = RGB565(partAt(parts, k+2), partAt(parts, k+3), partAt(parts, k+4))
				k += 5
				continue
			}
			k++
			continue
		}
		k++
	}
	return fg, bold
}

func isFinalByte(c byte) bool {
	return c >= '@' && c <= '~'
}

func ParseLine(raw string, def uint16) (string, uint16) {
	var text []byte
	curFg := def
	curBold := false
	lineColor := def
	colorSet := false
	n := len(raw)
	i := 0
	for i < n {
		c := raw[i]
		if c == 0x1b && i+1 < n && raw[i+1] == '[' {
			j := i + 2
			for j < n && !isFinalByte(raw[j]) {
				j++
			}
			if j < n && raw[j] == 'm' {
				curFg, curBold = applySGR(raw[i+2:j], curFg, curBold, def)
			}
			i = j + 1
			continue
		}
		if c == 0x1b {
			i++
			continue
		}
		text = append(text, c)
		if !colorSet && c != ' ' {
			lineColor = curFg
			colorSet = true
		}
		i++
	}
	return string(text), lineColor
}

func StripAnsi(s string) string {
	text, _ := ParseLine(s, 0)
	return text
}

func splitByte(s string, sep byte) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
