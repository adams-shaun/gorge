package oracle

import "testing"

// CR 616.1 with CR 119.4: a player controlling two Worships takes combat damage
// from Exultant Skymarcher while holding the initiative. Both Worships floor
// the same loss at 1, so whichever applies first leaves the other nothing to
// do. Gorge parked an order ask on that damage, the kernel (which applies the
// floors in scan order) dealt it and stole the initiative: they parted at
// `projection Initiative` (tail-55-c000 pair 5, decision 1250).
func TestMBCExultantSkymarcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "worship_pair_life_floors_commute")
}
