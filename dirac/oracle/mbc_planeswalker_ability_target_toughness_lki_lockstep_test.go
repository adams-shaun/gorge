package oracle

import "testing"

// CR 608.2h: Garruk the Slayer's -10 destroys a creature and puts loyalty
// counters on Garruk equal to that creature's toughness. The creature (a
// Bloodstone Goblin with two +1/+1 counters, toughness 4) is gone by the time
// the counters are put, so the toughness is the last known one. The dirac
// kernel read the dead card's printed toughness (2) and left Garruk at 12
// loyalty where gorge had 14 (decision 408, tail-40 pair 159).
func TestMBCGarrukTheSlayerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "planeswalker_ability_target_toughness_lki")
}
