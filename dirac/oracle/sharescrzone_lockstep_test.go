package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The "shares a creature type with the triggered card" lockstep at four seats
// (kop.W7SharesCrTrigObj read in a zone-change trigger's body): Heirloom
// Blade's dies trigger reveals until a creature card sharing a creature type
// with the equipped creature that died (CR 603.10a: it looks back at the
// card, gorge's TriggerCard), and Mana Echoes' enter trigger adds {C} for
// each creature its controller has sharing a type with the one entering.
// Bears, Giants and changelings (every creature type) fill the decks; Shock
// kills the equipped creatures. The expected answer at every decision is
// gorge's.
var sharesCrZoneDecks = [4][]string{
	podDeck(deckOf(4, "Heirloom Blade"), deckOf(8, "Grizzly Bears"), deckOf(4, "Woodland Changeling"), deckOf(6, "Hill Giant"),
		deckOf(4, "Shock"), deckOf(10, "Mountain"), deckOf(10, "Forest")),
	podDeck(deckOf(4, "Mana Echoes"), deckOf(8, "Grizzly Bears"), deckOf(4, "Woodland Changeling"), deckOf(6, "Hill Giant"),
		deckOf(4, "Shock"), deckOf(10, "Mountain"), deckOf(10, "Forest")),
	podDeck(deckOf(3, "Heirloom Blade"), deckOf(2, "Mana Echoes"), deckOf(8, "Hill Giant"), deckOf(6, "Grizzly Bears"),
		deckOf(2, "Woodland Changeling"), deckOf(5, "Shock"), deckOf(14, "Mountain"), deckOf(6, "Forest")),
	podDeck(deckOf(4, "Heirloom Blade"), deckOf(10, "Grizzly Bears"), deckOf(4, "Hill Giant"), deckOf(4, "Woodland Changeling"),
		deckOf(4, "Shock"), deckOf(8, "Mountain"), deckOf(12, "Forest")),
}

func TestSharesCrTrigCardPodLockstep(t *testing.T) {
	t.Parallel()
	podFixtureLockstep(t, "sharescrzone", sharesCrZoneDecks, 24, true, nil)
}

// TestSharesCrTrigCardPodLockstepMutation: with every W7SharesCrTrigObj read
// as "matches anything", some seed diverges.
func TestSharesCrTrigCardPodLockstepMutation(t *testing.T) {
	t.Parallel()
	n := podFixtureLockstep(t, "sharescrzone", sharesCrZoneDecks, 24, false, dropFilterPred(func(f *kop.Filter) bool {
		return f.Pred == kop.PredWord7 && f.PredN&0xff == kop.W7SharesCrTrigObj
	}))
	if n == 0 {
		t.Fatal("no seed diverged with the shares-a-creature-type filters dropped")
	}
}
