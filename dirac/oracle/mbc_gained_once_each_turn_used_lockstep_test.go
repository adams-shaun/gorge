package oracle

import "testing"

// CR 602.5b with CR 613.1f: a Food that gained Ecstatic Awakener's
// "activate only once each turn" ability (Hazel's Brewmaster exiled it) may
// activate it once this turn, and gorge withholds the offer once it has used
// that identity (gainedActivationsUsed counts the gained pushes per
// recipient, foreign object and index). The kernel offered nothing at all:
// its gained-ability walk stalled (kernel gainsof) at decision 227 of
// tail-45 pair 15 (seed 756318 deal), holding the pool's evidence back.
func TestMBCTail45ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gained_once_each_turn_used")
}
