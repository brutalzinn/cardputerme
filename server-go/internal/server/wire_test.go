package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// End-to-end wire test: a real tmux session behind the real WS handler, driven
// by a gorilla client — the contract the firmware relies on. Skips without tmux.
func TestWireEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	sess := "wiretest-go"
	exec.Command("tmux", "kill-session", "-t", sess).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", sess, "-c", "/tmp").Run(); err != nil {
		t.Skipf("cannot create tmux session: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", sess).Run()

	s := New(Config{Name: sess, WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40, Notify: true})
	srv := httptest.NewServer(http.HandlerFunc(s.wsHandler))
	defer srv.Close()

	stop := s.sess.backend.Subscribe(s.schedulePush, func() {})
	defer stop()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if _, _, err := c.ReadMessage(); err != nil {
		t.Fatalf("initial read: %v", err)
	}
	if err := c.WriteJSON(map[string]string{"type": "cmd", "text": "echo wiretestmarker"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		c.SetReadDeadline(time.Now().Add(6 * time.Second))
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var m struct {
			Type string `json:"type"`
			Body []struct {
				Text string `json:"text"`
			} `json:"body"`
		}
		if json.Unmarshal(data, &m) != nil || m.Type != "display" {
			continue
		}
		for _, l := range m.Body {
			if strings.Contains(l.Text, "wiretestmarker") {
				return
			}
		}
	}
	t.Fatal("mirror never showed the executed command output")
}

// A prompt already on screen at connect time must set awaiting, so a digit
// answers the menu instead of landing in the input buffer.
func TestAwaitingSyncsOnPrompt(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	sess := "wiretest-ask"
	exec.Command("tmux", "kill-session", "-t", sess).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", sess, "-c", "/tmp").Run(); err != nil {
		t.Skipf("cannot create tmux session: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", sess).Run()
	// A blocking read keeps the menu on screen — a real interactive prompt.
	exec.Command("tmux", "send-keys", "-t", sess, "-l", "printf 'Deploy?\\n1. Yes\\n2. No\\n'; read x").Run()
	exec.Command("tmux", "send-keys", "-t", sess, "Enter").Run()
	time.Sleep(500 * time.Millisecond)

	s := New(Config{Name: sess, WrapCols: 20, LinesPerCard: 7, ScrollbackLines: 200, MaxCards: 40, Notify: true})
	s.pushIfChanged(true)

	s.mu.Lock()
	awaiting := s.sess.lastAwaiting
	s.mu.Unlock()
	if !awaiting {
		t.Fatal("awaiting should be true after a menu is on screen")
	}
}
