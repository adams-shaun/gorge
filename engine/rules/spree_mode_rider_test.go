package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Spree mode's cost is priced with the mana rider the cast captured: with Rakdos,
// the Muscle's "mana of any type", {B} plus a mode's {2} is payable by three red, and
// the grant no longer finds the card once it is on the stack (CR 601.2b, 702.172a).
func TestSpreeModePricedWithTheCastsManaRider(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Insatiable Avarice")}, nil)
	card := moveCorpusCard(t, e, "Insatiable Avarice", 0, state.ZHand)
	pc := &pendingCast{player: 0, card: card, ability: -1, cost: ParseCost("B")}
	var pot state.Mana
	pot[state.MR] = 3
	if e.modeCostFeasible(pc, ParseCost("2"), pot) {
		t.Fatal("three red paid {B}{2} with no rider")
	}
	pc.mayPlayIgnoreType = true
	if !e.modeCostFeasible(pc, ParseCost("2"), pot) {
		t.Fatal("three red did not pay {B}{2} under mana of any type")
	}
}
