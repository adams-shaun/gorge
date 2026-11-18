package oracle

import "testing"

// CR 601.2h / 605.3: the kernel offered Stampeding Serow ({2}{G}{G}) to a
// seat whose only untapped mana sources are Firemind Vessel and Survivors'
// Encampment; gorge's planner could not find four mana and did not.
func TestMBCStampedingSerowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "serow_cast_unaffordable_vessel")
}
