package server

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const writeWait = 5 * time.Second

// client owns the only goroutine allowed to write to its conn (gorilla forbids
// concurrent writes). Display frames coalesce in its mailbox, so a device that
// falls behind renders the newest screen instead of a backlog.
type client struct {
	conn *websocket.Conn
	box  mailbox
	wake chan struct{}
	done chan struct{}
	once sync.Once
}

func newClient(c *websocket.Conn) *client {
	return &client{conn: c, wake: make(chan struct{}, 1), done: make(chan struct{})}
}

func (c *client) nudge() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *client) sendFrame(msg string) {
	c.box.putFrame(msg)
	c.nudge()
}

func (c *client) sendEvent(msg string) {
	c.box.putEvent(msg)
	c.nudge()
}

func (c *client) stop() {
	c.once.Do(func() { close(c.done) })
}

func (c *client) writeLoop(gone func()) {
	defer gone()
	for {
		select {
		case <-c.done:
			return
		case <-c.wake:
			for _, msg := range c.box.take() {
				c.conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					return
				}
			}
		}
	}
}

// hub tracks connected devices. It never writes to a conn itself — it hands
// messages to each client's writer, so a slow device cannot stall a push.
type hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]*client
}

func newHub() *hub {
	return &hub{clients: map[*websocket.Conn]*client{}}
}

func (h *hub) add(c *websocket.Conn) {
	cl := newClient(c)
	h.mu.Lock()
	h.clients[c] = cl
	h.mu.Unlock()
	go cl.writeLoop(func() { h.remove(c) })
}

func (h *hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	cl, ok := h.clients[c]
	delete(h.clients, c)
	h.mu.Unlock()
	if ok {
		cl.stop()
	}
	c.Close()
}

func (h *hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *hub) each(fn func(*client)) {
	h.mu.Lock()
	targets := make([]*client, 0, len(h.clients))
	for _, cl := range h.clients {
		targets = append(targets, cl)
	}
	h.mu.Unlock()
	for _, cl := range targets {
		fn(cl)
	}
}

func (h *hub) broadcastFrame(msg string) {
	h.each(func(cl *client) { cl.sendFrame(msg) })
}

func (h *hub) broadcast(msg string) {
	h.each(func(cl *client) { cl.sendEvent(msg) })
}
