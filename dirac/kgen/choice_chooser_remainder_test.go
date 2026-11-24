package kgen

import "testing"

// choiceChooserRemainderCarriers are the paper cards the Chooser$ slice left
// refused, each needing a kernel op: ChooseCard AtRandom$, Amount$ X/ShiftX,
// Defined$ Player / TriggeredDefendingPlayer, a GenericChoice of each
// opponent or player with unless-cost branches, the as-enters GenericChoice
// Clone and an opponent's EACH library pick.
var choiceChooserRemainderCarriers = []string{
	// ChooseCard AtRandom$
	"Cinderheart Giant", "Deadbridge Chant", "Last One Standing", "The Nipton Lottery",
	// ChooseCard Amount$ X / ShiftX
	"Hew the Entwood", "Split the Party", "Mist of Stagnation",
	// ChooseCard Defined$ Player / TriggeredDefendingPlayer
	"Call to the Void", "Sadistic Shell Game", "The Horus Heresy", "Drana, the Last Bloodchief",
	// GenericChoice answered by each opponent or each player
	"Osseous Sticktwister", "Zoyowa Lava-Tongue", "Lim-Dûl's Hex",
	// GenericChoice as an as-enters DB$ Clone replacement
	"Corrupted Shapeshifter", "Primal Clay", "Primal Plasma",
	// an opponent's EACH library pick
	"Guided Passage",
}

func TestChoiceChooserRemainder(t *testing.T) {
	for _, name := range choiceChooserRemainderCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
