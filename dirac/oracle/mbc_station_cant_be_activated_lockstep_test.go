package oracle

import "testing"

// CR 602.5 with CR 702.184a: a Spacecraft's Station ability is an activated
// ability, so a CantBeActivated static on it (Prison Term's "Enchanted
// creature can't ... its activated abilities can't be activated") forbids
// stationing: neither engine offers the "station" priority option. Gorge
// once offered it; the kernel (which routes the offer through the
// CantBeActivated walk) withheld it. dr-mbc-aaab pair 76.
func TestMBCFellGravshipReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "station_cant_be_activated")
}
