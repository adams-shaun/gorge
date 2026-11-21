package oracle

import "testing"

// CR 702.22c: a band is formed by creatures attacking the same player or
// planeswalker. Battering Ram (banding until end of combat) attacked a
// planeswalker while Kozilek's Channeler attacked its controller; gorge
// compares the whole defender (Object.AttackingBattle covers planeswalkers)
// and posed no band ask, the kernel counted only battles and posed one.
func TestMBCBatteringRamReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "band_planeswalker_defender")
}
