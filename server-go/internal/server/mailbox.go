package server

import "sync"

const maxPendingEvents = 32

type mailbox struct {
	mu     sync.Mutex
	frame  string
	events []string
}

func (m *mailbox) putFrame(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frame = msg
}

func (m *mailbox) putEvent(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, msg)
	if len(m.events) > maxPendingEvents {
		m.events = m.events[len(m.events)-maxPendingEvents:]
	}
}

func (m *mailbox) take() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.events
	m.events = nil
	if m.frame != "" {
		out = append(out, m.frame)
		m.frame = ""
	}
	return out
}
