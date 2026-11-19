package oracle

import "testing"

// CR 106.6 with CR 107.3: a Powerstone token's {C} can't be spent on a
// nonartifact spell, so it adds nothing to the X ask of Death Wind ({X}{B})
// even when a Taps watcher (Relic Bind on the token) makes the token a
// Taps-deferred gap. Both engines offer X = 0..8 with nine mana floating;
// dirac once counted the token and offered 0..9 (decision 588 of
// tail-08-c002 pair 31).
func TestMBCDeathWindReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_ask_restricted_tapsgap_powerstone")
}
