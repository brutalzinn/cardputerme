package terminal

import (
	_ "embed"
	"errors"
	"os/exec"
	"strings"
)

// SessionInfo is one tmux session found on the machine — enough to register
// it (name = tmux target, cwd = where it started).
type SessionInfo struct {
	Name string
	Cwd  string
}

func parseSessionList(output string) []SessionInfo {
	var sessions []SessionInfo
	for _, line := range strings.Split(output, "\n") {
		name, cwd, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		sessions = append(sessions, SessionInfo{Name: name, Cwd: cwd})
	}
	return sessions
}

// ListSessions enumerates every tmux session on the default socket. An empty
// result (no server, or a server with no sessions) is normal, not an error —
// `tmux list-sessions` exits non-zero for both and the distinction does not
// matter to a caller that only wants to know what to register.
func ListSessions() []SessionInfo {
	code, out := tmux("list-sessions", "-F", "#{session_name}\t#{session_path}")
	if code != 0 {
		return nil
	}
	return parseSessionList(out)
}

// discoveryHookCmd runs on every new tmux session, wherever it was created —
// a shell, a script, `tmux new`. It reuses the existing launcher unchanged:
// `cd` into the pane's cwd (tmux expands #{...} before the shell runs, so
// this does not depend on run-shell's own, unrelated working directory)
// and invoke `cardputerme` exactly as a human would, naming it after the
// tmux session so the label matches what tmux itself already calls it.
const discoveryHookCmd = `run-shell -b "cd '#{pane_current_path}' && SESSION='#{session_name}' cardputerme '#{session_name}' >>$HOME/.cardputerme/hook.log 2>&1"`

func discoveryHookArgs() []string {
	return []string{"set-hook", "-g", "session-created", discoveryHookCmd}
}

//go:embed scripts/install-tmux-hook.sh
var installHookScript string

// InstallDiscoveryHook (re-)arms tmux's own session-created notification so a
// brand-new session exposes itself with zero manual commands. It runs the
// embedded install-tmux-hook.sh (see scripts/), which both applies the hook
// to the tmux server already running AND persists it to ~/.tmux.conf so it
// survives a `tmux kill-server` — a bare `tmux set-hook` call does not.
// Idempotent, so Run() calls this on every startup.
//
// A failure here is not cosmetic: it means new tmux sessions will silently
// never expose themselves, which defeats the entire point of the server, so
// the caller is expected to treat a non-nil error as fatal.
func InstallDiscoveryHook() error {
	out, err := exec.Command("bash", "-c", installHookScript).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}
