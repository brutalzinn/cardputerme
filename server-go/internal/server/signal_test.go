package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLedMessage(t *testing.T) {
	cases := []struct {
		in   led
		want string
	}{
		{led{}, `{"type":"led","r":0,"g":0,"b":0,"pattern":"off"}`},
		{led{R: 255, G: 60, B: 0, Pattern: "pulse"}, `{"type":"led","r":255,"g":60,"b":0,"pattern":"pulse"}`},
		{led{R: 1, G: 2, B: 3, Pattern: "solid"}, `{"type":"led","r":1,"g":2,"b":3,"pattern":"solid"}`},
	}
	for _, c := range cases {
		if got := ledMessage(c.in); got != c.want {
			t.Fatalf("ledMessage(%+v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestLedOffIsTheZeroValue(t *testing.T) {
	if ledMessage(ledOff) != ledMessage(led{}) {
		t.Fatal("ledOff must be the dark, patternless state")
	}
}

func TestSoundMessageCarriesACompleteUrl(t *testing.T) {
	got := soundMessage("http://192.168.0.20:8001/sound/notify.wav")
	want := `{"type":"sound","url":"http://192.168.0.20:8001/sound/notify.wav"}`
	if got != want {
		t.Fatalf("soundMessage = %s, want %s", got, want)
	}
}

func TestSoundUrlIsMintedFromTheAddressTheDeviceReached(t *testing.T) {
	s := New(Config{Name: "sig", WrapCols: 20, SoundsDir: "/tmp"})
	s.setSoundBase("192.168.0.20:8001")
	if got := s.soundURL("notify.wav"); got != "http://192.168.0.20:8001/sound/notify.wav" {
		t.Fatalf("soundURL = %s", got)
	}
}

func TestSoundUrlIsEmptyUntilAClientHasConnected(t *testing.T) {
	s := New(Config{Name: "sig", WrapCols: 20, SoundsDir: "/tmp"})
	if got := s.soundURL("notify.wav"); got != "" {
		t.Fatalf("without a known address there is no URL to send, got %q", got)
	}
}

func TestSoundUrlIsEmptyWithoutASoundsDir(t *testing.T) {
	s := New(Config{Name: "sig", WrapCols: 20})
	s.setSoundBase("192.168.0.20:8001")
	if got := s.soundURL("notify.wav"); got != "" {
		t.Fatalf("no sounds dir means no URL, got %q", got)
	}
}

func TestToneMessageFallback(t *testing.T) {
	want := `{"type":"sound","freq":1200,"ms":90}`
	if got := toneMessage(1200, 90); got != want {
		t.Fatalf("toneMessage = %s, want %s", got, want)
	}
}

func soundServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	s := New(Config{Name: "sig", WrapCols: 20, SoundsDir: dir})
	mux := http.NewServeMux()
	mux.Handle(soundPrefix, s.soundHandler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSoundHandlerServesAFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notify.wav"), []byte("RIFFfake"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := soundServer(t, dir)
	res, err := http.Get(srv.URL + "/sound/notify.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200", res.StatusCode)
	}
}

func TestSoundHandlerMissingFileIs404(t *testing.T) {
	srv := soundServer(t, t.TempDir())
	res, err := http.Get(srv.URL + "/sound/nope.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", res.StatusCode)
	}
}

func TestSoundHandlerRefusesTraversal(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	if err := os.WriteFile(secret, []byte("private"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(secret) })
	srv := soundServer(t, dir)
	res, err := http.Get(srv.URL + "/sound/../secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("the beacon advertises this port to the whole subnet — traversal must never serve outside the sounds dir")
	}
}

func TestSoundHandlerWithNoDirConfigured(t *testing.T) {
	srv := soundServer(t, "")
	res, err := http.Get(srv.URL + "/sound/notify.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		t.Fatal("no sounds dir configured must not serve anything")
	}
}
