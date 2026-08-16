package server

import (
	"strings"
	"testing"
)

func TestPickerItemsListsSessionsThenMachines(t *testing.T) {
	s := New(Config{Name: "thismachine", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.register("lara", "lara", "/tmp")
	s.handleBeacons([]beacon{
		{Name: "thismachine", IP: "192.168.0.20", Port: 8001},
		{Name: "laptop", IP: "192.168.0.30", Port: 8001},
	})

	items := s.pickerItems()
	if len(items) != 3 {
		t.Fatalf("2 sessions + 1 OTHER machine, got %d: %+v", len(items), items)
	}
	if items[0].session != "gitme" || items[1].session != "lara" {
		t.Fatalf("sessions come first: %+v", items)
	}
	if items[2].session != "" || items[2].machine.Name != "laptop" {
		t.Fatalf("machines come after sessions: %+v", items)
	}
}

func TestPickerItemsHidesThisMachine(t *testing.T) {
	s := New(Config{Name: "thismachine", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.handleBeacons([]beacon{{Name: "thismachine", IP: "192.168.0.20", Port: 8001}})

	for _, it := range s.pickerItems() {
		if it.session == "" && it.machine.Name == "thismachine" {
			t.Fatal("the machine we are already connected to must not be offered as a jump target")
		}
	}
}

func TestPickerItemsMarksTheCurrentSession(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.register("lara", "lara", "/tmp")
	items := s.pickerItems()
	if !items[0].current {
		t.Fatal("the session being viewed is marked")
	}
	if items[1].current {
		t.Fatal("only one item is current")
	}
}

func TestPickerItemsWithNothingAtAll(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	if got := s.pickerItems(); len(got) != 0 {
		t.Fatalf("no sessions and no beacons means an empty list, got %+v", got)
	}
}

func TestItemLinesLabelSessionsAndMachines(t *testing.T) {
	items := []pickerItem{
		{session: "gitme", current: true},
		{machine: beacon{Name: "laptop", IP: "192.168.0.30", Port: 8001}},
	}
	lines := itemLines(items, 0, 4, 20)
	if !strings.Contains(lines[0].Text, "gitme") {
		t.Fatalf("session row = %q", lines[0].Text)
	}
	if !strings.Contains(lines[1].Text, "laptop") {
		t.Fatalf("machine row = %q", lines[1].Text)
	}
	if lines[0].Text == lines[1].Text {
		t.Fatal("a session and a machine must not look identical")
	}
	for _, l := range lines {
		if len([]rune(l.Text)) > 20 {
			t.Fatalf("row overflows the screen: %q", l.Text)
		}
	}
}

func TestSwitchToChangesTheViewedSession(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.register("lara", "lara", "/tmp")
	if s.currentName() != "gitme" {
		t.Fatalf("precondition, current = %q", s.currentName())
	}
	s.switchTo("lara")
	if s.currentName() != "lara" {
		t.Fatalf("switchTo must change the viewed session, got %q", s.currentName())
	}
}

func TestSwitchToIgnoresAnUnknownSession(t *testing.T) {
	s := New(Config{Name: "m", WrapCols: 20})
	s.register("gitme", "gitme", "/tmp")
	s.switchTo("ghost")
	if s.currentName() != "gitme" {
		t.Fatalf("an unknown name must not blank the view, got %q", s.currentName())
	}
}
