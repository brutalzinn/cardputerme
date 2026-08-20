package terminal

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseSessionListSplitsNameAndCwd(t *testing.T) {
	got := parseSessionList("proj\t/home/rob/proj\nother\t/tmp\n")
	want := []SessionInfo{{Name: "proj", Cwd: "/home/rob/proj"}, {Name: "other", Cwd: "/tmp"}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseSessionListIgnoresBlankLines(t *testing.T) {
	got := parseSessionList("\nproj\t/tmp\n\n")
	if len(got) != 1 || got[0].Name != "proj" {
		t.Fatalf("got %v", got)
	}
}

func TestParseSessionListSkipsAMalformedLine(t *testing.T) {
	got := parseSessionList("no-tab-here\nproj\t/tmp\n")
	if len(got) != 1 || got[0].Name != "proj" {
		t.Fatalf("a line with no separator must be skipped, got %v", got)
	}
}

func TestParseSessionListOnEmptyOutputIsEmpty(t *testing.T) {
	if got := parseSessionList(""); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestDiscoveryHookTargetsSessionCreated(t *testing.T) {
	got := discoveryHookArgs()
	joined := strings.Join(got, " ")
	for _, want := range []string{"set-hook", "-g", "session-created", "cardputerme"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("hook args %v missing %q", got, want)
		}
	}
}

func TestDiscoveryHookExpandsThePaneCwdBeforeInvokingIt(t *testing.T) {
	got := discoveryHookArgs()
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "#{pane_current_path}") || !strings.Contains(joined, "#{session_name}") {
		t.Fatalf("hook args %v must use tmux's own format strings, not the caller's cwd", got)
	}
}

func skipWithoutTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
}

func TestListSessionsFindsARealSession(t *testing.T) {
	skipWithoutTmux(t)
	name := "discover-list-test"
	exec.Command("tmux", "kill-session", "-t", name).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", "/tmp").Run(); err != nil {
		t.Skipf("cannot create tmux session: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", name).Run()

	found := false
	for _, s := range ListSessions() {
		if s.Name == name {
			found = true
			if s.Cwd != "/tmp" {
				t.Fatalf("cwd = %q, want /tmp", s.Cwd)
			}
		}
	}
	if !found {
		t.Fatalf("ListSessions did not report %q", name)
	}
}

func TestInstallDiscoveryHookIsReflectedByTmux(t *testing.T) {
	skipWithoutTmux(t)
	// A hook needs a live tmux server to attach to.
	name := "discover-hook-test"
	exec.Command("tmux", "kill-session", "-t", name).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", "/tmp").Run(); err != nil {
		t.Skipf("cannot create tmux session: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", name).Run()

	if err := InstallDiscoveryHook(); err != nil {
		t.Fatalf("InstallDiscoveryHook reported failure: %v", err)
	}
	out, err := exec.Command("tmux", "show-hooks", "-g").Output()
	if err != nil {
		t.Fatalf("show-hooks: %v", err)
	}
	if !strings.Contains(string(out), "session-created") {
		t.Fatalf("show-hooks -g did not list session-created: %s", out)
	}
}

// installHookScript's failure branches matter as much as its success path:
// InstallDiscoveryHook's caller (server.Run) treats any error as fatal, so a
// false negative here would mean the server refuses to start when it should
// not, and a false positive would mean it silently starts without the hook.
// Exercised directly against the embedded script text — with PATH and HOME
// overridden — so this never touches the real tmux server the rest of the
// suite (and the developer's own machine) depends on.
func TestInstallHookScriptFailsWithoutTmuxOnPath(t *testing.T) {
	cmd := exec.Command("bash", "-c", installHookScript)
	cmd.Env = []string{"PATH=/nonexistent", "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure with no tmux on PATH, got success: %s", out)
	}
	if !strings.Contains(string(out), "tmux is not installed") {
		t.Fatalf("expected a tmux-not-installed message, got: %s", out)
	}
}

func TestInstallHookScriptFailsWithoutARunningTmuxServer(t *testing.T) {
	skipWithoutTmux(t)
	home := t.TempDir()
	cmd := exec.Command("bash", "-c", installHookScript)
	cmd.Env = append(os.Environ(), "HOME="+home, "TMUX_TMPDIR="+home)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure with no tmux server running, got success: %s", out)
	}
	if !strings.Contains(string(out), "no tmux server is running") {
		t.Fatalf("expected a no-server message, got: %s", out)
	}
	// The hook line is still persisted even though the live apply failed —
	// that half is unconditional so a later tmux server start still picks it up.
	conf, rerr := os.ReadFile(home + "/.tmux.conf")
	if rerr != nil || !strings.Contains(string(conf), "session-created") {
		t.Fatalf(".tmux.conf was not written with the hook: %v %s", rerr, conf)
	}
}
