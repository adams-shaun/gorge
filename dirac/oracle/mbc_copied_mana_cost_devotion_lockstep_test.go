package oracle

import "testing"

// CR 700.5 and CR 707.2: Phantasmal Image becomes a copy of Daxos, Blessed by
// the Sun, whose toughness is its controller's devotion to white. A copy
// acquires the copied card's mana cost too, so the Image's own {W}{W} counts
// toward that devotion (Sidar Jabari of Zhalfir's {W} plus Daxos's {W}{W},
// 5 total). The kernel counted the Image's printed cost ({1}{U}) and reported
// toughness 3 (decision 540, tail-16 focus game, seed 591790).
func TestMBCPhantasmalImageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copied_mana_cost_devotion")
}
