package oracle

import "testing"

// CR 702.22d: every creature in an attacking band attacks the same player,
// planeswalker or battle. Hatut Zeraze, Dross Harvester and Battering Ram
// (banding) attack the player while Slayer of the Wicked and Setessan Battle
// Priest attack a planeswalker, so only the first two are band candidates.
// The recording is marked stalled: the game ends at the kernel's band
// stall once the first candidate is accepted (band formation is not modelled),
// after both engines offered the same candidates. The kernel compared only the defending seat and a battle, and offered the
// planeswalker's attackers too (decision 388 of 388).
func TestMBCSetessanBattlePriestReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "band_candidates_same_planeswalker")
}
