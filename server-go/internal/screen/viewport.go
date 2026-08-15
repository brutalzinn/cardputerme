package screen

const (
	RowStep = 1
	ColStep = 8
)

type View struct {
	Row    int
	Col    int
	Follow bool
	SelRow int
}

func PanViewport(v View, key string) View {
	switch key {
	case "up":
		return View{Row: max(0, v.Row-RowStep), Col: v.Col, Follow: false, SelRow: v.SelRow}
	case "down":
		return View{Row: v.Row + RowStep, Col: v.Col, Follow: false, SelRow: v.SelRow}
	case "left":
		return View{Row: v.Row, Col: max(0, v.Col-ColStep), Follow: v.Follow, SelRow: v.SelRow}
	case "right":
		return View{Row: v.Row, Col: v.Col + ColStep, Follow: v.Follow, SelRow: v.SelRow}
	}
	return v
}

func AnchorRow(selRow, viewRows int) int {
	return max(0, selRow-(viewRows-2))
}

func WindowLines(grid []Line, v View, rows, cols int) []Line {
	out := []Line{}
	for r := v.Row; r < v.Row+rows && r < len(grid); r++ {
		runes := []rune(grid[r].Text)
		lo := min(v.Col, len(runes))
		hi := min(v.Col+cols, len(runes))
		out = append(out, Line{Text: string(runes[lo:hi]), Color: grid[r].Color})
	}
	return out
}
