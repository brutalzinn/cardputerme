package screen

import (
	"encoding/json"
	"testing"
)

func TestBuildDisplayAssembles(t *testing.T) {
	msg := BuildDisplay([]Line{{"hello world", Colors.Text}}, "[generic] ready")
	if msg.Type != "display" || len(msg.Body) != 1 {
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
	msg := BuildDisplay([]Line{}, "")
	if len(msg.Body) != 0 || msg.Status.Text != "" || msg.Status.Color != Colors.Status {
		t.Fatalf("got %+v", msg)
	}
}

func TestDisplayJSONShape(t *testing.T) {
	b, _ := json.Marshal(BuildDisplay([]Line{{"hi", 0xFFFF}}, "s"))
	want := `{"type":"display","body":[{"text":"hi","color":65535}],"status":{"text":"s","color":2047}}`
	if string(b) != want {
		t.Fatalf("json = %s", b)
	}
}
