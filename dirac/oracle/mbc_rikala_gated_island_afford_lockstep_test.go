package oracle

import "testing"

// CR 605.1a, 605.4a, 106.1: Rikala, Homarid King with three tide counters adds
// an extra {U} for each Island tapped for mana, so p1's three untapped Islands,
// a Forest and a Mystic Gate pay Altar Golem's {7}. gorge offers the cast; the
// kernel did not (tail-02 pair 7, decision 546).
func TestMBCAltarGolemReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rikala_gated_island_afford")
}
