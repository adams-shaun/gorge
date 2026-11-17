package oracle

import "testing"

// CR 605.1a, 106.4: Lotus Blossom's "{T}, Sacrifice this: Add X mana of any one
// color" (X its petal counters) funds a cast gorge's planner prices through its
// prefix search. With one petal counter, an Island and a Buried Ruin on the
// battlefield, gorge offers Guul Draz Assassin ({B}) and Warren Weirding
// ({1}{B}); the kernel listed neither because it modelled only a fixed-colour
// counter-scaled sacrifice source (decision 98, tail-74-c000 pair 5).
func TestMBCGuulDrazAssassinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_counter_any_colour_mana_gap")
}
