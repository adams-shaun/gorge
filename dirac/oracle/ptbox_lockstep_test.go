package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Two constant power/toughness compares in one clause at four seats
// (kop.PredStatBox): the 1/1 payoffs read "a 1/1 creature" as power 1 and
// toughness 1 together, each on its current value (Pendelhaven's pump
// target, Pendelhaven Elder's and The Seedcore's pumps, Sword of the Meek's
// return trigger, Aegis of the Meek's target, Sigil Captain's counters,
// Lovestruck Beast's attack restriction), and Sarkhan's Unsealing's cast
// trigger reads power 4 to 6. Creatures of every size, 0/2 to 7/7, fill the
// decks; once pumped or countered a 1/1 stops being one. The expected answer
// at every decision is gorge's.
var ptBoxDecks = [4][]string{
	podDeck(deckOf(2, "Pendelhaven"), deckOf(3, "Pendelhaven Elder"), deckOf(3, "Sword of the Meek"), deckOf(6, "Llanowar Elves"),
		deckOf(4, "Ornithopter"), deckOf(4, "Lovestruck Beast"), deckOf(3, "Grizzly Bears"), deckOf(15, "Forest")),
	podDeck(deckOf(3, "Sarkhan's Unsealing"), deckOf(3, "Spined Wurm"), deckOf(3, "Craw Wurm"), deckOf(2, "Enormous Baloth"),
		deckOf(4, "Raging Goblin"), deckOf(4, "Shock"), deckOf(3, "Grizzly Bears"), deckOf(9, "Mountain"), deckOf(9, "Forest")),
	podDeck(deckOf(3, "Sigil Captain"), deckOf(3, "Aegis of the Meek"), deckOf(5, "Savannah Lions"), deckOf(5, "Llanowar Elves"),
		deckOf(2, "The Seedcore"), deckOf(2, "Sword of the Meek"), deckOf(10, "Plains"), deckOf(10, "Forest")),
	podDeck(deckOf(3, "Lovestruck Beast"), deckOf(2, "Pendelhaven"), deckOf(4, "Raging Goblin"), deckOf(4, "Llanowar Elves"),
		deckOf(3, "Sarkhan's Unsealing"), deckOf(3, "Spined Wurm"), deckOf(3, "Shock"), deckOf(9, "Forest"), deckOf(9, "Mountain")),
}

func TestPTBoxPodLockstep(t *testing.T) {
	t.Parallel()
	podFixtureLockstep(t, "ptbox", ptBoxDecks, 24, true, nil)
}

// TestPTBoxPodLockstepMutation: with every PredStatBox read as "matches
// anything", some seed diverges.
func TestPTBoxPodLockstepMutation(t *testing.T) {
	t.Parallel()
	n := podFixtureLockstep(t, "ptbox", ptBoxDecks, 24, false, dropFilterPred(func(f *kop.Filter) bool {
		return f.Pred == kop.PredStatBox
	}))
	if n == 0 {
		t.Fatal("no seed diverged with the power/toughness box dropped")
	}
}
