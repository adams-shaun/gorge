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

// corpusSwitchedCounterSA returns the REAL compiled Counter SA of a named
// corpus card that carries UnlessSwitched$ True. All five of the corpus's
// switched Counter shapes hang off a T: line's Execute$ SVar rather than a
// face ability, so unlike corpusCounterSA this walks Abilities, every
// Trigger.Effect, every Repl.With and each of their Sub chains. It also
// asserts that the SA it found really carries both UnlessCost$ and
// UnlessSwitched$ True: a synthetic map[string]string fixture would prove
// nothing about the corpus, and that shortcut has shipped a regression here
// before.
func corpusSwitchedCounterSA(t *testing.T, name string) *cards.SA {
	t.Helper()
	return corpusUnlessCounterSA(t, name, func(sa *cards.SA) bool {
		return strings.EqualFold(sa.Params["UnlessSwitched"], "True")
	})
}

// corpusUnlessCounterSA finds the first compiled Counter SA of a corpus card
// that carries a non-empty UnlessCost$ and satisfies extra, searching
// Abilities, every Trigger.Effect, every Repl.With and each of their Sub
// chains. It asserts the UnlessCost$ is really there, so a caller can never
// be handed something that only looks like the shape under test.
func corpusUnlessCounterSA(t *testing.T, name string, extra func(*cards.SA) bool) *cards.SA {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	var found *cards.SA
	walk := func(sa *cards.SA) {
		for ; sa != nil && found == nil; sa = sa.Sub {
			if sa.API != "Counter" || strings.TrimSpace(sa.Params["UnlessCost"]) == "" {
				continue
			}
			if extra == nil || extra(sa) {
				found = sa
				return
			}
		}
	}
	for _, f := range c.Faces {
		for _, a := range f.Abilities {
			walk(a)
		}
		for _, tr := range f.Triggers {
			walk(tr.Effect)
		}
		for _, r := range f.Repls {
			walk(r.With)
		}
	}
	if found == nil {
		t.Fatalf("corpus card %q has no compiled Counter SA with UnlessCost$ matching the predicate", name)
	}
	return found
}

// TestCounterUnlessSwitchedSuppressesTheAsk pins the suppression on all five
// REAL compiled corpus Counter SAs that carry UnlessSwitched$ True.
//
// UnlessSwitched$ True inverts the deal: paying CAUSES the counter. The
// engine does not implement that, and posing the ordinary ask on these cards
// is worse than posing nothing -- it is backwards, letting a player prevent
// a counter by paying for it. So effCounter must not pose the ask at all on
// a switched shape, and must counter unconditionally, which is exactly what
// main did before the UnlessCost$ ask existed.
//
// This FAILS without the `&& !switched` guard: every one of these SAs has a
// non-empty UnlessCost$, so the unguarded branch poses the decision and
// suspends instead of countering. Verified by running it on a tree without
// the guard, not asserted.
func TestCounterUnlessSwitchedSuppressesTheAsk(t *testing.T) {
	for _, name := range []string{
		"Brain Gorgers", "Dash Hopes", "Ice Cave", "Phantasmagorian", "Temporal Extortion",
	} {
		t.Run(name, func(t *testing.T) {
			sa := corpusSwitchedCounterSA(t, name)
			h := &askHost{}
			h.g = state.NewGame(names(2))
			src := counterSource(t, &h.fakeHost, 0)
			target := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
			// Defined$ TriggeredSpellAbility reads the trigger's remembered
			// object, which is how these SAs name the spell being countered.
			Resolve(h, &Ctx{Source: src, Controller: 0,
				Remembered: []state.Target{{Obj: target.ID}}}, sa)

			if h.asked != nil {
				t.Fatalf("%s (UnlessCost$ %q, UnlessSwitched$ True) posed the backwards pay ask: %+v",
					name, sa.Params["UnlessCost"], h.asked)
			}
			if target.Zone != state.ZGraveyard {
				t.Fatalf("%s: countered spell zone = %s, want Graveyard (switched shapes counter unconditionally)",
					name, target.Zone)
			}
			if got := counterMoves(&h.fakeHost, target.ID, state.ZGraveyard); got != 1 {
				t.Fatalf("%s: move-to-graveyard count = %d, want exactly 1", name, got)
			}
		})
	}
}

// TestCounterUnlessCostPromptNeverLeaksScriptSyntax pins the rendering of the
// ask on REAL compiled corpus SAs whose UnlessCost$ is not a mana cost.
// decision.Decision crosses to every seat, human ones included, so the prompt
// and the option labels must not carry raw Forge script -- neither a bare
// SVar name (Mausoleum Wanderer's UnlessCost$ X, whose value the engine never
// reads) nor a bracket form (Reality Smasher's Discard<1/Card>). Both cards
// ship in repo decks (mono-blue-tempo / uw-tempo and eldrazi-stompy).
//
// This FAILS without unlessCostLabel: the unmodified tree interpolates the
// raw UnlessCost$ into both strings. Verified by running it on a tree without
// the helper, not asserted. Plain mana costs are unaffected -- Mana Leak's
// "Pay 3" is pinned by TestCounterUnlessCostAsksTheCounteredSpellsController
// above, and that is what keeps the acceptance chain heads still.
func TestCounterUnlessCostPromptNeverLeaksScriptSyntax(t *testing.T) {
	for _, tc := range []struct{ card, cost string }{
		{"Mausoleum Wanderer", "X"},
		{"Reality Smasher", "Discard<1/Card>"},
	} {
		t.Run(tc.card, func(t *testing.T) {
			sa := corpusUnlessCounterSA(t, tc.card, nil)
			if got := strings.TrimSpace(sa.Params["UnlessCost"]); got != tc.cost {
				t.Fatalf("%s UnlessCost$ = %q, want %q -- the corpus changed under this test", tc.card, got, tc.cost)
			}
			h := &askHost{}
			h.g = state.NewGame(names(2))
			src := counterSource(t, &h.fakeHost, 0)
			target := spellOnStack(t, &h.fakeHost, "A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3", 1)
			Resolve(h, &Ctx{Source: src, Controller: 0,
				Targets:    []state.Target{{Obj: target.ID}},
				Remembered: []state.Target{{Obj: target.ID}}}, sa)

			if h.asked == nil {
				t.Fatalf("%s posed no pay decision for UnlessCost$ %s", tc.card, tc.cost)
			}
			shown := []string{h.asked.Prompt}
			for _, o := range h.asked.Options {
				shown = append(shown, o.Label)
			}
			for _, s := range shown {
				if strings.Contains(s, tc.cost) {
					t.Fatalf("%s: %q leaks the raw script cost %q to the seat", tc.card, s, tc.cost)
				}
			}
			if h.asked.Prompt != "Pay the cost to save the spell, or decline" {
				t.Fatalf("%s prompt = %q", tc.card, h.asked.Prompt)
			}
			if h.asked.Options[0].Label != "Pay the cost — don't counter" {
				t.Fatalf("%s pay label = %q", tc.card, h.asked.Options[0].Label)
			}
		})
	}
}
