package commands

import "strings"

import "testing"

type fakeNotify struct{ on bool }

func (f *fakeNotify) NotifyEnabled() bool { return f.on }
func (f *fakeNotify) SetNotify(v bool)    { f.on = v }

func ctxWith(f *fakeNotify) Ctx { return Ctx{Name: "test", Notify: f} }

func TestNotifyAcceptsOneAndZero(t *testing.T) {
	f := &fakeNotify{on: true}
	if out := Run(ctxWith(f), "notify 0"); f.on {
		t.Fatalf("`;notify 0` must turn it off, reply=%q", out)
	}
	if out := Run(ctxWith(f), "notify 1"); !f.on {
		t.Fatalf("`;notify 1` must turn it on, reply=%q", out)
	}
}

func TestNotifyAcceptsOnAndOff(t *testing.T) {
	f := &fakeNotify{on: true}
	Run(ctxWith(f), "notify off")
	if f.on {
		t.Fatal("`;notify off` must turn it off")
	}
	Run(ctxWith(f), "notify on")
	if !f.on {
		t.Fatal("`;notify on` must turn it on")
	}
}

func TestBareNotifyReportsWithoutChanging(t *testing.T) {
	f := &fakeNotify{on: true}
	out := Run(ctxWith(f), "notify")
	if !f.on {
		t.Fatal("a bare report must not change the setting")
	}
	if !strings.Contains(strings.ToLower(out), "on") {
		t.Fatalf("report should say the current value, got %q", out)
	}
}

func TestNotifyRejectsGarbageVisibly(t *testing.T) {
	f := &fakeNotify{on: true}
	out := Run(ctxWith(f), "notify maybe")
	if !f.on {
		t.Fatal("garbage must not change the setting")
	}
	if out == "" {
		t.Fatal("unknown input must never be silent (#32a)")
	}
	if !strings.Contains(out, "0") || !strings.Contains(out, "1") {
		t.Fatalf("the reply should name what IS valid, got %q", out)
	}
}

func TestNotifyIsListedInHelp(t *testing.T) {
	if !strings.Contains(List(), "notify") {
		t.Fatal("a command the user cannot discover may as well not exist")
	}
}

func TestNotifyWithoutAHostDoesNotPanic(t *testing.T) {
	if out := Run(Ctx{Name: "test"}, "notify 1"); out == "" {
		t.Fatal("with no host wired, say so rather than crashing or going silent")
	}
}
