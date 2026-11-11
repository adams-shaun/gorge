//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestFourSeatDamagedPlayerCtrlLockstep plays four-seat FDN pods of Trygon
// Predator against two artifact and enchantment decks: its combat damage
// trigger targets an artifact or enchantment the damaged player controls
// (ControlledBy TriggeredTarget, kop.PredTrigPlayerCtrl), the trigger
// entry's trigger player at any seat count. No divergence and no stall;
// read as any player but the active one (dirac.DebugTrigPlayerNextSeat),
// the other opponent's permanents are offered too and some seed diverges.
func TestFourSeatDamagedPlayerCtrlLockstep(t *testing.T) {
	trygon := deckOf(16, "Trygon Predator", deckOf(12, "Forest", deckOf(12, "Island")...)...)
	stuff := deckOf(10, "Swiftfoot Boots", deckOf(8, "Pacifism", deckOf(22, "Plains")...)...)
	play := fourSeatAtkPlay(t, "std-fdn", [4][]string{trygon, stuff, trygon, stuff})
	const seeds = 8
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugTrigPlayerNextSeat) {
		t.Fatal("reading the damaged player as any non-active player changed no seed")
	}
}
