package server

import (
	"strings"
	"testing"
)

func manyBeacons(n int) []beacon {
	out := make([]beacon, n)
	for i := range out {
		out[i] = beacon{Name: string(rune('a' + i)), IP: "192.168.0.20", Port: 8001 + i}
	}
	return out
}

func TestPickerStartKeepsTheSelectionVisible(t *testing.T) {
	cases := []struct {
		n, pick, rows, want int
	}{
		{3, 0, 4, 0},
		{3, 2, 4, 0},
		{9, 0, 4, 0},
		{9, 8, 4, 5},
		{9, 4, 4, 2},
		{0, 0, 4, 0},
	}
	for _, c := range cases {
		got := pickerStart(c.pick, c.n, c.rows)
		if got != c.want {
			t.Fatalf("pickerStart(pick=%d n=%d rows=%d) = %d, want %d", c.pick, c.n, c.rows, got, c.want)
		}
		if c.n > c.rows && (c.pick < got || c.pick >= got+c.rows) {
			t.Fatalf("selection %d fell outside window [%d,%d)", c.pick, got, got+c.rows)
		}
	}
}

func TestPickerLinesNumbersTheVisibleWindow(t *testing.T) {
	lines := pickerLines(manyBeacons(9), 8, 4, 20)
	if len(lines) != 4 {
		t.Fatalf("want 4 rows, got %d", len(lines))
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0].Text), "1.") {
		t.Fatalf("rows are numbered from 1 within the window, got %q", lines[0].Text)
	}
	last := lines[3].Text
	if !strings.Contains(last, "i") {
		t.Fatalf("the ninth beacon must be visible when selected, got %q", last)
	}
}

func TestPickerLinesMarksTheSelection(t *testing.T) {
	lines := pickerLines(manyBeacons(3), 1, 4, 20)
	if !strings.Contains(lines[1].Text, ">") {
		t.Fatalf("the selected row carries a cursor, got %q", lines[1].Text)
	}
	if strings.Contains(lines[0].Text, ">") {
		t.Fatalf("unselected rows carry none, got %q", lines[0].Text)
	}
}

func TestPickerLinesWithNoBeacons(t *testing.T) {
	lines := pickerLines(nil, 0, 4, 20)
	if len(lines) == 0 {
		t.Fatal("an empty list still tells the user what is happening")
	}
	if strings.Contains(lines[0].Text, ">") {
		t.Fatal("nothing to select, so no cursor")
	}
}

func TestPickerLinesTruncatesToWidth(t *testing.T) {
	long := []beacon{{Name: "a-very-long-exposure-name-indeed", IP: "192.168.0.20", Port: 8001}}
	for _, l := range pickerLines(long, 0, 4, 20) {
		if len([]rune(l.Text)) > 20 {
			t.Fatalf("row exceeds the 20-column screen: %q", l.Text)
		}
	}
}

func TestConnectMessage(t *testing.T) {
	got := connectMessage(beacon{Name: "gitme", IP: "192.168.0.20", Port: 8002})
	want := `{"type":"connect","ip":"192.168.0.20","port":8002}`
	if got != want {
		t.Fatalf("connectMessage = %s, want %s", got, want)
	}
}

func TestPickerStatusShowsPosition(t *testing.T) {
	if got := pickerStatus(manyBeacons(9), 4); !strings.Contains(got, "5/9") {
		t.Fatalf("status must show position in the full list, got %q", got)
	}
	if got := pickerStatus(nil, 0); strings.Contains(got, "/") {
		t.Fatalf("no position with an empty list, got %q", got)
	}
}
