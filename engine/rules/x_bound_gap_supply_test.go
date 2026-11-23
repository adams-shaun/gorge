package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestXBoundSpellCountsStorageGapSupply pins CR 107.3i, 601.2b, 601.2h and
// 605.3b: Spell Burst's X=4 needs {4}{U}, five mana. The seat's sources are
// a floating {U}, two one-mana gems and a storage land whose "remove counters: add {R} each"
// the payment planner does not price (a census gap). The land taps once, so
// one counter makes one {R} and the supply is four: the plain cast must be
// withheld. The gap used to count as an unbounded gain and left the count
// proof unrun (decision 370 of tail-26 seed 306680, pair 17).
func TestXBoundSpellCountsStorageGapSupply(t *testing.T) {
	t.Parallel()
	const store = "Name:Test Store\nManaCost:0\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ R | Amount$ X | SpellDescription$ Add {R} for each counter removed.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const gem = "Name:One Gem\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME/this artifact> | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
	const burst = "Name:Test Burst\nManaCost:X U\nTypes:Instant\n" +
		"A:SP$ Destroy | ValidTgts$ Creature.cmcEQX | TgtPrompt$ Pick a creature | SpellDescription$ Destroy a creature with mana value X.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const brute = "Name:Four Brute\nManaCost:2 B R\nTypes:Creature Cyclops\nPT:4/4\nOracle:x\n"
	e := handEngine(t, card(t, burst))
	id := e.G.Zone(state.ZHand, 0)[0]
	storeID := onBoardReady(t, e, 0, store)
	for range 2 {
		onBoard(t, e, 0, gem)
	}
	onBoard(t, e, 1, brute)
	e.G.Players[0].Pool[state.MU] = 1
	e.emit(events.Event{Kind: events.CounterChange, Obj: storeID, Counter: "STORAGE", Amount: 1})
	if xBoundOffered(e, "cast", id) {
		t.Fatal("offered X=4 ({4}{U}) with four mana at most")
	}
	// A second counter makes the fifth mana, so the same board offers the
	// cast: the assertion above fails on the supply and not on the fixture.
	e.emit(events.Event{Kind: events.CounterChange, Obj: storeID, Counter: "STORAGE", Amount: 1})
	if !xBoundOffered(e, "cast", id) {
		t.Fatal("not offered with five mana for {4}{U}")
	}
}
