package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestPotentialRiverOfTearsAfterLandDropMakesNoBlue: River of Tears adds {U},
// or {B} instead once its controller played a land this turn (CR 605.1a), so
// after a land drop its root {U} is superseded and the seat's potential mana
// holds no {U}. Before the land drop it holds no {B}. The potential bound
// priced Kels, Fight Fixer's announced {U} as payable on this board and
// offered a "pay" the kernel's decline-only ask did not.
func TestPotentialRiverOfTearsAfterLandDropMakesNoBlue(t *testing.T) {
	t.Parallel()
	const spellSrc = "Name:Blue Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

	e, _, spell := newFixtureDeck(t, 9865, spellSrc)
	toMain1(t, e)
	onBoardCard(t, e, 0, corpusCard(t, "River of Tears"))
	e.emit(events.Event{Kind: events.LandPlayed, Player: 0})
	if n := e.G.Players[0].LandsPlayed; n < 1 {
		t.Fatalf("precondition: LandsPlayed = %d, want >= 1", n)
	}
	if pm := e.PotentialMana(0); pm[2] < 1 {
		t.Fatalf("precondition: potential mana %v after a land drop holds no {B}", pm)
	}
	if pm := e.PotentialMana(0); pm[1] != 0 {
		t.Errorf("River of Tears after a land drop: potential mana %v counts {U}, want none (its root is superseded)", pm)
	}
	condSplitUnpayable(t, e, spell, "River of Tears after a land drop, {U} spell")

	e2, _, spell2 := newFixtureDeck(t, 9866, spellSrc)
	toMain1(t, e2)
	onBoardCard(t, e2, 0, corpusCard(t, "River of Tears"))
	if pm := e2.PotentialMana(0); pm[1] < 1 {
		t.Fatalf("precondition: potential mana %v with no land played holds no {U}", pm)
	}
	condSplitPayable(t, e2, spell2, "River of Tears with no land played, {U} spell")
}
