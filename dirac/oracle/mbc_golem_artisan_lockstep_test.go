package oracle

import "testing"

// CR 602.2 / 605.3: Golem Artisan's {2} abilities are payable when the only
// untapped sources are a Swamp and Viridescent Bog ({1},{T}: add {B}{G}); the
// Swamp's mana funds the Bog's fee. dirac once left the activations out of the
// priority options (decision 1077 of 1077, tail-12-c000 pair 58).
func TestMBCGolemArtisanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "golem_artisan_filter_land_funds_activation")
}
