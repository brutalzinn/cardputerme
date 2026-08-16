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

	"cardputerme/internal/battery"
	"cardputerme/internal/commands"
	"cardputerme/internal/discovery"
	"cardputerme/internal/idle"
	"cardputerme/internal/input"
	"cardputerme/internal/power"
	"cardputerme/internal/repeat"
	"cardputerme/internal/screen"
	"cardputerme/internal/settings"
	"cardputerme/internal/terminal"

	"github.com/gorilla/websocket"
)

const (
	viewRows            = 6
	promptTailRows      = 16
	maxLines            = 90
	historyMax          = 50
	noSession           = "Terminal is gone.\nRun cardputerme on\nthe computer to\nexpose it again."
	portStart           = 8001
	portTries           = 255
	repeatMaxHold       = 10 * time.Second
	maxFramedRows       = 4
	defaultPushDebounce = 15 * time.Millisecond
)

// Config is everything the CLI passes in; there are no package globals.
type Config struct {
	Name            string
	Session         string
	SessionCwd      string
	WrapCols        int
	LinesPerCard    int
	ScrollbackLines int
	MaxCards        int
	Notify          bool
	DimAfter        time.Duration
	OffAfter        time.Duration
	RepeatDelay     time.Duration
	RepeatInterval  time.Duration
	PushDebounce    time.Duration
	UsbMilliVolts   int
	IdleExit        time.Duration
	SoundsDir       string
	SettingsPath    string
	NotifySound     string
	HidePrefixes    []string
}

type mirrorCache struct {
	grid     []screen.Line
	status   string
	title    string
	awaiting bool
}

type stateResult struct {
	lines         []screen.Line
	status        string
	size          int
	sessionExists bool
	awaiting      bool
}

// Server owns the mirrored session(s), the device-facing state, and the
// websocket hub. Everything under mu; per-terminal state lives in session.
type Server struct {
	cfg      Config
	hub      *hub
	upgrader websocket.Upgrader

	mu       sync.Mutex
	sess     *session
	sessions map[string]*session
	order    []string

	size      int
	notify    bool
	soundBase string
	lastNoSig string
	lastLed   string
	beacons   []beacon
	picking   bool
	pick      int

	events    chan sessionEvent
	done      chan struct{}
	closeOnce sync.Once

	pushMu    sync.Mutex
	pushTimer *time.Timer

	power         *power.Tracker
	powerDeadline deadline

	gauge *battery.Gauge

	idle         *idle.Tracker
	idleDeadline deadline

	repeat         *repeat.Holder
	repeatDeadline deadline

	// onNotify lets a test observe which session raised the alert.
	onNotify func(session string)
}

const (
	baseSize = 2 // WrapCols/viewRows are calibrated at text size 2
	sizeMin  = 1
	sizeMax  = 3
)

var batteryPolicy = battery.Policy{
	MinMv:       3300,
	MaxMv:       4100,
	Window:      8,
	Deadband:    3,
	SettleAfter: 2 * time.Second,
	RiseRate:    0.5,
	RisePeriod:  30 * time.Second,
}

func sessionTarget(cfg Config) string {
	if cfg.Session != "" {
		return cfg.Session
	}
	return cfg.Name
}

// startingNotify resolves env-vs-file precedence: an explicitly set env var is a
// deliberate one-off override for THIS process, otherwise the persisted choice
// wins. Recorded here because it will otherwise be forgotten.
//
// An empty SettingsPath means "do not touch disk at all" — tests must never
// read or write the developer's real ~/.cardputerme/settings.json, which they
// did until this was made explicit.
func startingNotify(cfg Config) bool {
	if cfg.SettingsPath == "" {
		return cfg.Notify
	}
	if _, explicit := os.LookupEnv("NOTIFY"); explicit {
		return cfg.Notify
	}
	return settings.Load(cfg.SettingsPath).Notify
}

func New(cfg Config) *Server {
	return &Server{
		cfg:      cfg,
		sessions: map[string]*session{},
		events:   make(chan sessionEvent, eventQueue),
		done:     make(chan struct{}),
		hub:      newHub(),
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
		size:     baseSize,
		notify:   startingNotify(cfg),
		power:    power.NewTracker(power.Policy{DimAfter: cfg.DimAfter, OffAfter: cfg.OffAfter}, time.Now()),
		repeat:   repeat.NewHolder(repeat.Policy{Delay: cfg.RepeatDelay, Interval: cfg.RepeatInterval, MaxHold: repeatMaxHold}),
		gauge:    battery.NewGauge(batteryPolicy),
		idle:     idle.NewTracker(idle.Policy{After: cfg.IdleExit}, time.Now()),
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

// framedRows marks rows enclosed by a nearby pair of rule rows. Terminal UIs
// draw their input widget in such a box, and the server renders its own input
// line, so mirroring the widget only costs rows on a six-row screen. The gap
// bound keeps an unpaired rule from swallowing the transcript behind it.
func framedRows(rows []string) map[int]bool {
	hidden := map[int]bool{}
	prev := -1
	for i, raw := range rows {
		if _, isRule := screen.RuleTitle(raw); !isRule {
			continue
		}
		if prev >= 0 && i-prev <= maxFramedRows+1 {
			for j := prev + 1; j < i; j++ {
				hidden[j] = true
			}
		}
		prev = i
	}
	return hidden
}

func splitScreen(pane string, hide []string) ([]screen.Line, string, string) {
	rows := strings.Split(pane, "\n")
	last := -1
	for i := len(rows) - 1; i >= 0; i-- {
		if strings.TrimSpace(screen.StripAnsi(rows[i])) != "" {
			last = i
			break
		}
	}
	if last < 0 {
		return []screen.Line{}, "", ""
	}
	status := strings.TrimSpace(screen.ToAscii(screen.StripAnsi(rows[last])))
	for i := len(rows) - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(screen.StripAnsi(rows[i])), "esc to interrupt") {
			status = strings.TrimSpace(screen.ToAscii(screen.StripAnsi(rows[i])))
			break
		}
	}
	kept := []string{}
	title := ""
	hidden := framedRows(rows[:last])
	for i, raw := range rows[:last] {
		if hidden[i] || screen.StartsWithAny(raw, hide) {
			continue
		}
		label, isRule := screen.RuleTitle(raw)
		if isRule {
			if label != "" {
				title = label
			}
			continue
		}
		kept = append(kept, raw)
	}
	return gridLines(kept), status, title
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

func (s *Server) composeMirror(grid []screen.Line, status, title string, awaiting bool) stateResult {
	if s.picking {
		cols, rows := s.cols(), s.rows()
		items := s.itemsLocked()
		s.pick = clamp(s.pick, 0, max(0, len(items)-1))
		return stateResult{
			lines:         itemLines(items, s.pick, rows, cols),
			status:        itemStatus(items, s.pick),
			size:          s.size,
			sessionExists: true,
		}
	}
	maxLen := 0
	for _, l := range grid {
		if rl := len([]rune(l.Text)); rl > maxLen {
			maxLen = rl
		}
	}
	cols, rows := s.cols(), s.rows()

	// Build the trailing lines FIRST and reserve room for them. Filling the
	// grid to `rows` and appending afterwards pushed the input line onto a page
	// the user never sees — visible as "I can type but cannot read it" once
	// zoomed in, where there is no slack.
	tail := []screen.Line{}
	add := func(text string, color uint16) {
		for _, line := range strings.Split(text, "\n") {
			for _, piece := range screen.WrapLine(line, cols) {
				tail = append(tail, screen.Line{Text: piece, Color: color})
			}
		}
	}
	if s.sess.reply != "" {
		add(s.sess.reply, screen.Colors.Status)
	}
	if s.sess.cmd != "" {
		add(s.sess.cmd, screen.Colors.Prompt)
	}
	if len(s.sess.input) > 0 {
		composed := screen.WrapLine("> "+strings.ReplaceAll(s.sess.input, "\n", " | "), cols)
		from := max(0, len(composed)-2)
		for _, piece := range composed[from:] {
			tail = append(tail, screen.Line{Text: piece, Color: screen.Colors.Prompt})
		}
		if g := input.Suggest(s.sess.input, s.sess.history); g != "" {
			ghost := []rune(g)
			if len(ghost) > cols {
				ghost = ghost[:cols]
			}
			tail = append(tail, screen.Line{Text: string(ghost), Color: screen.Colors.Dim})
		}
	}
	if len(tail) > rows {
		tail = tail[len(tail)-rows:]
	}
	gridRows := max(1, rows-len(tail))

	maxRow := max(0, len(grid)-gridRows)
	maxCol := max(0, maxLen-cols)
	selRow := -1
	if awaiting {
		selRow = findSelectorRow(grid)
	}
	if selRow >= 0 && selRow != s.sess.view.SelRow {
		s.sess.view.Row = screen.AnchorRow(selRow, gridRows)
		s.sess.view.Follow = false
	}
	s.sess.view.SelRow = selRow
	if selRow < 0 && s.sess.view.Follow {
		s.sess.view.Row = maxRow
	}
	s.sess.view.Row = clamp(s.sess.view.Row, 0, maxRow)
	s.sess.view.Col = clamp(s.sess.view.Col, 0, maxCol)
	if selRow < 0 && s.sess.view.Row >= maxRow {
		s.sess.view.Follow = true
	}
	lines := screen.WindowLines(grid, s.sess.view, gridRows, cols)
	lines = append(lines, tail...)

	hint := status
	if awaiting {
		hint = "PROMPT: press a number"
	}
	bat := battery.Label(s.gauge.Status())
	segs := []string{"[" + s.cfg.Name + "]"}
	for _, seg := range []string{title, hint, bat} {
		if seg != "" {
			segs = append(segs, seg)
		}
	}
	segs = append(segs, fmt.Sprintf("r%d/%d c%d z%d", s.sess.view.Row, maxRow, s.sess.view.Col, s.size))
	bar := strings.Join(segs, "  ")
	if len(lines) == 0 {
		lines = s.screenLines("(empty)")
	}
	return stateResult{lines: lines, status: bar, size: s.size, sessionExists: true, awaiting: awaiting}
}

// stateFrom renders a captured pane into the device state. The caller holds mu;
// the tmux Capture() itself is done OUTSIDE the lock (it's a read-only subprocess)
// so a keystroke echo never waits on a capture in flight.
func (s *Server) stateFrom(pane string, ok bool) stateResult {
	if !ok {
		return stateResult{lines: s.screenLines(noSession), status: "terminal gone", size: s.size}
	}
	grid, status, title := splitScreen(pane, s.cfg.HidePrefixes)
	// Detect a prompt over the SAME blank-trimmed content the device shows (the
	// grid + status row), not the raw tail — a menu with blank lines below it
	// would otherwise fall outside the last-N raw rows and be missed.
	awaiting := detectPromptAwaiting(gridTail(grid, status))
	s.sess.cache = &mirrorCache{grid: grid, status: status, title: title, awaiting: awaiting}
	return s.composeMirror(grid, status, title, awaiting)
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
	s.mu.Lock()
	sess := s.sess
	s.mu.Unlock()
	if sess == nil {
		s.pushNoSessions(force)
		return
	}
	pane, ok := sess.backend.Capture() // read-only subprocess — kept OUT of the lock
	s.mu.Lock()
	if s.sess != sess { // switched while capturing — that session will push its own frame
		s.mu.Unlock()
		return
	}
	st := s.stateFrom(pane, ok)
	sg := sig(st)
	changed := sg != s.sess.lastSig || force
	if changed {
		s.sess.lastSig = sg
	}
	freshQuestion := st.awaiting && !s.sess.lastAwaiting
	awaitingChanged := st.awaiting != s.sess.lastAwaiting
	if !st.awaiting && s.sess.lastAwaiting {
		s.sess.view.Follow = true
	}
	s.sess.lastAwaiting = st.awaiting
	msg := displayMessage(st)
	s.mu.Unlock()

	if awaitingChanged {
		s.applyPower(s.power.SetInhibit(time.Now(), st.awaiting))
	}
	if changed {
		s.hub.broadcastFrame(msg)
	}
	if s.notify && freshQuestion {
		s.hub.broadcast(`{"type":"notify","reason":"question"}`)
		s.signalAttention()
	}
}

func powerMessage(st power.State) string {
	return `{"type":"power","state":"` + string(st) + `"}`
}

func (s *Server) applyPower(st power.State, changed bool) {
	if changed {
		s.gauge.Disturb(time.Now())
		s.hub.broadcast(powerMessage(st))
	}
	s.armPowerTimer()
}

func (s *Server) armIdleTimer() {
	now := time.Now()
	if s.idle.Expired(now) {
		log.Printf("[expose] no device has connected to '%s' for %v — shutting down (IDLE_EXIT_H=0 disables this)", s.cfg.Name, s.cfg.IdleExit)
		s.shutdown()
		return
	}
	s.idleDeadline.arm(s.idle.Until(now), s.armIdleTimer)
}

func (s *Server) armPowerTimer() {
	s.powerDeadline.arm(s.power.Until(time.Now()), func() {
		s.applyPower(s.power.At(time.Now()))
	})
}

func (s *Server) applyKey(key string) input.Action {
	return s.applyKeyTimes(key, 1)
}

// applyKeyNoSession keeps the device usable on a machine with nothing
// registered: the picker must still open, or the user is stranded with no way
// to reach another machine.
func (s *Server) applyKeyNoSession(key string) input.Action {
	s.mu.Lock()
	items := s.itemsLocked()
	res := input.InterpretKey(
		input.State{Picking: s.picking, Pick: s.pick},
		key,
		input.KeyCtx{Beacons: len(items)},
	)
	s.picking = res.State.Picking
	s.pick = res.State.Pick
	connect, switchTo := s.resolvePick(res.Action, items)
	s.mu.Unlock()

	if connect != "" {
		log.Printf("[picker] %s", connect)
		s.hub.broadcast(connect)
	}
	if switchTo != "" {
		s.switchTo(switchTo)
	}
	s.pushIfChanged(true)
	return res.Action
}

func (s *Server) applyKeyTimes(key string, times int) input.Action {
	s.applyPower(s.power.Wake(time.Now()))
	s.setLed(ledOff)
	if s.currentName() == "" {
		return s.applyKeyNoSession(key)
	}
	s.mu.Lock()
	items := s.itemsLocked()
	sess := s.sess
	sess.reply = ""
	res := input.InterpretKey(
		input.State{Input: s.sess.input, Hist: s.sess.hist, Cmd: s.sess.cmd, Picking: s.picking, Pick: s.pick},
		key,
		input.KeyCtx{Awaiting: s.sess.lastAwaiting, History: s.sess.history, Beacons: len(items)},
	)
	s.sess.input = res.State.Input
	s.sess.cmd = res.State.Cmd
	s.sess.hist = res.State.Hist
	s.picking = res.State.Picking
	s.pick = res.State.Pick
	a := res.Action
	connect, switchTo := s.resolvePick(a, items)
	switch a.Kind {
	case "pan":
		for i := 0; i < times; i++ {
			s.sess.view = screen.PanViewport(s.sess.view, a.Key)
		}
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
		s.sess.history = append(s.sess.history, a.Text)
		if len(s.sess.history) > historyMax {
			s.sess.history = s.sess.history[1:]
		}
		s.sess.view.Follow = true
	case "pressKey":
		s.sess.view.Follow = true
	}
	var echo string
	if s.sess.cache != nil {
		st := s.composeMirror(s.sess.cache.grid, s.sess.cache.status, s.sess.cache.title, s.sess.cache.awaiting)
		if sg := sig(st); sg != s.sess.lastSig {
			s.sess.lastSig = sg
			echo = displayMessage(st)
		}
	}
	s.mu.Unlock()

	if connect != "" {
		log.Printf("[picker] %s", connect)
		s.hub.broadcast(connect)
	}
	if switchTo != "" {
		s.switchTo(switchTo)
	}
	// Commands run with NO lock held: a command may call back into the server
	// (`;notify` flips live state), and a Go mutex is not reentrant — running
	// this inside the lock deadlocked the whole server and froze the device.
	if a.Kind == "command" {
		reply := commands.Run(commands.Ctx{Name: s.cfg.Name, Notify: s}, a.Text)
		s.mu.Lock()
		sess.reply = reply
		if sess.cache != nil {
			st := s.composeMirror(sess.cache.grid, sess.cache.status, sess.cache.title, sess.cache.awaiting)
			if sg := sig(st); sg != sess.lastSig {
				sess.lastSig = sg
				echo = displayMessage(st)
			}
		}
		s.mu.Unlock()
	}

	switch a.Kind {
	case "send":
		sess.backend.SendText(a.Text)
	case "pressKey":
		sess.backend.SendKeyTimes(a.Key, times)
	}
	if echo != "" {
		s.hub.broadcastFrame(echo)
	}
	return a
}

func onExternalPower(usb bool, milliVolts, threshold int) bool {
	if usb {
		return true
	}
	return threshold > 0 && milliVolts >= threshold
}

func (s *Server) handleReport(usb bool, milliVolts int, now time.Time) {
	wired := onExternalPower(usb, milliVolts, s.cfg.UsbMilliVolts)
	s.gauge.Observe(battery.Reading{Millivolts: milliVolts, External: wired, At: now})
	pct, known, external := s.gauge.Status()
	log.Printf("[power] device reports usb=%v mv=%d (threshold %d) — external=%v battery=%q", usb, milliVolts, s.cfg.UsbMilliVolts, external, battery.Label(pct, known, external))
	s.applyPower(s.power.SetExternalPower(now, external))
	s.schedulePush()
}

func (s *Server) handleKeyEvent(key, state string, now time.Time) {
	if state == "up" {
		s.keyUp(key)
		return
	}
	s.keyDown(key, now)
}

func (s *Server) keyDown(key string, now time.Time) {
	if s.power.State() == power.Off {
		s.applyPower(s.power.Wake(now))
		key = ""
	}
	if key != "" && !s.applyKey(key).Repeat {
		key = ""
	}
	s.repeat.Down(key, now)
	s.armRepeatTimer()
}

func (s *Server) keyUp(key string) {
	if !s.repeat.Up(key) {
		return
	}
	s.armRepeatTimer()
}

func (s *Server) armRepeatTimer() {
	d, ok := s.repeat.Next(time.Now())
	if !ok {
		d = 0
	}
	s.repeatDeadline.arm(d, func() {
		key := s.repeat.Key()
		if key == "" {
			return
		}
		now := time.Now()
		times := s.repeat.Due(now)
		s.repeat.Fired(now)
		s.applyKeyTimes(key, times)
		s.armRepeatTimer()
	})
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.hub.add(c)
	s.setSoundBase(c.LocalAddr().String())
	s.idle.Connect(time.Now())
	s.armIdleTimer()
	log.Printf("[ws] connect %s (clients=%d)", c.RemoteAddr(), s.hub.count())
	defer func() {
		s.hub.remove(c)
		s.idle.Disconnect(time.Now())
		s.armIdleTimer()
		log.Printf("[ws] close %s", c.RemoteAddr())
	}()

	// Force an initial push so the newcomer gets the screen AND lastAwaiting is
	// synced — otherwise a digit answering a prompt that was already on screen at
	// connect time would wrongly land in the input buffer.
	s.pushIfChanged(true)
	s.hub.broadcast(powerMessage(s.power.State()))
	s.resendLed()

	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		var m struct {
			Type    string   `json:"type"`
			Key     string   `json:"key"`
			Text    string   `json:"text"`
			State   string   `json:"state"`
			Usb     bool     `json:"usb"`
			Mv      int      `json:"mv"`
			Heap    int      `json:"heap"`
			HeapMin int      `json:"heapmin"`
			List    []beacon `json:"list"`
			Events  []struct {
				Key   string `json:"key"`
				State string `json:"state"`
			} `json:"events"`
		}
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "report":
			log.Printf("[device] heap=%d minheap=%d", m.Heap, m.HeapMin)
			s.handleReport(m.Usb, m.Mv, time.Now())
		case "beacons":
			s.handleBeacons(m.List)
		case "wake":
			s.applyPower(s.power.Wake(time.Now()))
		case "sleep":
			s.applyPower(s.power.Sleep(time.Now()))
		case "key":
			s.handleKeyEvent(m.Key, m.State, time.Now())
		case "keys":
			now := time.Now()
			for _, e := range m.Events {
				s.handleKeyEvent(e.Key, e.State, now)
			}
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
	s.mu.Lock()
	sess := s.sess
	awaiting := sess != nil && sess.lastAwaiting
	s.mu.Unlock()

	exists := false
	if sess != nil {
		exists = sess.backend.Exists()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"machine":  s.cfg.Name,
		"name":     s.cfg.Name,
		"current":  s.currentName(),
		"sessions": s.sessionNames(),
		"exists":   exists,
		"notify":   s.NotifyEnabled(),
		"awaiting": awaiting,
	})
}

func (s *Server) pushAfter() time.Duration {
	if s.cfg.PushDebounce > 0 {
		return s.cfg.PushDebounce
	}
	return defaultPushDebounce
}

func (s *Server) schedulePush() {
	s.pushMu.Lock()
	defer s.pushMu.Unlock()
	if s.pushTimer != nil {
		return
	}
	s.pushTimer = time.AfterFunc(s.pushAfter(), func() {
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
	if err := terminal.Available(); err != nil {
		return err
	}
	port := discovery.PickPort(discovery.FreePort, portStart, portTries)
	if port == 0 {
		return fmt.Errorf("no free port between %d and %d", portStart, portStart+portTries-1)
	}
	target := sessionTarget(s.cfg)
	created, ok := terminal.CreateBackend(target, s.cfg.ScrollbackLines).EnsureSession(s.cfg.SessionCwd)
	if !ok {
		return fmt.Errorf("could not attach to or create terminal '%s'", target)
	}
	if created {
		log.Printf("[expose] no terminal '%s' was running — created an empty one; run cardputerme from inside the terminal you want mirrored", target)
	}
	s.register(s.cfg.Name, target, s.cfg.SessionCwd)

	mux := http.NewServeMux()
	s.routes(mux)

	log.Printf("cardputerme — exposing '%s' on http://0.0.0.0:%d  (ws://…/ws)", s.cfg.Name, port)
	log.Printf("  screen : dim after %v, off after %v (0 = never)", s.cfg.DimAfter, s.cfg.OffAfter)
	log.Printf("  idle   : exit after %v with no device connected (0 = never)", s.cfg.IdleExit)
	if err := publishPort(port); err != nil {
		log.Printf("[expose] could not publish the port (%v) — `cardputerme` in another project will start a second server instead of attaching", err)
	}
	go s.dispatch()
	s.startBeacon(port)
	s.armPowerTimer()
	s.armIdleTimer()

	srvErr := make(chan error, 1)
	go func() { srvErr <- http.ListenAndServe(":"+strconv.Itoa(port), mux) }()
	select {
	case err := <-srvErr:
		return err
	case <-s.done:
		return nil
	}
}
