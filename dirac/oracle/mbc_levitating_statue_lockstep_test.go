package oracle

import "testing"

// CR 106.6: the colourless mana Mishra's Workshop leaves floating after paying
// for Levitating Statue may be spent only on artifact spells, so with the
// Workshop tapped and one Swamp untapped Levitating Statue's {2} ability is
// unaffordable. The kernel offered the activation anyway (tail-29 pair 6,
// decision 48): gorge's options were [pass, cast Graveyard Shovel].
func TestMBCLevitatingStatueReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "workshop_restricted_float_pays_no_ability")
}
