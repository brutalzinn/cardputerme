package screen

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildDisplayAssembles(t *testing.T) {
	msg := BuildDisplay([]Line{{"hello world", Colors.Text}}, "[generic] ready", 2)
	if msg.Type != "display" || msg.Size != 2 || len(msg.Body) != 1 {
		t.Fatalf("got %+v", msg)
	}
	if msg.Body[0].Text != "hello world" || msg.Body[0].Color != Colors.Text {
		t.Fatalf("body %+v", msg.Body[0])
	}
	if msg.Status.Text != "[generic] ready" || msg.Status.Color != Colors.Status {
		t.Fatalf("status %+v", msg.Status)
	}
}

func TestLineColorRules(t *testing.T) {
	if LineColor("Proceed?", true) != Colors.Ask {
		t.Fatal("awaiting")
	}
	if LineColor("> run the plan", false) != Colors.Prompt {
		t.Fatal("prompt")
	}
	if LineColor("ok, running", false) != Colors.Text {
		t.Fatal("text")
	}
}

func TestBuildDisplayEmpty(t *testing.T) {
	msg := BuildDisplay([]Line{}, "", 2)
	if len(msg.Body) != 0 || msg.Status.Text != "" || msg.Status.Color != Colors.Status {
		t.Fatalf("got %+v", msg)
	}
}

func TestDisplayJSONShape(t *testing.T) {
	b, _ := json.Marshal(BuildDisplay([]Line{{"hi", 0xFFFF}}, "s", 3))
	want := `{"type":"display","size":3,"body":[{"text":"hi","color":65535}],"badge":"","status":{"text":"s","color":2047}}`
	if string(b) != want {
		t.Fatalf("json = %s", b)
	}
}

// The badge is a short server-chosen string for the device header. It carries
// the battery today; keeping it generic means what it shows can change without
// a re-flash.
func TestDisplayCarriesABadge(t *testing.T) {
	msg := BuildDisplayBadge([]Line{{"hi", 0xFFFF}}, "s", "53%", 2)
	if msg.Badge != "53%" {
		t.Fatalf("badge = %q", msg.Badge)
	}
	b, _ := json.Marshal(msg)
	if !strings.Contains(string(b), `"badge":"53%"`) {
		t.Fatalf("json = %s", b)
	}
}
