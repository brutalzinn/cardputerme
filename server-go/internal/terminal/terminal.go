package terminal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var ErrNoTmux = errors.New("tmux is not installed — cardputerme captures your terminal through it and cannot run without it (brew install tmux / apt install tmux)")

func lookupTmux(look func(string) (string, error)) error {
	if _, err := look("tmux"); err != nil {
		return ErrNoTmux
	}
	return nil
}

func Available() error {
	return lookupTmux(exec.LookPath)
}

var keyNames = map[string]string{
	"enter":     "Enter",
	"escape":    "Escape",
	"tab":       "Tab",
	"up":        "Up",
	"down":      "Down",
	"left":      "Left",
	"right":     "Right",
	"backspace": "BSpace",
	"space":     "Space",
}

func namedKey(key string) (string, bool) {
	if v, ok := keyNames[key]; ok {
		return v, true
	}
	if strings.HasPrefix(key, "ctrl+") && len(key) == 6 {
		return "C-" + key[5:], true
	}
	return "", false
}

type Backend struct {
	session         string
	scrollbackLines int
	typeDelay       time.Duration
}

func CreateBackend(session string, scrollbackLines int) *Backend {
	return &Backend{session: session, scrollbackLines: scrollbackLines, typeDelay: 120 * time.Millisecond}
}

func tmux(args ...string) (int, string) {
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), string(out)
		}
		return 1, string(out)
	}
	return 0, string(out)
}

func (b *Backend) Exists() bool {
	code, _ := tmux("has-session", "-t", b.session)
	return code == 0
}

func (b *Backend) EnsureSession(cwd string) bool {
	if code, _ := tmux("has-session", "-t", b.session); code == 0 {
		return true
	}
	args := []string{"new-session", "-d", "-s", b.session}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	code, _ := tmux(args...)
	return code == 0
}

// Capture returns the visible pane text (with ANSI), or ("", false) if the
// session is gone — the caller distinguishes gone from blank via the bool.
func (b *Backend) Capture() (string, bool) {
	args := []string{"capture-pane", "-p", "-e", "-t", b.session}
	if b.scrollbackLines > 0 {
		args = append(args, "-S", "-"+strconv.Itoa(b.scrollbackLines))
	}
	code, out := tmux(args...)
	if code != 0 {
		return "", false
	}
	return out, true
}

func (b *Backend) SendText(text string) bool {
	if code, _ := tmux("send-keys", "-t", b.session, "-l", text); code != 0 {
		return false
	}
	if b.typeDelay > 0 {
		time.Sleep(b.typeDelay)
	}
	code, _ := tmux("send-keys", "-t", b.session, "Enter")
	return code == 0
}

func (b *Backend) SendKey(key string) bool {
	if named, ok := namedKey(key); ok {
		code, _ := tmux("send-keys", "-t", b.session, named)
		return code == 0
	}
	code, _ := tmux("send-keys", "-t", b.session, "-l", key)
	return code == 0
}

// Subscribe streams pane changes via `tmux pipe-pane` into a fifo. onChange
// fires per burst of pane output; onGone fires when the session dies (detected
// on the fifo writer closing — no timers, no polling). Returns a stop func.
func (b *Backend) Subscribe(onChange, onGone func()) func() {
	fifo := filepath.Join(os.TempDir(), "cardputerme-"+b.session+"-"+strconv.Itoa(os.Getpid())+".fifo")
	os.Remove(fifo)
	var stopped atomic.Bool

	var read func()
	read = func() {
		if stopped.Load() {
			return
		}
		f, err := os.OpenFile(fifo, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			return
		}
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				onChange()
			}
			if err != nil {
				break
			}
		}
		f.Close()
		if stopped.Load() {
			return
		}
		if !b.Exists() {
			onGone()
			return
		}
		read()
	}

	if err := syscall.Mkfifo(fifo, 0o600); err == nil {
		tmux("pipe-pane", "-o", "-t", b.session, "cat > '"+fifo+"'")
		go read()
	}

	return func() {
		stopped.Store(true)
		tmux("pipe-pane", "-t", b.session)
		os.Remove(fifo)
	}
}
