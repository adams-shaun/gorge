package oracle

import "testing"

// Azusa's Many Journeys and Kiku's Shadow were offered as casts by dirac but
// not by gorge at a priority decision for p0 (decision 553 of tail-55 pair 25,
// seed 14678303355374664241).
func TestMBCAzusaSManyJourneysReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "azusa_many_journeys_cast_offer")
}
