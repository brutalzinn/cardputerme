package server

import (
	"sync"

	"github.com/gorilla/websocket"
)

// hub tracks connected devices and serializes all writes (gorilla forbids
// concurrent writes to one conn).
type hub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]bool
}

func newHub() *hub {
	return &hub{conns: map[*websocket.Conn]bool{}}
}

func (h *hub) add(c *websocket.Conn) {
	h.mu.Lock()
	h.conns[c] = true
	h.mu.Unlock()
}

func (h *hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns, c)
	h.mu.Unlock()
	c.Close()
}

func (h *hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns)
}

func (h *hub) broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			delete(h.conns, c)
			c.Close()
		}
	}
}

func (h *hub) sendTo(c *websocket.Conn, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.WriteMessage(websocket.TextMessage, []byte(msg))
}
