package oracle

import "testing"

// CR 602.2, 605.3a, 117.3: Hollow One's cycling {2} is payable when the only
// untapped sources are a Spectator Seating ({T}: {R} or {W}) and a Golgari
// Signet ({1},{T}: {B}{G}): the Seating's mana pays the Signet's {1}, and the
// Signet's {B}{G} pays the cycling cost. Gorge offered the cycling activation
// (tail-45 pair 53, decision 610); the kernel did not.
func TestMBCHollowOneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cycling_fed_by_signet_over_a_land")
}
