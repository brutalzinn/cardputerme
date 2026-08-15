package main

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
		if got := rgb565(c.r, c.g, c.b); got != c.want {
			t.Fatalf("rgb565(%d,%d,%d)=%#x want %#x", c.r, c.g, c.b, got, c.want)
		}
	}
}

func TestXterm256(t *testing.T) {
	if xterm256(196) != rgb565(255, 0, 0) {
		t.Fatal("196")
	}
	if xterm256(231) != rgb565(255, 255, 255) {
		t.Fatal("231")
	}
	if xterm256(16) != rgb565(0, 0, 0) {
		t.Fatal("16")
	}
	if xterm256(255) != rgb565(238, 238, 238) {
		t.Fatal("255")
	}
}

func TestStripAnsi(t *testing.T) {
	if stripAnsi(esc+"[31mhi"+esc+"[0m") != "hi" {
		t.Fatal("colored")
	}
	if stripAnsi("plain") != "plain" {
		t.Fatal("plain")
	}
	if stripAnsi(esc+"[38;5;246m x "+esc+"[39m") != " x " {
		t.Fatal("256")
	}
}

func TestParseLine(t *testing.T) {
	txt, col := parseLine(esc+"[38;5;231muser typed this"+esc+"[39m", 0xFFFF)
	if txt != "user typed this" || col != xterm256(231) {
		t.Fatalf("got %q %#x", txt, col)
	}
	txt, col = parseLine("just text", 0xFFFF)
	if txt != "just text" || col != 0xFFFF {
		t.Fatal("default color")
	}
	_, col = parseLine(esc+"[38;2;255;0;0mred", 0xFFFF)
	if col != rgb565(255, 0, 0) {
		t.Fatal("truecolor")
	}
	txt, col = parseLine("   "+esc+"[38;5;196mX", 0xFFFF)
	if txt != "   X" || col != xterm256(196) {
		t.Fatalf("leading spaces got %q %#x", txt, col)
	}
}

func TestParseLineBoldBrightens(t *testing.T) {
	// bold + red (1;31) renders as bright red, matching most terminals
	_, col := parseLine(esc+"[1;31mX", 0xFFFF)
	if col != sys16[9] {
		t.Fatalf("bold+31 got %#x want bright red %#x", col, sys16[9])
	}
	// bold set in a prior escape carries into the next color
	_, col = parseLine(esc+"[1m"+esc+"[32mX", 0xFFFF)
	if col != sys16[10] {
		t.Fatalf("bold then 32 got %#x want bright green %#x", col, sys16[10])
	}
	// non-bold basic color stays normal
	_, col = parseLine(esc+"[31mX", 0xFFFF)
	if col != sys16[1] {
		t.Fatalf("plain 31 got %#x want normal red %#x", col, sys16[1])
	}
}
