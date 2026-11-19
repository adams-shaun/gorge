package oracle

import "testing"

// CR 605.1a, 601.2g: Overflowing Basin's first-ability filter ("{1}, {T}:
// Add {G}{U}") is the only census gap and Relic of Legends is tapped, so its
// free "{T}: Add one mana of any color" is gone; the relic's second ability
// ("Tap an untapped legendary creature: Add one mana of any color") taps
// Balan, Wandering Knight and funds the Basin's {1}. The Basin's {G}{U}
// then pays Aberrant's {1}{G} at X=0. The kernel admitted the first-only
// filter only when the plain walk covered its fee, leaving the cast out of
// the priority options (decision 655 of 655; synth tail-49 pair 5).
func TestMBCAberrantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_funded_by_tap_another_source")
}
