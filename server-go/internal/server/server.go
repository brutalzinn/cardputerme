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
	"cardputerme/internal/power"
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
	DimAfter        time.Duration
	OffAfter        time.Duration
}

type mirrorCache struct {
	grid     []screen.Line
	status   string
	awaiting bool
}

type stateResult struct {
	lines         []screen.Line
	status        string
	size          int
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
	size         int
	cache        *mirrorCache
	lastSig      string
	lastAwaiting bool

	pushMu    sync.Mutex
	pushTimer *time.Timer

	power      *power.Tracker
	powerMu    sync.Mutex
	powerTimer *time.Timer
}

const (
	baseSize = 2 // WrapCols/viewRows are calibrated at text size 2
	sizeMin  = 1
	sizeMax  = 3
)

func New(cfg Config) *Server {
	return &Server{
		cfg:      cfg,
		backend:  terminal.CreateBackend(cfg.Name, cfg.ScrollbackLines),
		hub:      newHub(),
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
		view:     screen.View{Follow: true, SelRow: -1},
		hist:     -1,
		size:     baseSize,
		power:    power.NewTracker(power.Policy{DimAfter: cfg.DimAfter, OffAfter: cfg.OffAfter}, time.Now()),
	}
}

// cols/rows scale inversely with text size — bigger text, fewer fit on the
// 240x135 screen. Calibrated so size 2 == the configured WrapCols x viewRows.
func (s *Server) cols() int { return max(1, s.cfg.WrapCols*baseSize/s.size) }
func (s *Server) rows() int { return max(1, viewRows*baseSize/s.size) }

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
	cols, rows := s.cols(), s.rows()
	maxRow := max(0, len(grid)-rows)
	maxCol := max(0, maxLen-cols)
	selRow := -1
	if awaiting {
		selRow = findSelectorRow(grid)
	}
	if selRow >= 0 && selRow != s.view.SelRow {
		s.view.Row = screen.AnchorRow(selRow, rows)
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
	lines := screen.WindowLines(grid, s.view, rows, cols)

	if len(s.input) > 0 {
		composed := screen.WrapLine("> "+strings.ReplaceAll(s.input, "\n", " | "), cols)
		from := max(0, len(composed)-2)
		for _, piece := range composed[from:] {
			lines = append(lines, screen.Line{Text: piece, Color: screen.Colors.Prompt})
		}
		if g := input.Suggest(s.input, s.history); g != "" {
			ghost := []rune(g)
			if len(ghost) > cols {
				ghost = ghost[:cols]
			}
			lines = append(lines, screen.Line{Text: string(ghost), Color: screen.Colors.Dim})
		}
	}
	hint := status
	if awaiting {
		hint = "PROMPT: press a number"
	}
	bar := fmt.Sprintf("%s  r%d/%d c%d z%d", hint, s.view.Row, maxRow, s.view.Col, s.size)
	if len(lines) == 0 {
		lines = s.screenLines("(empty)")
	}
	return stateResult{lines: lines, status: "[" + s.cfg.Name + "] " + bar, size: s.size, sessionExists: true, awaiting: awaiting}
}

// stateFrom renders a captured pane into the device state. The caller holds mu;
// the tmux Capture() itself is done OUTSIDE the lock (it's a read-only subprocess)
// so a keystroke echo never waits on a capture in flight.
func (s *Server) stateFrom(pane string, ok bool) stateResult {
	if !ok {
		return stateResult{lines: s.screenLines(noSession), status: "terminal gone", size: s.size}
	}
	grid, status := splitScreen(pane)
	// Detect a prompt over the SAME blank-trimmed content the device shows (the
	// grid + status row), not the raw tail — a menu with blank lines below it
	// would otherwise fall outside the last-N raw rows and be missed.
	awaiting := detectPromptAwaiting(gridTail(grid, status))
	s.cache = &mirrorCache{grid: grid, status: status, awaiting: awaiting}
	return s.composeMirror(grid, status, awaiting)
}

func gridTail(grid []screen.Line, status string) string {
	from := max(0, len(grid)-promptTailRows)
	var sb strings.Builder
	for _, l := range grid[from:] {
		sb.WriteString(l.Text)
		sb.WriteByte('\n')
	}
	sb.WriteString(status)
	return sb.String()
}

func displayMessage(st stateResult) string {
	b, _ := json.Marshal(screen.BuildDisplay(st.lines, st.status, st.size))
	return string(b)
}

func sig(st stateResult) string {
	b, _ := json.Marshal(st.lines)
	return string(b) + st.status
}

func (s *Server) pushIfChanged(force bool) {
	pane, ok := s.backend.Capture() // read-only subprocess — kept OUT of the lock
	s.mu.Lock()
	st := s.stateFrom(pane, ok)
	sg := sig(st)
	changed := sg != s.lastSig || force
	if changed {
		s.lastSig = sg
	}
	freshQuestion := st.awaiting && !s.lastAwaiting
	awaitingChanged := st.awaiting != s.lastAwaiting
	if !st.awaiting && s.lastAwaiting {
		s.view.Follow = true
	}
	s.lastAwaiting = st.awaiting
	msg := displayMessage(st)
	s.mu.Unlock()

	if awaitingChanged {
		s.applyPower(s.power.SetInhibit(time.Now(), st.awaiting))
	}
	if changed {
		s.hub.broadcast(msg)
	}
	if s.cfg.Notify && freshQuestion {
		s.hub.broadcast(`{"type":"notify","reason":"question"}`)
	}
}

func powerMessage(st power.State) string {
	return `{"type":"power","state":"` + string(st) + `"}`
}

func (s *Server) applyPower(st power.State, changed bool) {
	if changed {
		s.hub.broadcast(powerMessage(st))
	}
	s.armPowerTimer()
}

func (s *Server) armPowerTimer() {
	d := s.power.Until(time.Now())
	s.powerMu.Lock()
	defer s.powerMu.Unlock()
	if s.powerTimer != nil {
		s.powerTimer.Stop()
		s.powerTimer = nil
	}
	if d <= 0 {
		return
	}
	s.powerTimer = time.AfterFunc(d, func() {
		s.applyPower(s.power.At(time.Now()))
	})
}

func (s *Server) applyKey(key string) {
	s.applyPower(s.power.Wake(time.Now()))
	s.mu.Lock()
	res := input.InterpretKey(input.State{Input: s.input, Hist: s.hist}, key, input.KeyCtx{Awaiting: s.lastAwaiting, History: s.history})
	s.input = res.State.Input
	s.hist = res.State.Hist
	a := res.Action
	switch a.Kind {
	case "pan":
		s.view = screen.PanViewport(s.view, a.Key)
	case "zoom":
		switch a.Key {
		case "in":
			s.size = min(sizeMax, s.size+1)
		case "out":
			s.size = max(sizeMin, s.size-1)
		case "reset":
			s.size = baseSize
		}
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

	// Force an initial push so the newcomer gets the screen AND lastAwaiting is
	// synced — otherwise a digit answering a prompt that was already on screen at
	// connect time would wrongly land in the input buffer.
	s.pushIfChanged(true)
	s.hub.broadcast(powerMessage(s.power.State()))

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
		case "wake":
			s.applyPower(s.power.Wake(time.Now()))
		case "sleep":
			s.applyPower(s.power.Sleep(time.Now()))
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
	log.Printf("  screen : dim after %v, off after %v (0 = never)", s.cfg.DimAfter, s.cfg.OffAfter)
	s.startBeacon(port)
	s.armPowerTimer()

	stop := s.backend.Subscribe(s.schedulePush, func() {
		log.Printf("[expose] terminal '%s' is gone — shutting down", s.cfg.Name)
		os.Exit(0)
	})
	defer stop()

	return http.ListenAndServe(":"+strconv.Itoa(port), mux)
}
