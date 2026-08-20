package terminal

import "strings"

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

// InstallDiscoveryHook (re-)arms tmux's own session-created notification so a
// brand-new session exposes itself with zero manual commands. Idempotent —
// setting the same hook twice is harmless — so Run() calls this on every
// startup rather than trusting install.sh's one-time ~/.tmux.conf write to
// have survived a tmux server restart.
func InstallDiscoveryHook() bool {
	code, _ := tmux(discoveryHookArgs()...)
	return code == 0
}
