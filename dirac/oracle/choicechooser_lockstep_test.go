package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Chooser$ shapes of the choice slice, one carrier a game: the
// controller searching another player's library (Head Games, Jester's Mask,
// Haunting Echoes, Mimeofacture, Thada Adel, Rootwater Thief), a triggered
// player's own pick from the battlefield (Mana Breach, Overburden, Blizzard
// Specter), a targeted graveyard move with Chooser$ You (Misinformation) and an
// opponent's pick among the found cards (Realms Uncharted).
// The other deck plays Bears and artifacts, which the searches find and the
// triggers punish.
var choiceChooserCarriers = []string{
	"Blizzard Specter", "Haunting Echoes", "Head Games", "Jester's Mask", "Mimeofacture",
	"Thada Adel, Acquisitor", "Rootwater Thief", "Mana Breach", "Overburden", "Misinformation",
	"Realms Uncharted",
}

func choiceChooserDecks(card string) [2][]cardset.Entry {
	lands := func(n int) []cardset.Entry {
		return []cardset.Entry{{Name: "Island", Count: n}, {Name: "Swamp", Count: n}, {Name: "Forest", Count: n}, {Name: "Mountain", Count: n}, {Name: "Plains", Count: n}}
	}
	d0 := append([]cardset.Entry{{Name: card, Count: 12}, {Name: "Grizzly Bears", Count: 3}}, lands(5)...)
	d1 := append([]cardset.Entry{{Name: "Grizzly Bears", Count: 12}, {Name: "Jester's Mask", Count: 4}}, lands(5)...)
	d1[len(d1)-1].Count = 4 // 12+4+5*4+4 = 40
	return [2][]cardset.Entry{d0, d1}
}

// choiceChooserLockstep plays 200 games a carrier. lockstepSeedsMode plays every
// fourth seed on the gorge autopay surface (and every eighth on a random
// policy), so both autopay surfaces run in each carrier's 200 games.
func choiceChooserLockstep(t *testing.T, cards []string) {
	if isolate(t) {
		return
	}
	for _, card := range cards {
		t.Run(card, func(t *testing.T) {
			lockstepRun(t, "choicechooser", choiceChooserDecks(card), 200, true, nil)
		})
	}
}

func TestChoiceChooserLockstepSearch(t *testing.T) {
	choiceChooserLockstep(t, choiceChooserCarriers[1:5])
}

func TestChoiceChooserLockstepTriggers(t *testing.T) {
	choiceChooserLockstep(t, append([]string{choiceChooserCarriers[0]}, choiceChooserCarriers[7:10]...))
}

func TestChoiceChooserLockstepFound(t *testing.T) {
	choiceChooserLockstep(t, append(append([]string{}, choiceChooserCarriers[5:7]...), choiceChooserCarriers[10:]...))
}
