package oracle

import "testing"

const sorcererFangRepro = "sorcerer_of_the_fang_activation_offer"

// Sorcerer of the Fang's {5}{B},{T} (pool tail-51, surface autopay, the
// recorded game; gorge ids 71/72): p1 has one untapped land (Radiant Grove),
// every Swamp tapped, Prismite ("{2}: Add one mana of any color") and a
// Treasonous Ogre ("Pay 3 life: Add {R}"). The only {B} left is Prismite's, and
// its {2} fee, with the five generic, is paid from the Grove and six Ogre
// activations (18 of p1's 20 life), so gorge's prefix search offers the
// activation at decision 233. The kernel's filter search once funded a fee only
// from taps and census gaps, never from a sole life-only pain unit, so it
// offered nothing and the card was refused by name. The kernel now funds the
// Prismite fee through the life-only Ogre (dirac/mana_lifefund.go, CR 605.3b,
// 601.2g), both engines offer the activation, and the recorded game replays
// clean; the card is therefore no longer refused (dirac/support/unrefused.tsv),
// and this test pins the unrefusal's proof.
func TestMBCSorcererOfTheFangReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, sorcererFangRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", sorcererFangRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", sorcererFangRepro, res.Stall)
	}
}
