package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
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

// TestCopySpellAbilityAsksTheUnlessPayToTheTargetsController covers the
// effects side of the R-8 closure: with a host that can ask, the first pass
// poses a KModes pay/decline decision to the TARGET's controller (the
// UnlessPayer$ TargetedOrController shape Chain Lightning uses), and the
// resolution suspends before any copy is made. The re-entry then takes the
// Ctx.UnlessPay branch: "decline" makes no copy, "pay" makes the copy (the
// payment itself is rules' job -- events/payMana -- so the Ctx contract here
// is just the flag).
func TestCopySpellAbilityAsksTheUnlessPayToTheTargetsController(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	o := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	Resolve(h, &Ctx{Source: o.ID, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}},
		sa(t, "SP$ CopySpellAbility | Defined$ Parent | UnlessCost$ R R | UnlessPayer$ TargetedOrController"))
	if h.asked == nil {
		t.Fatal("no pay decision was posed")
	}
	d := h.asked
	if d.Kind != decision.KModes || d.Player != 1 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==Max==1 KModes for the TARGET's controller (seat 1)", d)
	}
	if len(d.Options) != 2 {
		t.Fatalf("options = %+v, want the pay/decline pair", d.Options)
	}
	if got := copyEvents(&h.fakeHost); got != 0 {
		t.Fatalf("%d copies made before the pay decision was answered", got)
	}
	// Re-entry, decline: no copy.
	Resolve(h, &Ctx{Source: o.ID, Controller: 0, UnlessPay: "decline"},
		sa(t, "SP$ CopySpellAbility | Defined$ Parent | UnlessCost$ R R | UnlessPayer$ TargetedOrController"))
	if got := copyEvents(&h.fakeHost); got != 0 {
		t.Fatalf("%d copies made on a decline", got)
	}
	// Re-entry, pay: the copy loop runs (the pool payment is rules' job).
	Resolve(h, &Ctx{Source: o.ID, Controller: 0, UnlessPay: "pay"},
		sa(t, "SP$ CopySpellAbility | Defined$ Parent | UnlessCost$ R R | UnlessPayer$ TargetedOrController"))
	if got := copyEvents(&h.fakeHost); got != 1 {
		t.Fatalf("%d copies made on a pay, want 1", got)
	}
}
