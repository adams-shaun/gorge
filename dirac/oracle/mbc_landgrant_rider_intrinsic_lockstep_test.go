package oracle

import "testing"

// CR 305.6 with CR 613.1d: Sealock Monster's Monstrosity trigger turns a
// target land into an Island in addition to its other types. Gemstone Mine's
// own "{T}, Remove a mining counter: Add one mana of any color" is a
// planner-deferred rider, so the land's only planner census unit is the
// intrinsic "{T}: Add {U}" the granted basic land type gives it. Gorge's
// planner listed that unit and offered Dromoka Warrior {1}{W} and Stratus
// Dancer {1}{U} beside a fresh Plains and the now-Island Mine; the kernel
// left the intrinsic out, so both casts were gorge-only (decision 570,
// tail-28 pair 2, surface autopay).
func TestMBCDromokaWarriorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "landgrant_rider_intrinsic")
}
