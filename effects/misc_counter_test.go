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

// corpusCounterSA returns the REAL compiled Counter sub-ability of a named
// corpus card (Mana Leak, Counterspell, Runeboggle). The brief insists on
// real compiled SAs rather than a synthetic map[string]string fixture --
// the exact opposite of what shipped two bugs this week -- so the payer
// resolution and the ask/re-entry contract are asserted against the real
// card parameters (UnlessCost$ 3, no UnlessPayer$, a SubAbility$ chain),
// not a hand-built bag that could quietly differ.
func corpusCounterSA(t *testing.T, name string) *cards.SA {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	for _, f := range c.Faces {
		for _, a := range f.Abilities {
			if a.API == "Counter" {
				return a
			}
		}
	}
	t.Fatalf("corpus card %q has no Counter ability", name)
	return nil
}

// counterSource makes a throwaway object to stand in for the resolving
// counterspell -- only its ID feeds the ask's Source/option Obj, so it need
// not correspond to the real corpus card whose SA we resolve.
func counterSource(t *testing.T, h *fakeHost, ctlr state.PlayerID) state.ObjID {
	t.Helper()
	return h.g.AddObject(mkCard(t, "Name:Counterer\nManaCost:U\nTypes:Instant\nOracle:x\n"), ctlr).ID
}

// counterMoves counts how many times id left the stack for `to` via a
// MoveZone -- the "was the spell countered" observable.
func counterMoves(h *fakeHost, id state.ObjID, to state.Zone) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZStack && ev.To == to {
			n++
		}
	}
	return n
}

func hasNoteContaining(h *fakeHost, sub string) bool {
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, sub) {
			return true
		}
	}
	return false
}

// TestCounterUnlessCostAsksTheCounteredSpellsController is the core defect
// test: Mana Leak's real SA must offer the pay/decline decision to the
// CONTROLLER OF THE COUNTERED SPELL (seat 1 here), not to the caster of the
// counterspell (c.Controller == seat 0). effCopySpellAbility's default is
// c.Controller, which would ask seat 0 to pay their own counterspell's tax;
// effCounter must not inherit it. On the unmodified tree effCounter ignores
// UnlessCost$ entirely and never asks, so this fails with h.asked == nil.
func TestCounterUnlessCostAsksTheCounteredSpellsController(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := counterSource(t, &h.fakeHost, 0)
	target := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	Resolve(h, &Ctx{Source: src, Controller: 0,
		Targets: []state.Target{{Obj: target.ID}}}, corpusCounterSA(t, "Mana Leak"))

	if h.asked == nil {
		t.Fatal("no pay decision was posed for Mana Leak's UnlessCost$ 3")
	}
	d := h.asked
	if d.Kind != decision.KModes || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==Max==1 KModes", d)
	}
	if d.Player != 1 {
		t.Fatalf("payer = seat %d, want the CONTROLLER OF THE COUNTERED SPELL (seat 1), not the caster (seat 0)", d.Player)
	}
	if !strings.Contains(d.Prompt, "Pay 3") {
		t.Fatalf("prompt = %q, want it to name the {3} tax", d.Prompt)
	}
	if len(d.Options) != 2 || d.Options[0].Label != "Pay 3 — don't counter" || d.Options[1].Label != "Don't pay" {
		t.Fatalf("options = %+v, want the pay/decline pair", d.Options)
	}
	// The ask suspends: the countered spell must not have been countered yet.
	if target.Zone != state.ZStack {
		t.Fatalf("spell was countered before the pay decision was answered: zone %s", target.Zone)
	}
}

// TestCounterUnlessCostPayDoesNotCounter: once the payer has actually paid
// (Ctx.UnlessPay == "pay", set by rules' resumeResolution after payMana
// spent the cost), the counterspell does NOT counter -- the spell resolves
// normally. On the unmodified tree effCounter counters regardless of
// UnlessPay, so this fails (the spell leaves the stack).
func TestCounterUnlessCostPayDoesNotCounter(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := counterSource(t, &h.fakeHost, 0)
	target := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	// First pass: the ask suspends.
	Resolve(h, &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target.ID}}},
		corpusCounterSA(t, "Mana Leak"))
	if h.asked == nil {
		t.Fatal("no pay decision posed on the first pass")
	}
	if got := counterMoves(&h.fakeHost, target.ID, state.ZGraveyard); got != 0 {
		t.Fatalf("spell countered on the first pass before payment: %d", got)
	}
	// Re-entry, paid: not countered; the spell stays on the stack.
	Resolve(h, &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target.ID}},
		UnlessPay: "pay"}, corpusCounterSA(t, "Mana Leak"))
	if got := counterMoves(&h.fakeHost, target.ID, state.ZGraveyard); got != 0 {
		t.Fatalf("spell was countered despite the paid UnlessCost: %d move(s)", got)
	}
	if target.Zone != state.ZStack {
		t.Fatalf("paid-off spell left the stack: zone %s", target.Zone)
	}
}

// TestCounterUnlessCostDeclineCounters: once the payer declines
// (Ctx.UnlessPay == "decline"), the counterspell counters. The first pass
// must suspend (the ask is the bridge between "asked" and "declined"), and
// on the unmodified tree the spell is countered on that first pass instead
// of suspended, so this fails at the suspension check.
func TestCounterUnlessCostDeclineCounters(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := counterSource(t, &h.fakeHost, 0)
	target := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	Resolve(h, &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target.ID}}},
		corpusCounterSA(t, "Mana Leak"))
	if h.asked == nil {
		t.Fatal("no pay decision posed on the first pass")
	}
	if target.Zone != state.ZStack {
		t.Fatalf("spell countered on the first pass before the decline: zone %s", target.Zone)
	}
	Resolve(h, &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target.ID}},
		UnlessPay: "decline"}, corpusCounterSA(t, "Mana Leak"))
	if target.Zone != state.ZGraveyard {
		t.Fatalf("declined spell zone = %s, want Graveyard (it was countered)", target.Zone)
	}
	if got := counterMoves(&h.fakeHost, target.ID, state.ZGraveyard); got != 1 {
		t.Fatalf("move-to-graveyard count = %d, want exactly 1", got)
	}
}

// TestCounterWithoutUnlessCostCountersUnconditionally: Counterspell carries
// no UnlessCost$, so it must counter with NO ask posed (the guards around
// the unless-pay branch must not fire), and the spell hits the graveyard
// directly. This is the no-regression guard for the no-ask corpus shape; on
// both the unmodified and modified tree it counters unconditionally (that is
// why it is a guard, not a discriminative test -- the discriminator is
// TestCounterUnlessCostAsks... ).
func TestCounterWithoutUnlessCostCountersUnconditionally(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := counterSource(t, &h.fakeHost, 0)
	target := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	Resolve(h, &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target.ID}}},
		corpusCounterSA(t, "Counterspell"))
	if h.asked != nil {
		t.Fatalf("Counterspell posed a pay decision despite having no UnlessCost$: %+v", h.asked)
	}
	if target.Zone != state.ZGraveyard {
		t.Fatalf("Counterspell target zone = %s, want Graveyard", target.Zone)
	}
}

// TestCounterNoAskHostDeclinesDeterministically: a host that cannot ask
// (fakeHost.Ask returns false -- the effects-package test double, or a
// rules context with no engine to drive) falls back to the deterministic
// decline: the spell is countered, and a Note records that the pay was never
// actually posed (R-9). The stand-in sits behind `if h.Ask(d) { return }`.
// On the unmodified tree effCounter never reached the ask branch, so no
// Note was emitted -- that is what makes this fail there.
func TestCounterNoAskHostDeclinesDeterministically(t *testing.T) {
	h := newHost(t, 2)
	src := counterSource(t, h, 0)
	target := spellOnStack(t, h, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
	Resolve(h, &Ctx{Source: src, Controller: 0, Targets: []state.Target{{Obj: target.ID}}},
		corpusCounterSA(t, "Mana Leak"))
	if target.Zone != state.ZGraveyard {
		t.Fatalf("no-ask host must still counter (deterministic decline): zone %s", target.Zone)
	}
	if !hasNoteContaining(h, "declined") {
		t.Fatal("no Note recorded the no-ask decline stand-in")
	}
}
