package oracle

import "testing"

// CR 702.22d: every creature in an attacking band attacks the same player,
// planeswalker or battle. Hatut Zeraze, Dross Harvester and Battering Ram
// (banding) attack the player while Slayer of the Wicked and Setessan Battle
// Priest attack a planeswalker, so only the first two are band candidates.
// The recording runs past the first accepted candidate now that band formation
// is modelled (CR 702.22d-k); both engines offered the same candidates. The
// kernel compared only the defending seat and a battle, and offered the
// planeswalker's attackers too (decision 388 of 388).
func TestMBCSetessanBattlePriestReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "band_candidates_same_planeswalker")
}

// CR 702.22c: a band is formed by creatures attacking the same player or
// planeswalker. Battering Ram (banding until end of combat) attacked a
// planeswalker while Kozilek's Channeler attacked its controller; gorge
// compares the whole defender (Object.AttackingBattle covers planeswalkers)
// and posed no band ask, the kernel counted only battles and posed one.
func TestMBCBatteringRamReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "band_planeswalker_defender")
}
