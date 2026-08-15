package main

import "testing"

func TestPickPortFirstFree(t *testing.T) {
	probed := []int{}
	isFree := func(p int) bool { probed = append(probed, p); return p >= 4713 }
	if got := pickPort(isFree, 4711, 10); got != 4713 {
		t.Fatalf("got %d", got)
	}
	if !strsEqInt(probed, []int{4711, 4712, 4713}) {
		t.Fatalf("probed %+v", probed)
	}
}

func TestPickPortNoneFree(t *testing.T) {
	if got := pickPort(func(int) bool { return false }, 4711, 3); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestPickPortStartFree(t *testing.T) {
	if got := pickPort(func(int) bool { return true }, 4711, 3); got != 4711 {
		t.Fatalf("got %d", got)
	}
}

func TestPickPortDefaultRange(t *testing.T) {
	probed := []int{}
	isFree := func(p int) bool { probed = append(probed, p); return false }
	if got := pickPort(isFree, 8001, 255); got != 0 {
		t.Fatalf("got %d", got)
	}
	if probed[0] != 8001 || probed[len(probed)-1] != 8255 || len(probed) != 255 {
		t.Fatalf("range %d..%d n=%d", probed[0], probed[len(probed)-1], len(probed))
	}
}

func strsEqInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
