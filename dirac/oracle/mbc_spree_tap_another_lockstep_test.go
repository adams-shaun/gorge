package oracle

import "testing"

// CR 601.2b, 702.172a, 605.1a: Dance of the Tumbleweeds (Spree, {1}{G}) is
// cast with a floating {G}{U}, one untapped Island and Urza, Lord High
// Artificer beside one untapped Construct token (tap an untapped artifact:
// add {U}): four mana in all. Its second mode costs {3} more, five in all,
// so gorge's PotentialMana withholds it at the mode ask. dirac's potential
// walk already counted Urza's tap-another unit and the Spree pricing added it
// a second time, offering the unpayable mode (decision 523 of dr-mbc-baaa
// pair 167, seed 958249).
func TestMBCDanceOfTheTumbleweedsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spree_mode_tap_another_counted_once")
}
