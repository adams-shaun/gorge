package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kardurMaximumPodDeck is the two-card pod the Kardur/Maximum Carnage
// MustAttack$ Player.Other divergence was measured on: a bare battlefield
// where every seat can attack, so the only requirement in play is the
// UntilYourNextTurn Effect the two cards deliver.
var kardurMaximumPodDeck = []cardset.Entry{
	{Name: "Kardur, Doomscourge", Count: 4},
	{Name: "Maximum Carnage", Count: 4},
	{Name: "Grizzly Bears", Count: 8},
	{Name: "Savannah Lions", Count: 8},
	{Name: "Plains", Count: 12},
	{Name: "Mountain", Count: 12},
	{Name: "Swamp", Count: 12},
}

// TestFourSeatKardurMaximumLockstep: four-seat games of the Kardur/Maximum
// Carnage pod play in lockstep. Their Effect's UntilYourNextTurn emblem marks
// every opponent's creatures as required attackers, so a seat's Done must
// stay withheld until those creatures attack; when the emblem's controller
// leaves the game the emblem ends at the boundary gorge froze at the
// departure (CR 800.4a), not at the loss.
func TestFourSeatKardurMaximumLockstep(t *testing.T) {
	var unused bool
	st := runPodClass(t, kardurMaximumPodDeck, 32, opponentSetWatch, &unused, false)
	t.Logf("%d games, %d divergences, %d stalls, %d watched decisions", st.games, st.divs, st.stalls, st.hits)
	if st.divs != 0 {
		t.Errorf("%d divergences", st.divs)
	}
	if st.stalls != 0 {
		t.Errorf("%d stalls", st.stalls)
	}
	if st.hits == 0 {
		t.Errorf("no watched multi-opponent decisions")
	}
}
