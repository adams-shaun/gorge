package kgen

import (
	"slices"
	"testing"
)

// The Fact or Fiction-shaped script the two-seat pin lowers: a
// PeekAndReveal remembering the revealed five chained into a TwoPiles whose
// Separator$ names Opponent, exactly the corpus carrier's shape, inline (no
// Forge script text is committed).
const twopilesFofScript = `Name:Test Fof
ManaCost:3 U
Types:Instant
A:SP$ PeekAndReveal | NoPeek$ True | RememberRevealed$ True | PeekAmount$ 5 | SubAbility$ DBTwoPiles
SVar:DBTwoPiles:DB$ TwoPiles | DefinedCards$ Remembered | Defined$ You | Separator$ Opponent | ChosenPile$ DBToHand | UnchosenPile$ DBToYard
SVar:DBToHand:DB$ ChangeZone | Destination$ Hand | Defined$ Remembered | Origin$ Library
SVar:DBToYard:DB$ ChangeZone | Destination$ Graveyard | Defined$ Remembered | Origin$ Library
Oracle:Reveal the top five of your library. An opponent splits them into two piles. One pile goes to your hand, the other to your graveyard.
`

// TestTwoPilesOpponentIsTwoSeat: a TwoPiles whose Separator$/Chooser$ names
// an opponent is CR 801.5b at three or more seats -- the spell controller
// picks WHICH opponent -- and dirac reads that opponent as the next live
// seat, so the carrier lowers unrefused but carries the two-seat marker and
// its reason (a 3+-seat dirac game refuses it on StallTwoSeat).
func TestTwoPilesOpponentIsTwoSeat(t *testing.T) {
	if r := lowerScript(t, "Test Fof", twopilesFofScript); len(r) != 0 {
		t.Fatalf("the FoF shape refused: %v", r)
	}
	p := lowerScriptCard(t, "Test Fof", twopilesFofScript)
	const want = "TwoPiles: an opponent separates or chooses, read as the next live seat"
	if !p.TwoSeat {
		t.Fatalf("the FoF-shaped TwoPiles lowered without the two-seat marker (why: %v)", p.TwoSeatWhy)
	}
	if !slices.Contains(p.TwoSeatWhy, want) {
		t.Fatalf("TwoSeatWhy = %v, want it to contain %q", p.TwoSeatWhy, want)
	}
}
