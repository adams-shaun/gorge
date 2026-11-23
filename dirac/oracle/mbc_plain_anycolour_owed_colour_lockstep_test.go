package oracle

import "testing"

// CR 605.3, 601.2g: a second-ability source's any-colour plain first ability
// asks its colour, and gorge's payment window answers a colour the play still
// owes first. dirac's tap-another search branched the plain arm at its ability
// ask with no colour ask of its own, floating W where the play owes R
// (Relic of Legends funding a red Rain of Salt, tail-49 pair 23, decision 543)
// and W beside the owed G (Salt Road Quartermasters, tail-49 pair 100,
// decision 438): both sides' projections then part on the pool at the cast's
// apply.
func TestMBCRelicOfLegendsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "relic_of_legends_plain_anycolour_owed_colour")
}

func TestMBCSaltRoadQuartermastersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "salt_road_quartermasters_plain_anycolour_owed_colour")
}
