package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
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

// corpusCopySA returns the REAL compiled CopySpellAbility sub-ability of a
// named corpus card, walking every face's abilities, triggers and their
// SubAbility chains. The copy shapes hide in different places: Chain
// Lightning's lives in a spell's SubAbility (DBCopy1), Wandering Archaic's
// in a trigger's Effect (TrigCopySpell). The brief insists on real compiled
// SAs rather than a synthetic map fixture -- a hand-built bag is exactly
// what let the inverted unswitched orientation look right (B2).
func corpusCopySA(t *testing.T, name string) *cards.SA {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	var found *cards.SA
	var walk func(*cards.SA)
	walk = func(sa *cards.SA) {
		if sa == nil || found != nil {
			return
		}
		if sa.API == "CopySpellAbility" {
			found = sa
			return
		}
		walk(sa.Sub)
	}
	for _, f := range c.Faces {
		for _, a := range f.Abilities {
			walk(a)
		}
		for _, t := range f.Triggers {
			walk(t.Effect)
		}
	}
	if found == nil {
		t.Fatalf("corpus card %q has no CopySpellAbility", name)
	}
	return found
}

// TestCopySpellAbilityUnswitchedNoAskHostCopiesOnTheDecline covers the
// no-ask determination for the UNSWITCHED shape (an UnlessCost$ with NO
// UnlessSwitched$): its deterministic decline IS the copy path ("if they
// don't pay, you may copy"), so a host that cannot ask (an effects-package
// test double) still copies and records the suppressed-ask Note -- the
// opposite of the SWITCHED shape, where the deterministic decline copies
// nothing.
func TestCopySpellAbilityUnswitchedNoAskHostCopiesOnTheDecline(t *testing.T) {
	h := newHost(t, 2)
	o := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	Resolve(h, &Ctx{Source: o.ID, Controller: 0},
		sa(t, "SP$ CopySpellAbility | Defined$ Parent | UnlessCost$ R R | UnlessPayer$ TargetedOrController"))

	if got := copyEvents(h); got != 1 {
		t.Fatalf("%d StackCopy events, want 1 (the unswitched decline IS the copy path)", got)
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

// TestCopySpellAbilitySwitchedShapePayingMakesTheCopies drives Chain
// Lightning's REAL compiled copy SA -- the SWITCHED shape (UnlessCost$ R R
// with UnlessSwitched$ True). Paying CAUSES the copy, so "decline" makes
// nothing and "pay" makes exactly one; the copy keeps the target's
// controller as payer (UnlessPayer$ TargetedOrController, resolved from the
// first target). This is main's already-correct behaviour and must stay put:
// the guard reads UnlessSwitched$ True, so the switched shapes are untouched
// by the unswitched inversion.
func TestCopySpellAbilitySwitchedShapePayingMakesTheCopies(t *testing.T) {
	saCL := corpusCopySA(t, "Chain Lightning")
	if !strings.EqualFold(strings.TrimSpace(saCL.Params["UnlessSwitched"]), "True") {
		t.Fatalf("Chain Lightning copy SA unexpectedly lacks UnlessSwitched$ True: %+v", saCL.Params)
	}
	h := &askHost{}
	h.g = state.NewGame(names(2))
	o := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 0)
	Resolve(h, &Ctx{Source: o.ID, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}, saCL)
	if h.asked == nil {
		t.Fatal("no pay decision was posed")
	}
	d := h.asked
	if d.Kind != decision.KModes || d.Player != 1 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==Max==1 KModes posed to the TARGET's controller (seat 1)", d)
	}
	if len(d.Options) != 2 || d.Options[0].Label != "Pay R R — make a copy" {
		t.Fatalf("options = %+v, want the switched pay/decline pair headed by the copy label", d.Options)
	}
	if got := copyEvents(&h.fakeHost); got != 0 {
		t.Fatalf("%d copies made before the pay decision was answered", got)
	}
	// Re-entry, decline: the switched shape copies nothing.
	Resolve(h, &Ctx{Source: o.ID, Controller: 0, UnlessPay: "decline"}, saCL)
	if got := copyEvents(&h.fakeHost); got != 0 {
		t.Fatalf("%d copies made on a switched decline, want 0", got)
	}
	// Re-entry, pay: the switched copy loop runs exactly once.
	Resolve(h, &Ctx{Source: o.ID, Controller: 0, UnlessPay: "pay"}, saCL)
	if got := copyEvents(&h.fakeHost); got != 1 {
		t.Fatalf("%d copies made on a switched pay, want 1", got)
	}
}

// TestCopySpellAbilityUnswitchedShapePayingStopsTheCopies is the corrected
// B2 carrier. Before this fix the engine treated the UNSWITCHED shape
// (Wandering Archaic: "they may pay {2}. If they don't, you may copy that
// spell.") the same as the switched one, so paying MADE the copy — exactly
// inverted. It now drives Wandering Archaic's REAL compiled corpus SA: the
// first pass poses the pay decision to the payer (for a triggered copy the
// trigger's controller, c.Controller, since UnlessPayer$ TriggeredActivator
// resolves to it), the resolution suspends before any copy, "pay" makes NO
// copy, and "decline" makes exactly one.
func TestCopySpellAbilityUnswitchedShapePayingStopsTheCopies(t *testing.T) {
	saWA := corpusCopySA(t, "Wandering Archaic")
	if _, ok := saWA.Params["UnlessSwitched"]; ok {
		t.Fatalf("Wandering Archaic copy SA unexpectedly carries UnlessSwitched$: %+v", saWA.Params)
	}
	h := &askHost{}
	h.g = state.NewGame(names(2))
	// The opponent's instant the trigger remembered (Defined$
	// TriggeredSpellAbility copies it). It must sit on the stack for the
	// copy to be a no-op-free duplicate.
	spell := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1", 1)
	Resolve(h, &Ctx{Source: spell.ID, Controller: 0,
		Remembered: []state.Target{{Obj: spell.ID}}}, saWA)
	if h.asked == nil {
		t.Fatal("no pay decision was posed")
	}
	d := h.asked
	if d.Kind != decision.KModes || d.Player != 0 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==Max==1 KModes posed to the trigger's controller (seat 0)", d)
	}
	if len(d.Options) != 2 || d.Options[0].Label != "Pay 2 — no copy" || d.Options[1].Label != "Don't pay — make a copy" {
		t.Fatalf("options = %+v, want the inverted pay/decline pair headed by the no-copy label", d.Options)
	}
	if got := copyEvents(&h.fakeHost); got != 0 {
		t.Fatalf("%d copies made before the pay decision was answered", got)
	}
	// Re-entry, pay: on the unswitched shape paying STOPS the copy. The
	// Resume path re-derives the same Ctx (source/remembered are the trigger's
	// stack object's), so Remembered is carried here as it would be in the
	// engine.
	Resolve(h, &Ctx{Source: spell.ID, Controller: 0, UnlessPay: "pay",
		Remembered: []state.Target{{Obj: spell.ID}}}, saWA)
	if got := copyEvents(&h.fakeHost); got != 0 {
		t.Fatalf("%d copies made on an unswitched pay, want 0", got)
	}
	// Re-entry, decline: the decline IS the copy path.
	Resolve(h, &Ctx{Source: spell.ID, Controller: 0, UnlessPay: "decline",
		Remembered: []state.Target{{Obj: spell.ID}}}, saWA)
	if got := copyEvents(&h.fakeHost); got != 1 {
		t.Fatalf("%d copies made on an unswitched decline, want 1", got)
	}
}
