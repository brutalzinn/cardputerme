package screen

import "testing"

const esc = "\x1b"

func TestRGB565(t *testing.T) {
	cases := []struct {
		r, g, b int
		want    uint16
	}{
		{255, 255, 255, 0xFFFF}, {0, 0, 0, 0x0000}, {255, 0, 0, 0xF800},
		{0, 255, 0, 0x07E0}, {0, 0, 255, 0x001F},
	}
	for _, c := range cases {
		if got := RGB565(c.r, c.g, c.b); got != c.want {
			t.Fatalf("RGB565(%d,%d,%d)=%#x want %#x", c.r, c.g, c.b, got, c.want)
		}
	}
}

func TestXterm256(t *testing.T) {
	if Xterm256(196) != RGB565(255, 0, 0) {
		t.Fatal("196")
	}
	if Xterm256(231) != RGB565(255, 255, 255) {
		t.Fatal("231")
	}
	if Xterm256(16) != RGB565(0, 0, 0) {
		t.Fatal("16")
	}
	if Xterm256(255) != RGB565(238, 238, 238) {
		t.Fatal("255")
	}
}

func TestStripAnsi(t *testing.T) {
	if StripAnsi(esc+"[31mhi"+esc+"[0m") != "hi" {
		t.Fatal("colored")
	}
	if StripAnsi("plain") != "plain" {
		t.Fatal("plain")
	}
	if StripAnsi(esc+"[38;5;246m x "+esc+"[39m") != " x " {
		t.Fatal("256")
	}
}

func TestParseLine(t *testing.T) {
	txt, col := ParseLine(esc+"[38;5;231muser typed this"+esc+"[39m", 0xFFFF)
	if txt != "user typed this" || col != Xterm256(231) {
		t.Fatalf("got %q %#x", txt, col)
	}
	txt, col = ParseLine("just text", 0xFFFF)
	if txt != "just text" || col != 0xFFFF {
		t.Fatal("default color")
	}
	_, col = ParseLine(esc+"[38;2;255;0;0mred", 0xFFFF)
	if col != RGB565(255, 0, 0) {
		t.Fatal("truecolor")
	}
	txt, col = ParseLine("   "+esc+"[38;5;196mX", 0xFFFF)
	if txt != "   X" || col != Xterm256(196) {
		t.Fatalf("leading spaces got %q %#x", txt, col)
	}
}

func TestParseLineBoldBrightens(t *testing.T) {
	_, col := ParseLine(esc+"[1;31mX", 0xFFFF)
	if col != sys16[9] {
		t.Fatalf("bold+31 got %#x want bright red %#x", col, sys16[9])
	}
	_, col = ParseLine(esc+"[1m"+esc+"[32mX", 0xFFFF)
	if col != sys16[10] {
		t.Fatalf("bold then 32 got %#x want bright green %#x", col, sys16[10])
	}
	_, col = ParseLine(esc+"[31mX", 0xFFFF)
	if col != sys16[1] {
		t.Fatalf("plain 31 got %#x want normal red %#x", col, sys16[1])
	}
}

func TestParseLineBoldBrightensRegardlessOfOrder(t *testing.T) {
	_, col := ParseLine(esc+"[31;1mX", 0xFFFF)
	if col != sys16[9] {
		t.Fatalf("31;1 got %#x want bright red %#x", col, sys16[9])
	}
	_, col = ParseLine(esc+"[34m"+esc+"[1mX", 0xFFFF)
	if col != sys16[12] {
		t.Fatalf("34m then 1m before text got %#x want bright blue %#x", col, sys16[12])
	}
	_, col = ParseLine(esc+"[1;31;22mX", 0xFFFF)
	if col != sys16[1] {
		t.Fatalf("1;31;22 (unbolded) got %#x want normal red %#x", col, sys16[1])
	}
	_, col = ParseLine(esc+"[1;38;5;196mX", 0xFFFF)
	if col != Xterm256(196) {
		t.Fatalf("bold + 256-color got %#x want %#x (256 unaffected by bold)", col, Xterm256(196))
	}
}
