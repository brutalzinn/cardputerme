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
	want := `{"type":"display","size":3,"body":[{"text":"hi","color":65535}],"header":[],"status":{"text":"s","color":2047}}`
	if string(b) != want {
		t.Fatalf("json = %s", b)
	}
}

// The header is a list of coloured runs the device draws left to right. It
// carries WiFi, link state and battery today; because the server composes it,
// what it says changes with a restart and never with a re-flash (#45).
func TestDisplayCarriesAComposedHeader(t *testing.T) {
	header := []Cell{{Text: "WiFi", Color: Colors.Status}, {Text: "  53%", Color: Colors.Dim}}
	msg := BuildDisplayHeader([]Line{{"hi", 0xFFFF}}, "s", header, 2)
	if len(msg.Header) != 2 || msg.Header[1].Text != "  53%" {
		t.Fatalf("header = %+v", msg.Header)
	}
	b, _ := json.Marshal(msg)
	if !strings.Contains(string(b), `"header":[{"text":"WiFi","color":2047},{"text":"  53%","color":33808}]`) {
		t.Fatalf("json = %s", b)
	}
}

// A nil header must serialize as an empty list, never null: the device iterates
// it, and a renderer that has to special-case null is a renderer that decides.
func TestDisplayHeaderIsNeverNull(t *testing.T) {
	b, _ := json.Marshal(BuildDisplayHeader(nil, "s", nil, 2))
	if !strings.Contains(string(b), `"header":[]`) {
		t.Fatalf("json = %s", b)
	}
}
