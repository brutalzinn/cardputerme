package server

import "testing"

// The device shows (SCR_H-STATUS_H-HEADER_H)/(size*8) lines: 6 at size 2,
// 4 at size 3, 13 at size 1. If the server emits more, the trailing input line
// lands on a page the user never sees.
func deviceRows(size int) int { return (135 - 16 - 14) / (size * 8) }

func TestComposedFrameFitsTheScreenAtEveryZoom(t *testing.T) {
	rows := []string{}
	for i := range 40 {
		rows = append(rows, "line-"+string(rune('a'+i%26)))
	}
	for _, size := range []int{1, 2, 3} {
		s := withSession(New(Config{Name: "z", Session: "z", WrapCols: 20, LinesPerCard: 7, MaxCards: 40}))
		s.size = size
		s.sess.input = "typing here"
		st := s.composeMirror(gridLines(rows), "status", "", false)
		fit := deviceRows(size)
		t.Logf("size=%d serverLines=%d deviceRows=%d", size, len(st.lines), fit)
		if len(st.lines) > fit {
			t.Errorf("size %d: server sent %d lines, device shows %d — the input line is off-screen", size, len(st.lines), fit)
		}
	}
}
