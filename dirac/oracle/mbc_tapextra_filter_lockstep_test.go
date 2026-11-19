package oracle

import "testing"

// CR 106.12 with CR 605.3a: tapping an Island for mana with Rikala, Homarid
// King at three tide counters adds an extra {U}, and that {U}{U} pays Energy
// Refractor's "{2}: Add one mana of any color", whose {G} then pays Wall of
// Pine Needles' regenerate. Gorge plans the chain; the kernel found no plan
// (kernel: autopay found no plan, decision 968, tail-02 pair 117).
func TestMBCWallOfPineNeedlesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tap_extra_mana_feeds_filter_cost")
}
