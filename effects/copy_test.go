package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// spellOnStack places a fresh object of fixture card on the stack and returns
// it, so the copy effect has a genuine spell-in-standby to duplicate -- the
// Without-Storm companion to rules/storm_test.go, which drives the whole
// triggered path through the engine.
func spellOnStack(t *testing.T, h *fakeHost, src string, ctlr state.PlayerID) *state.Object {
	t.Helper()
	c := mkCard(t, "Name:Blast\nManaCost:R\nTypes:Sorcery\n"+src+"\nOracle:x\n")
	o := h.g.AddObject(c, ctlr)
	events.Move(h.g, o.ID, state.ZLibrary, state.ZStack)
	return h.g.Obj(o.ID)
}

func copyEvents(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.StackCopy {
			n++
		}
	}
	return n
}

func TestCopySpellAbilityDuplicatesTheSourceNamedByParent(t *testing.T) {
	h := newHost(t, 2)
	o := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	Resolve(h, &Ctx{Source: o.ID, Controller: 0},
		sa(t, "SP$ CopySpellAbility | Defined$ Parent | Amount$ 2 | MayChooseTarget$ True"))

	if got, want := copyEvents(h), 2; got != want {
		t.Fatalf("%d StackCopy events, want %d", got, want)
	}
	// The source spell never leaves the stack (a copy is placed above it).
	if o.Zone != state.ZStack {
		t.Fatalf("source left the stack: %s", o.Zone)
	}
	// A copy object actually exists, tagged IsCopy, on the stack.
	copies := 0
	for _, ob := range h.g.Objs {
		if ob.IsCopy {
			copies++
		}
	}
	if copies != 2 {
		t.Fatalf("%d copy objects, want 2", copies)
	}
	// MayChooseTarget$ True records the "keeps its targets" Note.
	notes := 0
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "keeps its targets") {
			notes++
		}
	}
	if notes != 2 {
		t.Fatalf("%d 'keeps its targets' Notes, want 2", notes)
	}
}

func TestCopySpellAbilityDeclinesUnlessCostWithANote(t *testing.T) {
	h := newHost(t, 2)
	o := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	Resolve(h, &Ctx{Source: o.ID, Controller: 0},
		sa(t, "SP$ CopySpellAbility | Defined$ Parent | UnlessCost$ R R | UnlessPayer$ TargetedOrController"))

	if got := copyEvents(h); got != 0 {
		t.Fatalf("%d StackCopy events created despite the declined may-pay", got)
	}
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "declined") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no declined-may-pay Note recorded")
	}
}
