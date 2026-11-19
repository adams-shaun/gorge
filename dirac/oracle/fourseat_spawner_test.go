package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestFourSeatSpawnerReferentsLockstep plays four-seat pods of the reflexive
// triggers a card's own combat damage or attack spawns. The reflexive body's
// Spawner> referents name the spawner's seat (CR 603.7c): Kappa Tech-Wrecker's
// exile target is an artifact or enchantment the damaged player controls, and
// Ace, Fearless Rebel's / Auron, Venerated Guardian's target is a creature the
// attacked player controls. Reading the spawner's player as any non-active
// opponent (the two-seat reduction) widens the ask and some seed diverges.
func TestFourSeatSpawnerReferentsLockstep(t *testing.T) {
	// Kappa: the damaged seat's artifact (Meekstone), never the other
	// opponent's. The mutation reads PredTrigPlayerCtrl as any non-active
	// player, the old OppCtrl lowering.
	t.Run("kappa-damaged-seat", func(t *testing.T) {
		kappa := deckOf(10, "Kappa Tech-Wrecker", deckOf(30, "Forest")...)
		meek := deckOf(12, "Meekstone", deckOf(28, "Island")...)
		play := fourSeatAtkPlay(t, "dr-mbc-aabb", [4][]string{kappa, meek, kappa, meek})
		const seeds = 8
		fourSeatAtkClean(t, play, seeds)
		if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugTrigPlayerNextSeat) {
			t.Fatal("reading the damaged seat as any non-active player changed no seed")
		}
	})

	// Ace: the attacked seat's creature, never another opponent's. The
	// mutation reads TriggeredDefendingPlayer as the seat after the active
	// one, the two-seat reduction.
	t.Run("ace-defending-seat", func(t *testing.T) {
		ace := deckOf(8, "Ace, Fearless Rebel", deckOf(10, "Greatsword", deckOf(22, "Forest")...)...)
		creatures := deckOf(12, "Elvish Mystic", deckOf(28, "Forest")...)
		play := fourSeatAtkPlay(t, "tail-48", [4][]string{ace, creatures, ace, creatures})
		const seeds = 8
		fourSeatAtkClean(t, play, seeds)
		if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugDefendingActiveOpp) {
			t.Fatal("reading the defending seat as the seat after the active one changed no seed")
		}
	})

	// Auron: the same defending-seat referent through a PutCounter's
	// ImmediateTrigger. Liliana's Steward's power 0 is always less than
	// Auron's.
	t.Run("auron-defending-seat", func(t *testing.T) {
		auron := deckOf(10, "Auron, Venerated Guardian", deckOf(30, "Plains")...)
		creatures := deckOf(12, "Liliana's Steward", deckOf(28, "Swamp")...)
		play := fourSeatAtkPlay(t, "tail-67", [4][]string{auron, creatures, auron, creatures})
		const seeds = 8
		fourSeatAtkClean(t, play, seeds)
		if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugDefendingActiveOpp) {
			t.Fatal("reading the defending seat as the seat after the active one changed no seed")
		}
	})
}
