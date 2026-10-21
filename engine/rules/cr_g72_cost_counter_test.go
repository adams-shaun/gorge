package rules

// Doubling Season: "If an EFFECT would put one or more counters on a
// permanent you control, it puts twice that many of those counters on that
// permanent instead." A cost is not an effect: the official ruling (and
// Forge's EffectOnly$ / XMage's isEffect=false for PayLoyaltyCost) leaves a
// planeswalker's [+N] loyalty cost and every other AddCounter<N/...> cost
// undoubled. The EffectOnly$ gate (replacement_match.go) reads "no resolving
// spell or ability" as actionCause() == 0; a cost paid while the ability
// being activated -- or any other object -- is on the stack must still read
// as a cost.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDoublingSeasonLeavesLoyaltyCostUndoubled: Ajani Goldmane (loyalty 4)
// enters, then Doubling Season; Ajani's [+1] makes it 5, not 6.
func TestDoublingSeasonLeavesLoyaltyCostUndoubled(t *testing.T) {
	t.Parallel()
	ajani := tokenReplCorpusCard(t, "Ajani Goldmane")
	ds := tokenReplCorpusCard(t, "Doubling Season")
	e, cfg := tokenReplGame(t, 97, ajani, ds)
	aj := moveSeededCard(t, e, 0, ajani, state.ZBattlefield)
	moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	if got := e.G.Obj(aj).Counter("LOYALTY"); got != 4 {
		t.Fatalf("precondition: Ajani entered with %d loyalty before Doubling Season, want 4", got)
	}
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, aj, 0).Index)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(aj).Counter("LOYALTY"); got != 5 {
		t.Fatalf("Doubling Season doubled Ajani's [+1] loyalty COST: 4 -> %d, want 5 (a cost is not an effect)", got)
	}
	replayCheck(t, e, cfg)
}

// TestDoublingSeasonLeavesCounterCostUndoubledInResponse: an authored
// artifact whose cost puts a CHARGE counter on itself is activated, then
// activated again in response with its first activation on the stack. Both
// costs are costs: 2 counters, not 3.
func TestDoublingSeasonLeavesCounterCostUndoubledInResponse(t *testing.T) {
	t.Parallel()
	src := card(t, "Name:Charge Battery\nTypes:Artifact\n"+
		"A:AB$ GainLife | Cost$ AddCounter<1/CHARGE> | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n")
	ds := tokenReplCorpusCard(t, "Doubling Season")
	e, cfg := tokenReplGame(t, 98, src, ds)
	bat := moveSeededCard(t, e, 0, src, state.ZBattlefield)
	moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, bat, 0).Index)
	if n := len(e.G.Stack); n != 1 {
		t.Fatalf("precondition: stack depth %d after the first activation, want 1", n)
	}
	submitChoices(t, e, abilityOption(t, e, bat, 0).Index)
	if got := e.G.Obj(bat).Counter("CHARGE"); got != 2 {
		t.Fatalf("Doubling Season doubled an AddCounter cost paid with another ability on the stack: %d CHARGE counters, want 2", got)
	}
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}
