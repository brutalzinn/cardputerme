package terminal

import (
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

	if !InstallDiscoveryHook() {
		t.Fatal("InstallDiscoveryHook reported failure")
	}
	out, err := exec.Command("tmux", "show-hooks", "-g").Output()
	if err != nil {
		t.Fatalf("show-hooks: %v", err)
	}
	if !strings.Contains(string(out), "session-created") {
		t.Fatalf("show-hooks -g did not list session-created: %s", out)
	}
}
