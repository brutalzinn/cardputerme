package screen

import "testing"

func TestPanUpDown(t *testing.T) {
	if got := PanViewport(View{Row: 5, Col: 0, Follow: true}, "up"); got.Row != 4 || got.Follow {
		t.Fatalf("up %+v", got)
	}
	if got := PanViewport(View{Row: 5, Col: 0, Follow: false}, "down"); got.Row != 6 || got.Follow {
		t.Fatalf("down %+v", got)
	}
}

func TestPanLeftRight(t *testing.T) {
	if PanViewport(View{Row: 0, Col: 10, Follow: false}, "left").Col != 2 {
		t.Fatal("left")
	}
	if PanViewport(View{Row: 0, Col: 0, Follow: false}, "left").Col != 0 {
		t.Fatal("left floor")
	}
	if PanViewport(View{Row: 0, Col: 0, Follow: false}, "right").Col != ColStep {
		t.Fatal("right")
	}
}

func TestPanFloorAndUnknown(t *testing.T) {
	if PanViewport(View{Row: 0, Col: 0, Follow: false}, "up").Row != 0 {
		t.Fatal("row floor")
	}
	got := PanViewport(View{Row: 2, Col: 2, Follow: true}, "zzz")
	if got.Row != 2 || got.Col != 2 || !got.Follow {
		t.Fatalf("unknown %+v", got)
	}
}

func TestWindowLines(t *testing.T) {
	grid := []Line{
		{"aaaabbbbcccc", 1}, {"ddddeeeeffff", 2}, {"gggghhhhiiii", 3},
	}
	win := WindowLines(grid, View{Row: 1, Col: 4}, 2, 4)
	if len(win) != 2 || win[0].Text != "eeee" || win[0].Color != 2 || win[1].Text != "hhhh" || win[1].Color != 3 {
		t.Fatalf("got %+v", win)
	}
}

func TestAnchorRow(t *testing.T) {
	if AnchorRow(10, 8) != 4 {
		t.Fatal("anchor")
	}
	if AnchorRow(1, 8) != 0 || AnchorRow(0, 8) != 0 {
		t.Fatal("floor")
	}
}

func TestWindowLinesEnd(t *testing.T) {
	grid := []Line{{"x", 9}}
	if len(WindowLines(grid, View{Row: 0, Col: 0}, 5, 10)) != 1 {
		t.Fatal("end")
	}
}
