package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestPaymentPlanReflectedExtraBesideCondSplit: River of Tears adds {U}, or
// {B} instead once its controller played a land this turn, and Mana Flare
// adds one mana of any type the tapped land produced (CR 605.1b, 106.12a) --
// the {B} branch's tap produces {B}{B}. With an untapped Plains beside it,
// {1}{B}{B} is payable ({B}{B} from the River, the Plains' {W}{W} or spare
// mana for the generic). Without the Flare the same cast stays unpayable: the
// bound's only {B} is the River's sub production, so the pair cannot pass
// vacuously.
func TestPaymentPlanReflectedExtraBesideCondSplit(t *testing.T) {
	t.Parallel()
	const spellSrc = "Name:Double Black Probe\nManaCost:1 B B\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

	// Beside Mana Flare: payable.
	e, _, spell := newFixtureDeck(t, 9921, spellSrc)
	toMain1(t, e)
	onBoardCard(t, e, 0, corpusCard(t, "Plains"))
	onBoardCard(t, e, 0, corpusCard(t, "River of Tears"))
	onBoardCard(t, e, 1, corpusCard(t, "Mana Flare"))
	e.emit(events.Event{Kind: events.LandPlayed, Player: 0})
	if n := e.G.Players[0].LandsPlayed; n < 1 {
		t.Fatalf("precondition: LandsPlayed = %d, want >= 1", n)
	}
	condSplitPayable(t, e, spell, "River of Tears after a land drop beside Mana Flare, {1}{B}{B} spell")

	// Without Mana Flare: unpayable (the bound's only {B} is one).
	e2, _, spell2 := newFixtureDeck(t, 9922, spellSrc)
	toMain1(t, e2)
	onBoardCard(t, e2, 0, corpusCard(t, "Plains"))
	onBoardCard(t, e2, 0, corpusCard(t, "River of Tears"))
	e2.emit(events.Event{Kind: events.LandPlayed, Player: 0})
	condSplitUnpayable(t, e2, spell2, "River of Tears after a land drop without Mana Flare, {1}{B}{B} spell")
}
