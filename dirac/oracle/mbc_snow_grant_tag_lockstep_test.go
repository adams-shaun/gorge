package oracle

import "testing"

// CR 106.3a: mana is snow mana when a snow permanent produced it, and a
// granted ability's producer is the permanent it resides on: Cryptolith Rite
// gives creatures "{T}: Add one mana of any of the tapped creature's colors",
// so tapping snow Rimescale Dragon through it floats {S}-paying mana (CR
// 107.4h) and gorge offers Rimescale Dragon's {2}{S} activation. The kernel's
// grant replay once committed the tap's mana with a bare pool write, its
// search branch's artTag dropped, so the snow tally missed the unit and the
// kernel offered only pass (decision 716, tail-15-c002 pair 92, seed 918033).
func TestMBCRimescaleDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "snow_grant_tag")
}
