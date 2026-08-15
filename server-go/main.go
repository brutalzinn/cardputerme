package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const (
	viewRows       = 6
	promptTailRows = 16
	maxLines       = 90
	historyMax     = 50
	noSession      = "Terminal is gone.\nRun cardputerme on\nthe computer to\nexpose it again."
)

var (
	name            = "cardputerme"
	sessionCwd      = ""
	wrapCols        = 20
	linesPerCard    = 7
	scrollbackLines = 200
	maxCards        = 40
	notify          = true
	backend         *Backend
)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- shared state (one terminal per server), all mutations under mu ----------

type mirrorCache struct {
	grid     []Line
	status   string
	awaiting bool
}

type stateResult struct {
	lines         []Line
	status        string
	sessionExists bool
	awaiting      bool
}

type Session struct {
	mu           sync.Mutex
	input        string
	hist         int
	history      []string
	view         View
	cache        *mirrorCache
	lastSig      string
	lastAwaiting bool
}

var session = &Session{view: View{Follow: true, SelRow: -1}, hist: -1}

func detectPromptAwaiting(pane string) bool {
	if pane == "" {
		return false
	}
	count := 0
	for _, raw := range strings.Split(toAscii(pane), "\n") {
		if len(parseChoices(strings.TrimRight(raw, " \t\r"))) > 0 {
			count++
		}
	}
	return count >= 2
}

func gridLines(rows []string) []Line {
	out := []Line{}
	for _, raw := range rows {
		text, color := parseLine(raw, colors.Text)
		out = append(out, Line{Text: toAscii(text), Color: color})
	}
	if len(out) > maxLines {
		return out[len(out)-maxLines:]
	}
	return out
}

func splitScreen(pane string) ([]Line, string) {
	rows := strings.Split(pane, "\n")
	last := -1
	for i := len(rows) - 1; i >= 0; i-- {
		if strings.TrimSpace(stripAnsi(rows[i])) != "" {
			last = i
			break
		}
	}
	if last < 0 {
		return []Line{}, ""
	}
	status := strings.TrimSpace(toAscii(stripAnsi(rows[last])))
	for i := len(rows) - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(stripAnsi(rows[i])), "esc to interrupt") {
			status = strings.TrimSpace(toAscii(stripAnsi(rows[i])))
			break
		}
	}
	return gridLines(rows[:last]), status
}

func screenLines(text string) []Line {
	out := []Line{}
	for _, card := range sliceIntoCards(text, wrapCols, linesPerCard, maxCards) {
		for _, s := range card {
			out = append(out, Line{Text: s, Color: lineColor(s, false)})
		}
	}
	if len(out) > maxLines {
		return out[len(out)-maxLines:]
	}
	return out
}

func findSelectorRow(grid []Line) int {
	for i := len(grid) - 1; i >= 0; i-- {
		t := strings.TrimSpace(grid[i].Text)
		if strings.HasPrefix(t, ">") && len(parseChoices(t[1:])) > 0 {
			return i
		}
	}
	return -1
}

func (s *Session) composeMirror(grid []Line, status string, awaiting bool) stateResult {
	maxLen := 0
	for _, l := range grid {
		if rl := len([]rune(l.Text)); rl > maxLen {
			maxLen = rl
		}
	}
	maxRow := maxInt(0, len(grid)-viewRows)
	maxCol := maxInt(0, maxLen-wrapCols)
	selRow := -1
	if awaiting {
		selRow = findSelectorRow(grid)
	}
	if selRow >= 0 && selRow != s.view.SelRow {
		s.view.Row = anchorRow(selRow, viewRows)
		s.view.Follow = false
	}
	s.view.SelRow = selRow
	if selRow < 0 && s.view.Follow {
		s.view.Row = maxRow
	}
	s.view.Row = clamp(s.view.Row, 0, maxRow)
	s.view.Col = clamp(s.view.Col, 0, maxCol)
	if selRow < 0 && s.view.Row >= maxRow {
		s.view.Follow = true
	}
	lines := windowLines(grid, s.view, viewRows, wrapCols)

	if len(s.input) > 0 {
		composed := wrapLine("> "+strings.ReplaceAll(s.input, "\n", " | "), wrapCols)
		from := maxInt(0, len(composed)-2)
		for _, piece := range composed[from:] {
			lines = append(lines, Line{Text: piece, Color: colors.Prompt})
		}
	}
	hint := status
	if awaiting {
		hint = "PROMPT: press a number"
	}
	bar := fmt.Sprintf("%s  r%d/%d c%d", hint, s.view.Row, maxRow, s.view.Col)
	if len(lines) == 0 {
		lines = screenLines("(empty)")
	}
	return stateResult{lines: lines, status: "[" + name + "] " + bar, sessionExists: true, awaiting: awaiting}
}

func (s *Session) buildState() stateResult {
	pane, ok := backend.Capture()
	if !ok {
		return stateResult{lines: screenLines(noSession), status: "terminal gone"}
	}
	tail := strings.Split(stripAnsi(pane), "\n")
	if len(tail) > promptTailRows {
		tail = tail[len(tail)-promptTailRows:]
	}
	awaiting := detectPromptAwaiting(strings.Join(tail, "\n"))
	grid, status := splitScreen(pane)
	s.cache = &mirrorCache{grid: grid, status: status, awaiting: awaiting}
	return s.composeMirror(grid, status, awaiting)
}

func displayMessage(st stateResult) string {
	b, _ := json.Marshal(buildDisplay(st.lines, st.status))
	return string(b)
}

func sig(st stateResult) string {
	b, _ := json.Marshal(st.lines)
	return string(b) + st.status
}

func (s *Session) pushIfChanged(force bool) {
	s.mu.Lock()
	st := s.buildState()
	sg := sig(st)
	changed := sg != s.lastSig || force
	if changed {
		s.lastSig = sg
	}
	freshQuestion := st.awaiting && !s.lastAwaiting
	if !st.awaiting && s.lastAwaiting {
		s.view.Follow = true
	}
	s.lastAwaiting = st.awaiting
	msg := displayMessage(st)
	s.mu.Unlock()

	if changed {
		hub.broadcast(msg)
	}
	if notify && freshQuestion {
		hub.broadcast(`{"type":"notify","reason":"question"}`)
	}
}

func (s *Session) applyKey(key string) {
	s.mu.Lock()
	res := interpretKey(State{Input: s.input, Hist: s.hist}, key, KeyCtx{Awaiting: s.lastAwaiting, History: s.history})
	s.input = res.State.Input
	s.hist = res.State.Hist
	a := res.Action
	switch a.Kind {
	case "pan":
		s.view = panViewport(s.view, a.Key)
	case "send":
		s.history = append(s.history, a.Text)
		if len(s.history) > historyMax {
			s.history = s.history[1:]
		}
		s.view.Follow = true
	case "pressKey":
		s.view.Follow = true
	}
	var echo string
	if s.cache != nil {
		echo = displayMessage(s.composeMirror(s.cache.grid, s.cache.status, s.cache.awaiting))
	}
	s.mu.Unlock()

	switch a.Kind {
	case "send":
		backend.SendText(a.Text)
	case "pressKey":
		backend.SendKey(a.Key)
	}
	if echo != "" {
		hub.broadcast(echo)
	}
}

// ---- websocket hub (all writes serialized here) ------------------------------

type Hub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]bool
}

var hub = &Hub{conns: map[*websocket.Conn]bool{}}

func (h *Hub) add(c *websocket.Conn) {
	h.mu.Lock()
	h.conns[c] = true
	h.mu.Unlock()
}

func (h *Hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns, c)
	h.mu.Unlock()
	c.Close()
}

func (h *Hub) broadcast(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			delete(h.conns, c)
			c.Close()
		}
	}
}

func (h *Hub) sendTo(c *websocket.Conn, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.WriteMessage(websocket.TextMessage, []byte(msg))
}

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	hub.add(c)
	defer hub.remove(c)

	session.mu.Lock()
	st := session.buildState()
	msg := displayMessage(st)
	session.mu.Unlock()
	hub.sendTo(c, msg)

	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		var m struct {
			Type string `json:"type"`
			Key  string `json:"key"`
			Text string `json:"text"`
		}
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "key":
			session.applyKey(m.Key)
		case "cmd":
			for _, ch := range m.Text {
				if ch == '\n' {
					session.applyKey("shift+enter")
					continue
				}
				session.applyKey(string(ch))
			}
			session.applyKey("enter")
		}
	}
}

// ---- HTTP debug routes -------------------------------------------------------

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	exists := backend.Exists()
	session.mu.Lock()
	awaiting := session.lastAwaiting
	session.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true, "name": name, "exists": exists, "notify": notify, "awaiting": awaiting,
	})
}

// ---- UDP beacon (periodic announce is not polling) ---------------------------

func startBeacon(port int) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		c.Control(func(fd uintptr) {
			serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
		})
		return serr
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		log.Printf("beacon disabled: %v", err)
		return
	}
	dst, _ := net.ResolveUDPAddr("udp4", beaconAddr+":"+strconv.Itoa(beaconPort))
	msg := []byte(beaconMessage(name, port))
	go func() {
		t := time.NewTicker(beaconIntervalMs * time.Millisecond)
		for range t.C {
			pc.WriteTo(msg, dst)
		}
	}()
	log.Printf("  beacon : udp %s:%d every %dms", beaconAddr, beaconPort, beaconIntervalMs)
}

func main() {
	log.SetFlags(0)
	if v := os.Getenv("SESSION"); v != "" {
		name = v
	}
	sessionCwd = os.Getenv("SESSION_CWD")
	wrapCols = envInt("WRAP_COLS", wrapCols)
	linesPerCard = envInt("LINES_PER_CARD", linesPerCard)
	scrollbackLines = envInt("SCROLLBACK_LINES", scrollbackLines)
	maxCards = envInt("MAX_CARDS", maxCards)
	notify = os.Getenv("NOTIFY") != "0"

	backend = createBackend(name, scrollbackLines)

	port := pickPort(freePort, 8001, 255)
	if port == 0 {
		log.Fatal("cardputerme — no free port between 8001 and 8255")
	}

	backend.EnsureSession(sessionCwd)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ws", wsHandler)
	srv := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: mux}

	log.Printf("cardputerme — exposing '%s' on http://0.0.0.0:%d  (ws://…/ws)", name, port)
	startBeacon(port)

	stop := backend.Subscribe(func() { schedulePush() }, func() {
		log.Printf("[expose] terminal '%s' is gone — shutting down", name)
		os.Exit(0)
	})
	defer stop()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// ---- event-driven push debounce (coalesce a burst into one capture) ----------

var (
	pushMu    sync.Mutex
	pushTimer *time.Timer
)

func schedulePush() {
	pushMu.Lock()
	defer pushMu.Unlock()
	if pushTimer != nil {
		return
	}
	pushTimer = time.AfterFunc(40*time.Millisecond, func() {
		pushMu.Lock()
		pushTimer = nil
		pushMu.Unlock()
		session.pushIfChanged(false)
	})
}
