package server

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

	"cardputerme/internal/discovery"
	"cardputerme/internal/input"
	"cardputerme/internal/screen"
	"cardputerme/internal/terminal"

	"github.com/gorilla/websocket"
)

const (
	viewRows       = 6
	promptTailRows = 16
	maxLines       = 90
	historyMax     = 50
	noSession      = "Terminal is gone.\nRun cardputerme on\nthe computer to\nexpose it again."
	portStart      = 8001
	portTries      = 255
)

// Config is everything the CLI passes in; there are no package globals.
type Config struct {
	Name            string
	SessionCwd      string
	WrapCols        int
	LinesPerCard    int
	ScrollbackLines int
	MaxCards        int
	Notify          bool
}

type mirrorCache struct {
	grid     []screen.Line
	status   string
	awaiting bool
}

type stateResult struct {
	lines         []screen.Line
	status        string
	sessionExists bool
	awaiting      bool
}

// Server owns the one exposed terminal, its render state (behind mu), and the
// websocket hub. One Server per exposure.
type Server struct {
	cfg      Config
	backend  *terminal.Backend
	hub      *hub
	upgrader websocket.Upgrader

	mu           sync.Mutex
	input        string
	hist         int
	history      []string
	view         screen.View
	cache        *mirrorCache
	lastSig      string
	lastAwaiting bool

	pushMu    sync.Mutex
	pushTimer *time.Timer
}

func New(cfg Config) *Server {
	return &Server{
		cfg:      cfg,
		backend:  terminal.CreateBackend(cfg.Name, cfg.ScrollbackLines),
		hub:      newHub(),
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
		view:     screen.View{Follow: true, SelRow: -1},
		hist:     -1,
	}
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

func detectPromptAwaiting(pane string) bool {
	if pane == "" {
		return false
	}
	count := 0
	for _, raw := range strings.Split(screen.ToAscii(pane), "\n") {
		if len(screen.ParseChoices(strings.TrimRight(raw, " \t\r"))) > 0 {
			count++
		}
	}
	return count >= 2
}

func gridLines(rows []string) []screen.Line {
	out := []screen.Line{}
	for _, raw := range rows {
		text, color := screen.ParseLine(raw, screen.Colors.Text)
		out = append(out, screen.Line{Text: screen.ToAscii(text), Color: color})
	}
	if len(out) > maxLines {
		return out[len(out)-maxLines:]
	}
	return out
}

func splitScreen(pane string) ([]screen.Line, string) {
	rows := strings.Split(pane, "\n")
	last := -1
	for i := len(rows) - 1; i >= 0; i-- {
		if strings.TrimSpace(screen.StripAnsi(rows[i])) != "" {
			last = i
			break
		}
	}
	if last < 0 {
		return []screen.Line{}, ""
	}
	status := strings.TrimSpace(screen.ToAscii(screen.StripAnsi(rows[last])))
	for i := len(rows) - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(screen.StripAnsi(rows[i])), "esc to interrupt") {
			status = strings.TrimSpace(screen.ToAscii(screen.StripAnsi(rows[i])))
			break
		}
	}
	return gridLines(rows[:last]), status
}

func findSelectorRow(grid []screen.Line) int {
	for i := len(grid) - 1; i >= 0; i-- {
		t := strings.TrimSpace(grid[i].Text)
		if strings.HasPrefix(t, ">") && len(screen.ParseChoices(t[1:])) > 0 {
			return i
		}
	}
	return -1
}

func (s *Server) screenLines(text string) []screen.Line {
	out := []screen.Line{}
	for _, card := range screen.SliceIntoCards(text, s.cfg.WrapCols, s.cfg.LinesPerCard, s.cfg.MaxCards) {
		for _, str := range card {
			out = append(out, screen.Line{Text: str, Color: screen.LineColor(str, false)})
		}
	}
	if len(out) > maxLines {
		return out[len(out)-maxLines:]
	}
	return out
}

func (s *Server) composeMirror(grid []screen.Line, status string, awaiting bool) stateResult {
	maxLen := 0
	for _, l := range grid {
		if rl := len([]rune(l.Text)); rl > maxLen {
			maxLen = rl
		}
	}
	maxRow := max(0, len(grid)-viewRows)
	maxCol := max(0, maxLen-s.cfg.WrapCols)
	selRow := -1
	if awaiting {
		selRow = findSelectorRow(grid)
	}
	if selRow >= 0 && selRow != s.view.SelRow {
		s.view.Row = screen.AnchorRow(selRow, viewRows)
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
	lines := screen.WindowLines(grid, s.view, viewRows, s.cfg.WrapCols)

	if len(s.input) > 0 {
		composed := screen.WrapLine("> "+strings.ReplaceAll(s.input, "\n", " | "), s.cfg.WrapCols)
		from := max(0, len(composed)-2)
		for _, piece := range composed[from:] {
			lines = append(lines, screen.Line{Text: piece, Color: screen.Colors.Prompt})
		}
		if g := input.Suggest(s.input, s.history); g != "" {
			ghost := []rune(g)
			if len(ghost) > s.cfg.WrapCols {
				ghost = ghost[:s.cfg.WrapCols]
			}
			lines = append(lines, screen.Line{Text: string(ghost), Color: screen.Colors.Dim})
		}
	}
	hint := status
	if awaiting {
		hint = "PROMPT: press a number"
	}
	bar := fmt.Sprintf("%s  r%d/%d c%d", hint, s.view.Row, maxRow, s.view.Col)
	if len(lines) == 0 {
		lines = s.screenLines("(empty)")
	}
	return stateResult{lines: lines, status: "[" + s.cfg.Name + "] " + bar, sessionExists: true, awaiting: awaiting}
}

func (s *Server) buildState() stateResult {
	pane, ok := s.backend.Capture()
	if !ok {
		return stateResult{lines: s.screenLines(noSession), status: "terminal gone"}
	}
	tail := strings.Split(screen.StripAnsi(pane), "\n")
	if len(tail) > promptTailRows {
		tail = tail[len(tail)-promptTailRows:]
	}
	awaiting := detectPromptAwaiting(strings.Join(tail, "\n"))
	grid, status := splitScreen(pane)
	s.cache = &mirrorCache{grid: grid, status: status, awaiting: awaiting}
	return s.composeMirror(grid, status, awaiting)
}

func displayMessage(st stateResult) string {
	b, _ := json.Marshal(screen.BuildDisplay(st.lines, st.status))
	return string(b)
}

func sig(st stateResult) string {
	b, _ := json.Marshal(st.lines)
	return string(b) + st.status
}

func (s *Server) pushIfChanged(force bool) {
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
		s.hub.broadcast(msg)
	}
	if s.cfg.Notify && freshQuestion {
		s.hub.broadcast(`{"type":"notify","reason":"question"}`)
	}
}

func (s *Server) applyKey(key string) {
	s.mu.Lock()
	res := input.InterpretKey(input.State{Input: s.input, Hist: s.hist}, key, input.KeyCtx{Awaiting: s.lastAwaiting, History: s.history})
	s.input = res.State.Input
	s.hist = res.State.Hist
	a := res.Action
	switch a.Kind {
	case "pan":
		s.view = screen.PanViewport(s.view, a.Key)
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
		s.backend.SendText(a.Text)
	case "pressKey":
		s.backend.SendKey(a.Key)
	}
	if echo != "" {
		s.hub.broadcast(echo)
	}
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.hub.add(c)
	log.Printf("[ws] connect %s (clients=%d)", c.RemoteAddr(), s.hub.count())
	defer func() {
		s.hub.remove(c)
		log.Printf("[ws] close %s", c.RemoteAddr())
	}()

	s.mu.Lock()
	msg := displayMessage(s.buildState())
	s.mu.Unlock()
	s.hub.sendTo(c, msg)

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
			s.applyKey(m.Key)
		case "cmd":
			for _, ch := range m.Text {
				if ch == '\n' {
					s.applyKey("shift+enter")
					continue
				}
				s.applyKey(string(ch))
			}
			s.applyKey("enter")
		}
	}
}

func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	exists := s.backend.Exists()
	s.mu.Lock()
	awaiting := s.lastAwaiting
	s.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]any{
		"ok": true, "name": s.cfg.Name, "exists": exists, "notify": s.cfg.Notify, "awaiting": awaiting,
	})
}

func (s *Server) schedulePush() {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	if s.pushTimer != nil {
		return
	}
	s.pushTimer = time.AfterFunc(40*time.Millisecond, func() {
		s.pushMu.Lock()
		s.pushTimer = nil
		s.pushMu.Unlock()
		s.pushIfChanged(false)
	})
}

// broadcastTargets returns each up IPv4 interface's subnet-directed broadcast
// address plus the limited broadcast fallback (255.255.255.255 is often
// unroutable on macOS, so the directed address is what reaches the LAN).
func broadcastTargets() []*net.UDPAddr {
	targets := []*net.UDPAddr{{IP: net.IPv4bcast, Port: discovery.BeaconPort}}
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil {
				continue
			}
			b := make(net.IP, 4)
			for i := range 4 {
				b[i] = ip4[i] | ^ipn.Mask[i]
			}
			targets = append(targets, &net.UDPAddr{IP: b, Port: discovery.BeaconPort})
		}
	}
	return targets
}

func (s *Server) startBeacon(port int) {
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
	msg := []byte(discovery.BeaconMessage(s.cfg.Name, port))
	go func() {
		t := time.NewTicker(discovery.BeaconIntervalMs * time.Millisecond)
		for range t.C {
			for _, dst := range broadcastTargets() {
				pc.WriteTo(msg, dst)
			}
		}
	}()
	log.Printf("  beacon : udp :%d every %dms (subnet-directed + limited broadcast)", discovery.BeaconPort, discovery.BeaconIntervalMs)
}

// Run exposes the configured terminal: pick a port, ensure the session, start
// the beacon + event-driven push, and serve until the terminal dies.
func (s *Server) Run() error {
	port := discovery.PickPort(discovery.FreePort, portStart, portTries)
	if port == 0 {
		return fmt.Errorf("no free port between %d and %d", portStart, portStart+portTries-1)
	}
	s.backend.EnsureSession(s.cfg.SessionCwd)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ws", s.wsHandler)

	log.Printf("cardputerme — exposing '%s' on http://0.0.0.0:%d  (ws://…/ws)", s.cfg.Name, port)
	s.startBeacon(port)

	stop := s.backend.Subscribe(s.schedulePush, func() {
		log.Printf("[expose] terminal '%s' is gone — shutting down", s.cfg.Name)
		os.Exit(0)
	})
	defer stop()

	return http.ListenAndServe(":"+strconv.Itoa(port), mux)
}
