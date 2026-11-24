package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// The choosecardkinds fixture: Archon of Valor's Reach (CantBeCast), Cloud Key
// and Stenn (ReduceCost) each choose a card type as they enter (gorge
// effChooseType over TypeChoiceLabels("Card", ...)) and read it through
// Card.ChosenType. Both decks cast spells of several card types (an Instant, a
// Sorcery, creatures, an artifact, an enchantment), so the answer decides which
// casts are barred or cheaper.
var cardKindsDecks = []fixtureDecks{
	{
		{{Name: "Cloud Key", Count: 4}, {Name: "Stenn, Paranoid Partisan", Count: 4}, {Name: "Searing Spear", Count: 4}, {Name: "Divination", Count: 4},
			{Name: "Goblin Piker", Count: 3}, {Name: "Short Sword", Count: 3}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 6}},
		{{Name: "Archon of Valor's Reach", Count: 4}, {Name: "Cloud Key", Count: 3}, {Name: "Searing Spear", Count: 4}, {Name: "Divination", Count: 3},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Goblin Piker", Count: 3}, {Name: "Glorious Anthem", Count: 3}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 6}},
	},
	{
		{{Name: "Archon of Valor's Reach", Count: 4}, {Name: "Stenn, Paranoid Partisan", Count: 3}, {Name: "Searing Spear", Count: 4}, {Name: "Divination", Count: 4},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Short Sword", Count: 3}, {Name: "Glorious Anthem", Count: 3}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 6}, {Name: "Island", Count: 6}},
		{{Name: "Cloud Key", Count: 4}, {Name: "Searing Spear", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Short Sword", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Island", Count: 9}},
	},
}

var cardKindsWatch = map[string]bool{"Archon of Valor's Reach": true, "Cloud Key": true, "Stenn, Paranoid Partisan": true}

// cardKindsDeckSize is the decks' own precondition: every deck is 40 cards.
func cardKindsDeckSize(t *testing.T) {
	t.Helper()
	for mi, md := range cardKindsDecks {
		for s, d := range md {
			n := 0
			for _, e := range d {
				n += e.Count
			}
			if n != 40 {
				t.Fatalf("matchup %d deck %d holds %d cards, want 40", mi, s, n)
			}
		}
	}
}

// TestChooseTypeCardKindsLockstep plays the choosecardkinds pool in lockstep
// with gorge. The precondition is that each of the three card-type choosers was
// cast in some game. The mutation makes a Card.ChosenType read from such a
// source ignore the chosen type (dirac.DebugCardKindIgnore: it matches every
// card), so a reduction or a bar lands on the wrong spells and the surfaces
// diverge.
func TestChooseTypeCardKindsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardKindsDeckSize(t)
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstep(t, "choosecardkinds", cardKindsDecks, cardKindsWatch, 120, true, nil)
		t.Logf("%d games, %d divergent, %d stalls; casts %v", run.games, run.divs, run.stalls, run.casts)
		for n := range cardKindsWatch {
			if run.casts[n] == 0 {
				t.Errorf("%s was never cast: the choice never played", n)
			}
		}
	})
	t.Run("mutation-ignore", func(t *testing.T) {
		dirac.DebugCardKindIgnore = true
		defer func() { dirac.DebugCardKindIgnore = false }()
		run := runFixtureLockstep(t, "choosecardkinds", cardKindsDecks, cardKindsWatch, 120, false, nil)
		run.requireDivergence(t, "a card-type source's Card.ChosenType ignoring its chosen type")
	})
}
