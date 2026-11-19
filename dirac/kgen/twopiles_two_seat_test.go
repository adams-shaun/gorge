package kgen

import "testing"

// The Fact or Fiction-shaped script the pin lowers: a
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

// The same split with the opponent choosing the pile and you separating
// (Steam Augury's shape), inline.
const twopilesOppChoosesScript = `Name:Test Augury
ManaCost:2 U R
Types:Instant
A:SP$ PeekAndReveal | NoPeek$ True | RememberRevealed$ True | PeekAmount$ 5 | SubAbility$ DBTwoPiles
SVar:DBTwoPiles:DB$ TwoPiles | DefinedCards$ Remembered | Defined$ You | Separator$ You | Chooser$ Opponent | ChosenPile$ DBToHand | UnchosenPile$ DBToYard
SVar:DBToHand:DB$ ChangeZone | Destination$ Hand | Defined$ Remembered | Origin$ Library
SVar:DBToYard:DB$ ChangeZone | Destination$ Graveyard | Defined$ Remembered | Origin$ Library
Oracle:Reveal the top five of your library. Separate them into two piles. An opponent chooses one; put it into your hand and the other into your graveyard.
`

// TestTwoPilesOpponentSeparatorAnySeats: either TwoPiles shape that names an
// opponent (Separator$ Opponent, or Chooser$ Opponent) is CR 801.5b at three
// or more seats -- the spell controller picks WHICH opponent -- and the
// kernel asks it (act_twopiles.go pilesOppAsk), so both lower with no
// two-seat marker.
func TestTwoPilesOpponentSeparatorAnySeats(t *testing.T) {
	if r := lowerScript(t, "Test Fof", twopilesFofScript); len(r) != 0 {
		t.Fatalf("the FoF shape refused: %v", r)
	}
	if p := lowerScriptCard(t, "Test Fof", twopilesFofScript); p.TwoSeat {
		t.Fatalf("the FoF-shaped TwoPiles lowered two-seat: %v", p.TwoSeatWhy)
	}
	if r := lowerScript(t, "Test Augury", twopilesOppChoosesScript); len(r) != 0 {
		t.Fatalf("the opponent-chooses shape refused: %v", r)
	}
	if p := lowerScriptCard(t, "Test Augury", twopilesOppChoosesScript); p.TwoSeat {
		t.Fatalf("the opponent-chooses TwoPiles lowered two-seat: %v", p.TwoSeatWhy)
	}
}
