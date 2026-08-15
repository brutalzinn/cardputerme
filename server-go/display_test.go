package main

import (
	"encoding/json"
	"testing"
)

func TestBuildDisplayAssembles(t *testing.T) {
	msg := buildDisplay([]Line{{"hello world", colors.Text}}, "[generic] ready")
	if msg.Type != "display" || len(msg.Body) != 1 {
		t.Fatalf("got %+v", msg)
	}
	if msg.Body[0].Text != "hello world" || msg.Body[0].Color != colors.Text {
		t.Fatalf("body %+v", msg.Body[0])
	}
	if msg.Status.Text != "[generic] ready" || msg.Status.Color != colors.Status {
		t.Fatalf("status %+v", msg.Status)
	}
}

func TestLineColorRules(t *testing.T) {
	if lineColor("Proceed?", true) != colors.Ask {
		t.Fatal("awaiting")
	}
	if lineColor("> run the plan", false) != colors.Prompt {
		t.Fatal("prompt")
	}
	if lineColor("ok, running", false) != colors.Text {
		t.Fatal("text")
	}
}

func TestBuildDisplayEmpty(t *testing.T) {
	msg := buildDisplay([]Line{}, "")
	if len(msg.Body) != 0 || msg.Status.Text != "" || msg.Status.Color != colors.Status {
		t.Fatalf("got %+v", msg)
	}
}

func TestDisplayJSONShape(t *testing.T) {
	b, _ := json.Marshal(buildDisplay([]Line{{"hi", 0xFFFF}}, "s"))
	want := `{"type":"display","body":[{"text":"hi","color":65535}],"status":{"text":"s","color":2047}}`
	if string(b) != want {
		t.Fatalf("json = %s", b)
	}
}
