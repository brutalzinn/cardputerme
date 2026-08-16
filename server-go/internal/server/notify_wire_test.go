package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeDevice struct {
	t   *testing.T
	c   *websocket.Conn
	srv *Server
	url string
}

func writeFile(dir, name, body string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
}

func (d *fakeDevice) dialAnother() *fakeDevice {
	d.t.Helper()
	c, _, err := websocket.DefaultDialer.Dial(d.url, nil)
	if err != nil {
		d.t.Fatalf("second dial: %v", err)
	}
	d.t.Cleanup(func() { c.Close() })
	return &fakeDevice{t: d.t, c: c, srv: d.srv, url: d.url}
}

// dialFake stands a real server in front of a real tmux session and connects a
// gorilla client in the firmware's place. notify/led/sound ride the ordered
// broadcast queue, so they are deliverable and assertable here.
func dialFake(t *testing.T, sess string, cfg Config) *fakeDevice {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	exec.Command("tmux", "kill-session", "-t", sess).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", sess, "-c", "/tmp").Run(); err != nil {
		t.Skipf("cannot create tmux session: %v", err)
	}
	t.Cleanup(func() { exec.Command("tmux", "kill-session", "-t", sess).Run() })

	cfg.Name = sess
	s := withSession(New(cfg))
	srv := httptest.NewServer(http.HandlerFunc(s.wsHandler))
	t.Cleanup(srv.Close)

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return &fakeDevice{t: t, c: c, srv: s, url: url}
}

type frame struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Pattern string `json:"pattern"`
	R       int    `json:"r"`
	G       int    `json:"g"`
	B       int    `json:"b"`
	URL     string `json:"url"`
	Freq    int    `json:"freq"`
	State   string `json:"state"`
}

// await collects frames until every wanted type has been seen, so the assertion
// does not depend on delivery order.
func (d *fakeDevice) await(want []string, within time.Duration) map[string]frame {
	d.t.Helper()
	got := map[string]frame{}
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		d.c.SetReadDeadline(deadline)
		_, data, err := d.c.ReadMessage()
		if err != nil {
			break
		}
		var f frame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		got[f.Type] = f
		all := true
		for _, w := range want {
			if _, ok := got[w]; !ok {
				all = false
			}
		}
		if all {
			return got
		}
	}
	missing := []string{}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		d.t.Fatalf("never received %v (saw %v)", missing, keysOf(got))
	}
	return got
}

func keysOf(m map[string]frame) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}

func notifyConfig() Config {
	return Config{WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40, Notify: true}
}

// A question the SERVER noticed must reach the human through the same channels
// an HTTP caller gets — LED, sound, and the header line. It used to broadcast a
// `notify` frame instead, which no client has parsed since #45 deleted the toast.
func TestWireFreshQuestionSignalsLedSoundAndHeader(t *testing.T) {
	d := dialFake(t, "wiretest-notify", notifyConfig())
	d.await([]string{"power"}, 5*time.Second)

	exec.Command("tmux", "send-keys", "-t", "wiretest-notify", "-l",
		"printf 'Deploy?\\n1. Yes\\n2. No\\n'; read x").Run()
	exec.Command("tmux", "send-keys", "-t", "wiretest-notify", "Enter").Run()
	time.Sleep(400 * time.Millisecond)
	d.srv.pushIfChanged(false)

	got := d.await([]string{"led", "sound"}, 6*time.Second)

	if h := headerText(d.srv.headerCells()); !strings.Contains(h, awaitingAlert) {
		t.Fatalf("the server's own detection must reach the header too, got %q", h)
	}
	if got["led"].Pattern != "pulse" {
		t.Fatalf("a pending question pulses, got %q", got["led"].Pattern)
	}
	if got["led"].R == 0 && got["led"].G == 0 && got["led"].B == 0 {
		t.Fatal("an attention LED must not be dark")
	}
	if got["sound"].Freq == 0 && got["sound"].URL == "" {
		t.Fatal("sound must carry either a tone or a URL")
	}
}

func TestWireSoundUsesTheConfiguredWavWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(dir, "notify.wav", "RIFFfake"); err != nil {
		t.Fatal(err)
	}
	cfg := notifyConfig()
	cfg.SoundsDir = dir
	cfg.NotifySound = "notify.wav"
	d := dialFake(t, "wiretest-wav", cfg)
	d.await([]string{"power"}, 5*time.Second)

	exec.Command("tmux", "send-keys", "-t", "wiretest-wav", "-l",
		"printf 'Deploy?\\n1. Yes\\n2. No\\n'; read x").Run()
	exec.Command("tmux", "send-keys", "-t", "wiretest-wav", "Enter").Run()
	time.Sleep(400 * time.Millisecond)
	d.srv.pushIfChanged(false)

	got := d.await([]string{"sound"}, 6*time.Second)
	if !strings.HasSuffix(got["sound"].URL, "/sound/notify.wav") {
		t.Fatalf("sound URL = %q, want the served WAV", got["sound"].URL)
	}
	if !strings.HasPrefix(got["sound"].URL, "http://") {
		t.Fatalf("the device must receive a COMPLETE url, got %q", got["sound"].URL)
	}
}

// A device that reconnects boots dark, so the server must re-send LED state or
// the dedupe in setLed silently leaves it unlit forever.
func TestWireLedStateIsResentOnConnect(t *testing.T) {
	d := dialFake(t, "wiretest-ledresend", notifyConfig())
	d.await([]string{"led"}, 5*time.Second)

	d.srv.setLed(ledAttention)
	d.await([]string{"led"}, 5*time.Second)

	fresh := d.dialAnother()
	got := fresh.await([]string{"led"}, 5*time.Second)
	if got["led"].Pattern != "pulse" {
		t.Fatalf("a reconnecting device must be told the CURRENT led state, got %q", got["led"].Pattern)
	}
}

func TestWireLedIsControllableAtAnyTime(t *testing.T) {
	d := dialFake(t, "wiretest-ledctl", notifyConfig())
	d.await([]string{"power", "led"}, 5*time.Second)

	d.srv.setLed(led{R: 10, G: 20, B: 30, Pattern: "solid"})
	got := d.await([]string{"led"}, 5*time.Second)
	if got["led"].R != 10 || got["led"].G != 20 || got["led"].B != 30 {
		t.Fatalf("led rgb = %d,%d,%d want 10,20,30", got["led"].R, got["led"].G, got["led"].B)
	}
	if got["led"].Pattern != "solid" {
		t.Fatalf("pattern = %q", got["led"].Pattern)
	}

	d.srv.setLed(ledOff)
	off := d.await([]string{"led"}, 5*time.Second)
	if off["led"].Pattern != "off" {
		t.Fatalf("the server must be able to turn it off at any time, got %q", off["led"].Pattern)
	}
}
