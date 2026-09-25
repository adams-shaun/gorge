package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

const (
	cwFeeGreenSource = "Name:FeeGreen\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ 1 T | Produced$ G | Amount$ 2 | SpellDescription$ Add {G}{G}.\nOracle:x\n"
	cwFreeColorlessSource = "Name:FreeColorless\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nOracle:x\n"
	cwTwoFeeGreenSource = "Name:TwoFeeGreen\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ 2 T | Produced$ G | SpellDescription$ Add {G}.\nOracle:x\n"
)

// TestCastWindowGenericFeeConsumesTheRealPool ensures an activation fee is
// deducted before checking the spell: {C} pays the {1} fee, leaving {G}{G},
// which cannot pay the spell's required {C}{G}.
func TestCastWindowGenericFeeConsumesTheRealPool(t *testing.T) {
	b := equipWindowGame(t, 718, cwFeeGreenSource)
	e := b.engine
	source := b.byName["FeeGreen"]
	if source == 0 || e.G.Obj(source).Zone != state.ZBattlefield || e.G.Obj(source).Tapped {
		t.Fatalf("FeeGreen must be an untapped battlefield source: %+v", e.G.Obj(source))
	}
	addMana(t, e, 0, "C")
	if got := e.G.Players[0].Pool; got[state.MC] != 1 || got.Total() != 1 {
		t.Fatalf("precondition pool = %v, want exactly {C}", got)
	}
	units := e.castWindowUnitsForSourceTest(source, "G", 1)
	units[0].alts[0].amt = 2
	if e.castWindowReachable(0, ParseCost("C G"), e.G.Players[0].Pool, state.Mana{}, [7]state.Mana{}, 20, nil, units) {
		t.Fatal("probe claims {C} pays a {1}: add {G}{G} activation followed by a {C}{G} spell")
	}
	if !e.untappedManaSource(0, source) {
		t.Fatal("precondition: the real cast window does not offer FeeGreen")
	}
}

// TestCastWindowFeeFundingMayExceedSpellPipCount ensures a free mana ability
// can fund a later {2} activation even when the spell itself has only one pip.
func TestCastWindowFeeFundingMayExceedSpellPipCount(t *testing.T) {
	b := equipWindowGame(t, 719, cwFreeColorlessSource, cwTwoFeeGreenSource)
	e := b.engine
	freeID, paidID := b.byName["FreeColorless"], b.byName["TwoFeeGreen"]
	for _, id := range []state.ObjID{freeID, paidID} {
		if id == 0 || e.G.Obj(id).Zone != state.ZBattlefield || e.G.Obj(id).Tapped {
			t.Fatalf("source %d must be untapped on battlefield: %+v", id, e.G.Obj(id))
		}
	}
	addMana(t, e, 0, "C")
	if got := e.G.Players[0].Pool.Total(); got != 1 {
		t.Fatalf("precondition pool total = %d, want 1", got)
	}
	units := append(e.castWindowUnitsForSourceTest(freeID, "C", 0), e.castWindowUnitsForSourceTest(paidID, "G", 2)...)
	if !e.castWindowReachable(0, ParseCost("G"), e.G.Players[0].Pool, state.Mana{}, [7]state.Mana{}, 20, nil, units) {
		t.Fatal("probe missed free {C}, then pay {2}, then produce {G}; search cannot be bounded by the spell's one pip")
	}
	if !e.untappedManaSource(0, freeID) {
		t.Fatal("precondition: the real cast window does not initially offer the free source")
	}
	// This is the pool state after the free source's {C} has resolved. The
	// live payment gate must then expose the paid source for the {2} fee.
	addMana(t, e, 0, "C")
	if !e.untappedManaSource(0, paidID) {
		t.Fatal("real cast-window payment gate does not expose the {2} source after the free {C}")
	}
}

// castWindowUnitsForSourceTest supplies a single known production alternative
// to isolate activation-fee reachability from the corpus census walk.
func (e *Engine) castWindowUnitsForSourceTest(id state.ObjID, symbol string, fee int32) []windowManaUnit {
	counts := [6]int32{}
	for i, c := range []byte("WUBRGC") {
		if len(symbol) == 1 && symbol[0] == c {
			counts[i] = 1
		}
	}
	return []windowManaUnit{{id: id, alts: []windowManaAlt{{counts: counts, amt: 1, costGeneric: fee}}}}
}
