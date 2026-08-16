package server

import (
	"strings"
	"testing"
)

// Absent or unknown must mean today's behaviour, so no existing caller changes
// when levels arrive.
func TestUnknownLevelDefaultsToAttention(t *testing.T) {
	for _, in := range []string{"", "shouty", "URGENT!!", "0"} {
		if got := levelOf(in); got != attention {
			t.Fatalf("levelOf(%q) = %v, want attention", in, got)
		}
	}
}

func TestLevelNamesAreCaseInsensitive(t *testing.T) {
	if levelOf("URGENT") != urgent || levelOf("Info") != info {
		t.Fatal("a caller should not have to guess the casing")
	}
}

// One sound for everything carries no information — the user learns to ignore
// it. Each level must be distinguishable by ear AND by eye.
func TestEachLevelLooksAndSoundsDifferent(t *testing.T) {
	seenLed := map[string]bool{}
	seenTone := map[int]bool{}
	for _, l := range []level{info, attention, urgent} {
		led := ledMessage(l.led())
		if seenLed[led] {
			t.Fatalf("level %v reuses an LED already taken: %s", l, led)
		}
		seenLed[led] = true

		notes := l.tones()
		if len(notes) == 0 {
			continue
		}
		if seenTone[notes[0].freq] {
			t.Fatalf("level %v reuses a lead tone: %d", l, notes[0].freq)
		}
		seenTone[notes[0].freq] = true
	}
}

// The device caches exactly ONE wav by URL and re-downloads over blocking HTTP
// whenever the URL changes. If more than one level used a wav the cache would
// thrash and the device would stall mid-render, so distinction is by TONE.
func TestOnlyOneLevelUsesTheWav(t *testing.T) {
	count := 0
	for _, l := range []level{info, attention, urgent} {
		if l.usesWav() {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%d levels want the wav; the device caches one", count)
	}
	if !attention.usesWav() {
		t.Fatal("the default level is the one that should get the real sound")
	}
}

// Urgent is a burst, so it must be several notes rather than one longer beep.
func TestUrgentIsABurstOfRisingTones(t *testing.T) {
	notes := urgent.tones()
	if len(notes) < 3 {
		t.Fatalf("urgent must be unmistakable, got %d notes", len(notes))
	}
	for i := 1; i < len(notes); i++ {
		if notes[i].freq <= notes[i-1].freq {
			t.Fatalf("a rising burst reads as urgent; got %v", notes)
		}
	}
}

func TestInfoIsASingleQuietNote(t *testing.T) {
	if got := len(info.tones()); got != 1 {
		t.Fatalf("info must not nag, got %d notes", got)
	}
}

func TestNotifyAcceptsALevel(t *testing.T) {
	s, srv := alertServer(t)
	_, body := postNotify(t, srv.URL, `{"session":"gitme","text":"deploy broken","level":"urgent"}`)
	if body["queued"] != true {
		t.Fatalf("body = %v", body)
	}
	if s.lastLed != ledMessage(urgent.led()) {
		t.Fatalf("led = %q, want the urgent colour", s.lastLed)
	}
}

// `;notify 0` is ONE switch over every channel. A level that could shout past
// it would make the switch a lie.
func TestSilenceBeatsEveryLevel(t *testing.T) {
	s, srv := alertServer(t)
	s.SetNotify(false)
	postNotify(t, srv.URL, `{"text":"deploy broken","level":"urgent"}`)
	if s.lastLed == ledMessage(urgent.led()) {
		t.Fatal("urgent is not an override for the user's choice")
	}
}

func TestLevelSurvivesIntoTheHeader(t *testing.T) {
	s, srv := alertServer(t)
	postNotify(t, srv.URL, `{"session":"gitme","text":"deploy broken","level":"urgent"}`)
	if got := headerText(s.headerCells()); !strings.Contains(got, "deploy broken") {
		t.Fatalf("header = %q", got)
	}
}
