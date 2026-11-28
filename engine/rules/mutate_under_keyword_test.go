package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins the keyword half of CR 702.140e ("A mutated permanent has all
// abilities of each card and token that represents it"): a keyword printed on a
// card beneath the top is the pile's, and the engine's granted-keyword trigger
// walks must read it as printed, since the under card's own expanded trigger
// already fires for it.

// mutateKeywordCrownSrc is the mutating card: it goes on TOP and prints none of
// the under card's keywords.
const mutateKeywordCrownSrc = "Name:Keyword Crown\nManaCost:2 G\nTypes:Creature Beast\nPT:3/3\n" +
	"K:Mutate:1 G\nOracle:x\n"

// mutatedKeywordPile casts the Crown on top of a creature printing under's
// lines and returns the pile.
func mutatedKeywordPile(t *testing.T, seed uint64, under string) (*Engine, Config, state.ObjID) {
	t.Helper()
	bearer := card(t, "Name:Keyword Bearer\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n"+under+"Oracle:x\n")
	crown := card(t, mutateKeywordCrownSrc)
	e, cfg := tokenReplGame(t, seed, crown, bearer)
	crownID := moveSeededCard(t, e, 0, crown, state.ZHand)
	pile := moveSeededCard(t, e, 0, bearer, state.ZBattlefield)
	addMana(t, e, 0, "GG") // the mutate cost {1}{G}
	mutateCastOnto(t, e, mutatedCastOption(t, e, crownID), pile, true)
	mutateDrain(t, e, 40)
	o := e.G.Obj(pile)
	if o == nil || o.Face() == nil || o.Face().Name != "Keyword Crown" || len(o.MergedCards) != 1 {
		t.Fatalf("precondition: want the Crown on top of the Bearer, got %+v", o)
	}
	if n := len(e.pendingTriggers); n != 0 || len(e.G.Stack) != 0 {
		t.Fatalf("precondition: %d pending triggers and %d stack entries after the merge", n, len(e.G.Stack))
	}
	return e, cfg, pile
}

// queuedSince counts the triggers sourced at id that are waiting in the order
// queue or were pushed after log position from.
func queuedSince(e *Engine, id state.ObjID, from int) int {
	n := 0
	for _, pt := range e.pendingTriggers {
		if pt.Source == id {
			n++
		}
	}
	for _, ev := range e.L.Events[from:] {
		switch ev.Kind {
		case events.TriggerPush, events.KeywordTriggerPush, events.MergedTriggerPush, events.GrantTriggerPush:
			if ev.Obj == id {
				n++
			}
		}
	}
	return n
}

// TestMutatedPileUnderCardKeywordsAreDerived: the pile's derived keywords hold
// the under card's lines, once each, beside the top card's own.
func TestMutatedPileUnderCardKeywordsAreDerived(t *testing.T) {
	t.Parallel()
	e, cfg, pile := mutatedKeywordPile(t, 431, "K:Flying\nK:Ward:2\n")
	top := e.G.Obj(pile).Face()
	if top.HasKeyword("Flying") || top.HasKeyword("Ward") {
		t.Fatalf("precondition: the top card prints the keyword under test: %q", top.Keywords)
	}
	kws := e.Derived(pile).Keywords
	for _, want := range []string{"Flying", "Ward:2", "Mutate:1 G"} {
		n := 0
		for _, k := range kws {
			if k == want {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("derived keywords %q hold %q %d times, want once", kws, want, n)
		}
	}
	// The filter-side keyword read and the single-head precheck agree.
	if !e.hasKeywordH(pile, kwHeadOf("Ward")) {
		t.Fatalf("hasKeywordH misses the under card's ward: %q", kws)
	}
	if !e.matchesSpecFrom("Creature.withFlying", pile, 0, 0) {
		t.Fatal("Creature.withFlying does not match the pile with flying beneath its top")
	}
	replayCheck(t, e, cfg)
}

// TestMutatedPileUnderCardKeywordLeavesWithThePile: the split gives the keyword
// back to its own card only.
func TestMutatedPileUnderCardKeywordLeavesWithThePile(t *testing.T) {
	t.Parallel()
	e, cfg, pile := mutatedKeywordPile(t, 432, "K:Flying\n")
	if !slices.Contains(e.Derived(pile).Keywords, "Flying") {
		t.Fatalf("precondition: the pile has flying: %q", e.Derived(pile).Keywords)
	}
	under := e.G.Obj(pile).MergedCards[0].Obj
	e.emit(events.Event{Kind: events.MoveZone, Obj: pile, From: state.ZBattlefield, To: state.ZHand})
	if o := e.G.Obj(pile); o.Zone != state.ZHand || len(o.MergedCards) != 0 {
		t.Fatalf("precondition: the pile split to hand: %+v", o)
	}
	if kws := e.Derived(pile).Keywords; slices.Contains(kws, "Flying") {
		t.Fatalf("the top card kept the under card's flying after the split: %q", kws)
	}
	if kws := e.Derived(under).Keywords; !slices.Contains(kws, "Flying") {
		t.Fatalf("the under card lost its own flying: %q", kws)
	}
	replayCheck(t, e, cfg)
}

// TestMutatedPileUnderCardAttackKeywordsTriggerOnce: each attack keyword printed
// beneath the top triggers once, from the under card's own expansion; the
// granted-keyword walks add no second instance.
func TestMutatedPileUnderCardAttackKeywordsTriggerOnce(t *testing.T) {
	t.Parallel()
	for i, kw := range []string{"Exalted", "Dethrone", "Melee", "Firebending:1"} {
		t.Run(kw, func(t *testing.T) {
			t.Parallel()
			e, _, pile := mutatedKeywordPile(t, 433+uint64(i), "K:"+kw+"\n")
			if !slices.Contains(e.Derived(pile).Keywords, kw) {
				t.Fatalf("precondition: the pile does not derive %q: %q", kw, e.Derived(pile).Keywords)
			}
			from := len(e.L.Events)
			declareAttackersOnly(t, e, 0, 1, pile)
			if got := queuedSince(e, pile, from); got != 1 {
				t.Fatalf("%s beneath the top queued %d triggers on the attack, want 1", kw, got)
			}
		})
	}
}

// TestMutatedPileUnderCardTrainingTriggersOnce is the same rule for training,
// which needs a stronger attacker beside the pile (CR 702.149a).
func TestMutatedPileUnderCardTrainingTriggersOnce(t *testing.T) {
	t.Parallel()
	e, _, pile := mutatedKeywordPile(t, 441, "K:Training\n")
	big := putToken(t, e, 0, "Name:Big Ox\nManaCost:4 G\nTypes:Creature Ox\nPT:6/6\nOracle:x\n", state.ZBattlefield)
	if e.Power(big) <= e.Power(pile) {
		t.Fatalf("precondition: the Ox %d is not stronger than the pile %d", e.Power(big), e.Power(pile))
	}
	from := len(e.L.Events)
	declareAttackersOnly(t, e, 0, 1, pile, big)
	if got := queuedSince(e, pile, from); got != 1 {
		t.Fatalf("training beneath the top queued %d triggers on the attack, want 1", got)
	}
}

// TestMutatedPileUnderCardEventKeywordsTriggerOnce is the same rule for the
// keyword walks other events reach: ward as the pile is targeted, afflict as it
// is blocked and cumulative upkeep at its controller's upkeep. The block-mode
// walk reads only the top face's expansions, so an under card's afflict fires
// through the granted walk: still exactly once.
func TestMutatedPileUnderCardEventKeywordsTriggerOnce(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, kw string
		fire     func(e *Engine, pile state.ObjID)
	}{
		{"ward", "Ward:2", func(e *Engine, pile state.ObjID) {
			cause := e.G.Zone(state.ZLibrary, 1)[0]
			e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
			e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{pile}})
		}},
		{"afflict", "Afflict:1", func(e *Engine, pile state.ObjID) {
			blocker := onBoard(t, e, 1, mutateBearSrc)
			declareAttackersOnly(t, e, 0, 1, pile)
			e.pendingTriggers = nil
			e.emit(events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{pile, blocker}}})
		}},
		{"cumulative upkeep", "Cumulative upkeep:1", func(e *Engine, pile state.ObjID) {
			e.G.Active, e.G.Priority = 0, 0
			e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
		}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, pile := mutatedKeywordPile(t, 451+uint64(i), "K:"+tc.kw+"\n")
			if !slices.Contains(e.Derived(pile).Keywords, tc.kw) {
				t.Fatalf("precondition: the pile does not derive %q: %q", tc.kw, e.Derived(pile).Keywords)
			}
			from := len(e.L.Events)
			tc.fire(e, pile)
			if got := queuedSince(e, pile, from); got != 1 {
				t.Fatalf("%s beneath the top queued %d triggers, want 1", tc.kw, got)
			}
		})
	}
}
