//go:build dirac_mp

package oracle

import "testing"

// TestFourSeatChosenOpponentLockstep plays four-seat pods of the cards that
// choose an opponent as they enter (chosenOppDecks: two such decks and two of
// lands only): the controller chooses one of three opponents (gorge
// effChoosePlayer's "player" options in AliveFrom order; the kernel's
// ChGenPlayer ask, posed with the permanent on the battlefield), and only that
// player's upkeep triggers The Rack and Black Vise, only that hand sizes
// Entropic Specter and Nyxathid. No divergence, no stall, the triggers hit,
// every card entered, and some pick is not the first opponent listed.
func TestFourSeatChosenOpponentLockstep(t *testing.T) {
	y, acts := playChosenOpp(t, 4, 6)
	checkChosenOpp(t, y)
	code, _ := chooseCode("player")
	pick := ActChoose + CanonKind(code)
	asks, farther := 0, 0
	for _, a := range acts {
		if a.Kind() != pick {
			continue
		}
		asks++
		// seats 0 and 2 choose, and each lists the other second (AliveFrom
		// order: 1 2 3 from seat 0, 3 0 1 from seat 2)
		if p := a.Tgt() - RefPlayer; p != 1 && p != 3 {
			farther++
		}
	}
	if asks == 0 || farther == 0 {
		t.Fatalf("opponent asks %d, past the first listed %d", asks, farther)
	}
	t.Logf("opponent asks %d, past the first listed %d", asks, farther)
}
