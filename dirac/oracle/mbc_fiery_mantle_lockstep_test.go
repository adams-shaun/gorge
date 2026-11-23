package oracle

import "testing"

// CR 601.2h with CR 603.2: casting Fiery Mantle ({1}{R}) with Scald in play
// pays the generic part from an Island, and each Island tap costs its tapper
// 1 life. The recording (tail-49 pair 9, decision 795) ends at a priority
// decision where dirac offered the cast and gorge did not.
func TestMBCFieryMantleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scald_self_damage_cast_offer")
}
