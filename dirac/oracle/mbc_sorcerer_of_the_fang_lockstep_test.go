package oracle

import "testing"

const sorcererFangRepro = "sorcerer_of_the_fang_activation_offer"

// Sorcerer of the Fang's {5}{B},{T} (pool tail-51, surface autopay, the
// recorded game; gorge ids 71/72): p1 has one untapped land (Radiant Grove),
// every Swamp tapped, Prismite ("{2}: Add one mana of any color") and a
// Treasonous Ogre ("Pay 3 life: Add {R}"). The only {B} left is Prismite's, and
// its {2} fee, with the five generic, is paid from the Grove and six Ogre
// activations, so gorge's prefix search offers the activation. The kernel's
// activation read now plans the ability's mana cost beside its own {T} against
// the same potential bound a cast reads, with the sole life-only pain unit as
// the plan's last resort (CR 602.2b, 605.3a, 119.4), so it offers it too: the
// recorded game replays clean and the card is no longer refused
// (dirac/support/unrefused.tsv). This test pins the unrefusal's proof.
func TestMBCSorcererOfTheFangReproReplaysClean(t *testing.T) {
	res := replayRawRepro(t, sorcererFangRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", sorcererFangRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", sorcererFangRepro, res.Stall)
	}
}
