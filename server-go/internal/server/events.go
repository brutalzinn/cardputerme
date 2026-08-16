package server

const (
	evChanged = "changed"
	evGone    = "gone"
)

const eventQueue = 64

// sessionEvent is what a terminal's reader goroutine reports. With one server
// owning many sessions, N reader goroutines would otherwise all reach into
// shared state; sending here lets ONE dispatcher decide, which is the Go idiom
// — channels coordinate goroutines, the mutex only guards data.
type sessionEvent struct {
	name string
	kind string
}

// emit never blocks a reader goroutine for long. `changed` coalesces: a push is
// already pending, so a dropped one changes nothing. `gone` must survive, or the
// session leaks forever — it waits, but gives up once the server is shutting down.
func (s *Server) emit(ev sessionEvent) {
	if ev.kind == evChanged {
		select {
		case s.events <- ev:
		default:
		}
		return
	}
	select {
	case s.events <- ev:
	case <-s.done:
	}
}

func (s *Server) dispatch() {
	for {
		select {
		case ev := <-s.events:
			s.handleEvent(ev)
		case <-s.done:
			return
		}
	}
}

func (s *Server) handleEvent(ev sessionEvent) {
	if ev.kind == evGone {
		s.terminalGone(ev.name)
		return
	}
	s.sessionChanged(ev.name)
}
