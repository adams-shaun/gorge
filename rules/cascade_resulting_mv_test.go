package rules

// Task cascade-resulting-mv (CR 702.85a): a cascade free cast must obey the
// strict "resulting spell's mana value is less than this spell's mana value"
// bound, including when the candidate itself has an {X} cost. The exile-until
// scan compares the candidate at its X=0 printed value while it sits in the
// library (CR 202.3b), but the RESULTING spell's value counts the X announced
// at casting (CR 202.3e). Real corpus pair: Bloodbraid Elf (cascade source,
// mana value 4) cascades into Villainous Wealth (ManaCost:X B G U, printed
// mana value 3 in the library) -- X=0 keeps the resulting value at 3 (legal),
// while X=1 makes it 4 and violates the strict bound.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// cascadeXValueIndex returns the option index whose announced X is want from
// the pending cascade X ask, asserting the ask's shape first.
func cascadeXValueIndex(t *testing.T, e *Engine, want int) int {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the cascade X announcement ask, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "x" && o.Amount == want {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no X=%d option in the ask: %+v", want, d.Options)
	}
	return idx
}

// cascadeIntoWealth sets up Bloodbraid Elf casting into Villainous Wealth and
// returns once the free-cast election is pending, with the shared
// preconditions asserted (the source on the stack, the candidate found and
// exiled from the library at its X=0 value). It returns the engine, its
// config, the Elf and Wealth object ids, and the election option index.
func cascadeIntoWealth(t *testing.T, seed uint64) (*Engine, Config, state.ObjID, state.ObjID, int) {
	t.Helper()
	e, cfg := cascadeTestEngine(t, seed, "Bloodbraid Elf", []string{"Forest", "Villainous Wealth"}, nil)
	lib := e.G.Zone(state.ZLibrary, 0)
	forestID, wealthID := lib[0], lib[1]
	if o := e.G.Obj(wealthID); o == nil || o.Face() == nil || o.Face().Name != "Villainous Wealth" {
		t.Fatalf("precondition: window[1] = %v, want Villainous Wealth", e.G.Obj(wealthID))
	}
	if o := e.G.Obj(wealthID); o.Zone != state.ZLibrary {
		t.Fatalf("precondition: Villainous Wealth in %s, want the library before the cascade scan", o.Zone)
	}
	if mv := e.G.Obj(wealthID).Face().Cmc(); mv != 3 {
		t.Fatalf("precondition: Villainous Wealth printed mana value = %d, want 3 (X counts 0 in the library)", mv)
	}
	elfID := searchMoveByName(t, e, "Bloodbraid Elf", state.ZHand)
	if mv := e.G.Obj(elfID).Face().Cmc(); mv != 4 {
		t.Fatalf("precondition: Bloodbraid Elf printed mana value = %d, want 4", mv)
	}
	addMana(t, e, 0, "GGRR")
	castFixture(t, e, elfID, -1)
	// The cascade source is still on the stack while its trigger resolves:
	// its on-stack mana value is the bound (CR 202.3b/702.85a).
	if o := e.G.Obj(elfID); o == nil || o.Zone != state.ZStack || o.X != 0 {
		t.Fatalf("precondition: cascade source zone=%v X=%v, want stack with X=0", o, o)
	}
	idx := cascadeElection(t, e, "Villainous Wealth")
	// The scan found the candidate at its X=0 value and exiled it: the
	// printed X=0 value qualifies (3 < 4), which is what makes the X=1
	// resulting value (4, not less than 4) the thing under test.
	if o := e.G.Obj(wealthID); o.Zone != state.ZExile {
		t.Fatalf("precondition: found candidate in %s, want exile (found at X=0)", o.Zone)
	}
	if o := e.G.Obj(forestID); o.Zone != state.ZLibrary {
		t.Fatalf("precondition: the unmatched land in %s, want already bottomed", o.Zone)
	}
	return e, cfg, elfID, wealthID, idx
}

// TestCascadeFreeCastXMustRemainBelowCascadeManaValue pins CR 702.85a's strict
// inequality across the candidate's announced {X}. Accepting the election and
// announcing X=0 (resulting mana value 3 < 4) casts Villainous Wealth for
// free; announcing X=1 (resulting mana value 4, not less than 4) must be
// rejected at CR 601.2e and the card returned to the library bottom by the
// cascade tail.
func TestCascadeFreeCastXMustRemainBelowCascadeManaValue(t *testing.T) {
	t.Run("below the bound casts", func(t *testing.T) {
		e, cfg, elfID, wealthID, idx := cascadeIntoWealth(t, 9231)
		submitChoices(t, e, idx)
		// CR 107.3c: the free cast still announces X. Precondition: the
		// announced value's resulting mana value is genuinely below the
		// source's (0 + 3 = 3 < 4).
		xIdx := cascadeXValueIndex(t, e, 0)
		submitChoices(t, e, xIdx)
		// CR 601.2c: the sorcery targets an opponent; answer the target ask
		// (it precedes the CR 601.2e recheck inside payCast).
		answerCascadeTargetAsk(t, e)
		passUntilStackEmpty(t, e, 60)
		if !hasNote(e, "not less than the cascade spell") {
			// no abort: the legal cast went through
		} else {
			t.Fatalf("the X=0 cast was rejected although 3 < 4")
		}
		if n := putOnStackCount(e, wealthID); n != 1 {
			t.Fatalf("legal free cast pushed the candidate %d time(s), want 1", n)
		}
		if got := e.G.Obj(wealthID).Zone; got == state.ZLibrary {
			t.Fatalf("the legally cast candidate is in %s, want it to have resolved", got)
		}
		if got := e.G.Obj(elfID).Zone; got != state.ZBattlefield {
			t.Fatalf("resolved cascade source in %s, want the battlefield", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("at the bound is reversed", func(t *testing.T) {
		e, cfg, elfID, wealthID, idx := cascadeIntoWealth(t, 9232)
		// Precondition: X=1 makes the resulting mana value exactly the
		// source's (1 + 3 = 4), which the strict CR 702.85a bound forbids.
		submitChoices(t, e, idx)
		xIdx := cascadeXValueIndex(t, e, 1)
		submitChoices(t, e, xIdx)
		// CR 601.2c precedes the CR 601.2e recheck: the target ask still
		// appears before payCast runs recheckIllegal and reverses the cast.
		answerCascadeTargetAsk(t, e)
		passUntilStackEmpty(t, e, 60)
		if !hasNote(e, "the resulting spell's mana value is not less than the cascade spell's") {
			t.Fatal("no CR 702.85a rejection Note: the X=1 cast proceeded although 4 is not less than 4")
		}
		// CR 601.2a pushed the proposal before CR 601.2e reversed it (CR
		// 733.1): exactly one push, and the object is back off the stack.
		if n := putOnStackCount(e, wealthID); n != 1 {
			t.Fatalf("rejected candidate pushed %d time(s), want 1 (pushed then reversed)", n)
		}
		// The cascade tail bottoms the still-uncast candidate (CR 702.85a's
		// "put the exiled cards on the bottom").
		if got := e.G.Obj(wealthID).Zone; got != state.ZLibrary {
			t.Fatalf("rejected candidate in %s, want the library (bottomed by the cascade tail)", got)
		}
		if n := movedTo(t, e, wealthID, state.ZExile, state.ZLibrary); n != 1 {
			t.Fatalf("the rejected candidate returned to the library %d time(s), want 1", n)
		}
		if got := e.G.Obj(elfID).Zone; got != state.ZBattlefield {
			t.Fatalf("the cascade source did not resolve: %s", got)
		}
		replayCheck(t, e, cfg)
	})
}

// answerCascadeTargetAsk answers the CR 601.2c target ask a cascade free cast
// of Villainous Wealth poses before the CR 601.2e recheck (inside payCast). It
// asserts the ask is present so a cast that silently stopped asking targets
// cannot pass the test vacuously.
func answerCascadeTargetAsk(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) == 0 {
		t.Fatalf("want the free-cast target ask, got %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
}

// TestCascadeXCandidateCrossesTheBound is a guard against a vacuous version
// of the test above: it asserts the X option set spans the strict bound and
// that the candidate's printed value plus the announced value crosses it, so
// the rejection the paired test pins is reachable rather than accidental.
func TestCascadeXCandidateCrossesTheBound(t *testing.T) {
	e, _, _, wealthID, idx := cascadeIntoWealth(t, 9233)
	submitChoices(t, e, idx)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the X ask, got %+v", d)
	}
	// The option set must include both a legal (0) and the first illegal (1)
	// value, so the strict boundary is genuinely reachable.
	amounts := map[int]bool{}
	for _, o := range d.Options {
		if o.Kind == "x" {
			amounts[o.Amount] = true
		}
	}
	if !amounts[0] || !amounts[1] {
		t.Fatalf("X ask %+v must offer both X=0 (legal) and X=1 (the boundary)", d.Options)
	}
	printed := e.G.Obj(wealthID).Face().Cmc() // X counts 0
	elfMV := int32(4)
	if printed >= elfMV {
		t.Fatalf("printed value %d is not below the source %d: nothing to announce", printed, elfMV)
	}
	if printed+1 < elfMV {
		t.Fatalf("X=1 gives %d, still below %d: the boundary is not X=1", printed+1, elfMV)
	}
	// Drain the ask so the engine does not sit parked.
	submitChoices(t, e, cascadeXValueIndex(t, e, 0))
}

// TestCascadeNonXFreeCastDoesNotAskX is the scope guard for cascadeXAsk: only
// a cascade candidate with its own printed {X} gets the extra announcement.
// A non-{X} candidate (Grizzly Bears) cascades and casts with no X ask.
func TestCascadeNonXFreeCastDoesNotAskX(t *testing.T) {
	e, cfg := cascadeTestEngine(t, 9234, "Bloodbraid Elf", []string{"Forest", "Grizzly Bears"}, nil)
	elfID := searchMoveByName(t, e, "Bloodbraid Elf", state.ZHand)
	addMana(t, e, 0, "GGRR")
	castFixture(t, e, elfID, -1)
	idx := cascadeElection(t, e, "Grizzly Bears")
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		for _, o := range d.Options {
			if o.Kind == "x" {
				t.Fatalf("a non-{X} cascade candidate was offered an X ask: %+v", d.Options)
			}
		}
	}
	passUntilStackEmpty(t, e, 40)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
			replayCheck(t, e, cfg)
			return
		}
	}
	t.Fatal("the non-{X} candidate never resolved")
}
