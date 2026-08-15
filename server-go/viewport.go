package main

const (
	rowStep = 1
	colStep = 8
)

type View struct {
	Row    int
	Col    int
	Follow bool
	SelRow int
}

type Line struct {
	Text  string
	Color uint16
}

func panViewport(v View, key string) View {
	switch key {
	case "up":
		return View{Row: maxInt(0, v.Row-rowStep), Col: v.Col, Follow: false, SelRow: v.SelRow}
	case "down":
		return View{Row: v.Row + rowStep, Col: v.Col, Follow: false, SelRow: v.SelRow}
	case "left":
		return View{Row: v.Row, Col: maxInt(0, v.Col-colStep), Follow: v.Follow, SelRow: v.SelRow}
	case "right":
		return View{Row: v.Row, Col: v.Col + colStep, Follow: v.Follow, SelRow: v.SelRow}
	}
	return v
}

func anchorRow(selRow, viewRows int) int {
	return maxInt(0, selRow-(viewRows-2))
}

func windowLines(grid []Line, v View, rows, cols int) []Line {
	out := []Line{}
	for r := v.Row; r < v.Row+rows && r < len(grid); r++ {
		runes := []rune(grid[r].Text)
		lo := minInt(v.Col, len(runes))
		hi := minInt(v.Col+cols, len(runes))
		out = append(out, Line{Text: string(runes[lo:hi]), Color: grid[r].Color})
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
