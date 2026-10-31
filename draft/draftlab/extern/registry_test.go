package extern

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/refagent"
	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

// TestRegistryVersionFreeze: an exec agent resolves once per endpoint, a
// pinned model_version that the hello does not serve is a draftlab.ErrVersion
// error, and the failed dial registers nothing.
func TestRegistryVersionFreeze(t *testing.T) {
	l := testLab(t)
	t.Setenv("GORGE_EXTAGENT_HELPER", `{"model_version":"1"}`)
	reg := NewRegistry(context.Background(), l, Options{Stderr: io.Discard})
	d, err := reg.Drafter("ext:exec:" + os.Args[0] + "@1")
	if err != nil {
		t.Fatalf("Drafter: %v", err)
	}
	if d.Ref != (draftlab.AgentRef{Name: "ext:random", Version: "1"}) {
		t.Errorf("drafter ref = %s, want ext:random@1", d.Ref)
	}
	_, err = reg.Player("ext:exec:" + os.Args[0] + "@2")
	if !errors.Is(err, draftlab.ErrVersion) {
		t.Errorf("pinned @2: error %v does not wrap draftlab.ErrVersion", err)
	}
	p, err := reg.Player("ext:exec:" + os.Args[0])
	if err != nil {
		t.Fatalf("Player (no pin): %v", err)
	}
	if p.Ref != (draftlab.AgentRef{Name: "ext:random", Version: "1"}) {
		t.Errorf("player ref = %s, want ext:random@1", p.Ref)
	}
	if got := len(reg.Agents()); got != 1 {
		t.Errorf("registry holds %d agents, want 1 (the failed pin must not register)", got)
	}
	if err := reg.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestRegistryBuiltinPassThrough: a non-ext spec resolves through the lab's
// built-in registry and dials nothing.
func TestRegistryBuiltinPassThrough(t *testing.T) {
	l := testLab(t)
	reg := NewRegistry(context.Background(), l, Options{})
	d, err := reg.Drafter("rating-colour@1")
	if err != nil {
		t.Fatalf("Drafter: %v", err)
	}
	want, err := l.Drafter(draftlab.AgentRef{Name: "rating-colour", Version: "1"})
	if err != nil {
		t.Fatalf("lab Drafter: %v", err)
	}
	if d.Ref != want.Ref {
		t.Errorf("registry ref %s != lab ref %s", d.Ref, want.Ref)
	}
	p, err := reg.Player("bot@1")
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	wantP, err := l.Player(draftlab.AgentRef{Name: "bot", Version: "1"})
	if err != nil {
		t.Fatalf("lab Player: %v", err)
	}
	if p.Ref != wantP.Ref {
		t.Errorf("player ref %s != lab ref %s", p.Ref, wantP.Ref)
	}
	if got := len(reg.Agents()); got != 0 {
		t.Errorf("registry dialed %d agents for a built-in spec", got)
	}
	if err := reg.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestRegistryRejectsBadSpec: a bad ext spec and an unknown built-in are
// wrapped errors, never a dial.
func TestRegistryRejectsBadSpec(t *testing.T) {
	l := testLab(t)
	reg := NewRegistry(context.Background(), l, Options{})
	if _, err := reg.Player("ext:tcp:127.0.0.1:1"); !errors.Is(err, ErrSpec) {
		t.Errorf("ext tcp spec: error %v does not wrap ErrSpec", err)
	}
	if _, err := reg.Drafter("ext:"); !errors.Is(err, ErrSpec) {
		t.Errorf("bare ext:: error %v does not wrap ErrSpec", err)
	}
	if _, err := reg.Builder("no-such-agent@1"); !errors.Is(err, draftlab.ErrUnknownAgent) {
		t.Errorf("unknown built-in: error %v does not wrap ErrUnknownAgent", err)
	}
	if got := len(reg.Agents()); got != 0 {
		t.Errorf("registry registered %d agents on refused specs", got)
	}
}

// TestRoleNotServed: an agent whose hello serves only decide wraps ErrRole
// on the other two roles and serves the third.
func TestRoleNotServed(t *testing.T) {
	l := testLab(t)
	a := dialRefagent(t, newHub(), refagent.Config{Kinds: []string{wire.KindDecide}}, Options{})
	if _, err := a.Drafter(l); !errors.Is(err, ErrRole) {
		t.Errorf("Drafter: error %v does not wrap ErrRole", err)
	}
	if _, err := a.Builder(l); !errors.Is(err, ErrRole) {
		t.Errorf("Builder: error %v does not wrap ErrRole", err)
	}
	p, err := a.Player(l)
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	if p.Ref != (draftlab.AgentRef{Name: "ext:random", Version: "1"}) {
		t.Errorf("player ref = %s, want ext:random@1", p.Ref)
	}
}
