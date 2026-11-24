package kgen

import "testing"

// choiceChooserCarriers are the paper cards of the choice slice that lower:
// Chooser$ on a DefinedPlayer$ library search (Haunting Echoes, Head Games,
// Jester's Mask, Mimeofacture, Thada Adel, Rootwater Thief, Realms
// Uncharted's opponent pick), Chooser$ Triggered<Player> on a public pick
// (Blizzard Specter, Mana Breach, Overburden) and Chooser$ You on a targeted
// graveyard move (Misinformation). Path of Mettle and The Kami War lowered
// before this slice.
//
// Still refused, each needing a kernel op: ChooseCard AtRandom$ (Cinderheart
// Giant, Deadbridge Chant, Last One Standing, The Nipton Lottery), ChooseCard
// Amount$ X / ShiftX (Hew the Entwood, Split the Party, Mist of Stagnation),
// ChooseCard Defined$ Player / TriggeredDefendingPlayer (Call to the Void,
// Sadistic Shell Game, The Horus Heresy, Drana), a GenericChoice of each
// opponent or player with unless-cost branches (Osseous Sticktwister, Zoyowa
// Lava-Tongue, Lim-Dul's Hex), the as-enters GenericChoice Clone (Corrupted
// Shapeshifter, Primal Clay, Primal Plasma) and an opponent's EACH library
// pick (Guided Passage: gorge's structured EACH Min under Mandatory$ is the
// per-type count, which the kernel's Mandatory flag cannot say).
var choiceChooserCarriers = []string{
	"Blizzard Specter", "Haunting Echoes", "Head Games", "Jester's Mask", "Mimeofacture",
	"Thada Adel, Acquisitor", "Rootwater Thief", "Mana Breach", "Overburden", "Misinformation",
	"Realms Uncharted", "Path of Mettle", "The Kami War",
}

func TestChoiceChooserSlice(t *testing.T) {
	for _, name := range choiceChooserCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
