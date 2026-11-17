package oracle

import "testing"

// CR 400.7: an object that moves from one zone to another becomes a new
// object with no memory of its previous existence, so the -1/-1 counters a
// reanimated creature card held in the graveyard do not follow it back onto
// the battlefield. Torture's {1}{B}: "put a -1/-1 counter on enchanted
// creature" resolves after its enchanted Catapult Master died, puts the
// counter on the graveyard card (a counter on a non-permanent object is
// inert, not illegal), and Doomed Necromancer then returns that same object:
// the re-entered permanent is a new object and enters at 3/3 clean. gorge
// carried the graveyard counter into the battlefield (2/2 with one counter)
// where the kernel cleared it (tail-71 pair 2219, decision 804: projection
// Cards[1].Power). Skullbriar's CountersRemain static is the CR's own
// exception and still carries across the return.
func TestMBCCatapultMasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "torture_reentry_counters")
}
