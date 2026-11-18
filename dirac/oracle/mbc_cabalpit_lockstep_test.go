package oracle

import "testing"

// CR 602.2b with CR 118.3: Cabal Pit's Threshold ability costs {B}, {T} and
// a sacrifice of itself. Beside the opponent's Price of Glory every land of
// the controller is a Taps-deferred mana source (the census never lists it),
// so the scripted prefix taps a Swamp for {B} into the pool and offers the
// ability priced against the pool alone: the pool's {B} pays the {B} and the
// untapped Cabal Pit pays its own {T}. The kernel took Cabal Pit's own mana
// out of that pool-only availability, where it was never counted, and so
// withheld an activation gorge offers (tail-53 pair 568, decision 820).
//
// The same recording also pins the reverse arm, at decision 777: the offer
// walk's availability counts the plain gap units (availTapsDeferIn), so
// Gate to Tumbledown's own {T} takes its {R} back out of its own {3}{R} seek
// -- the kernel offered a self-funded activation gorge's planner withholds,
// because removeSrc's availLeavesOut skip trusted its availFor premise where
// the gap units were counted.
func TestMBCCabalPitReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "deferred_land_own_tap_keeps_floating_mana")
}
