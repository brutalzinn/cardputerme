package main

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
// by a gorilla client — the same contract the firmware relies on. Skips when
// tmux is unavailable so `go test` stays portable.
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

	name = sess
	backend = createBackend(sess, 200)
	session = &Session{view: View{Follow: true, SelRow: -1}, hist: -1}
	hub = &Hub{conns: map[*websocket.Conn]bool{}}

	srv := httptest.NewServer(http.HandlerFunc(wsHandler))
	defer srv.Close()

	stop := backend.Subscribe(func() { schedulePush() }, func() {})
	defer stop()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	// initial frame
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
