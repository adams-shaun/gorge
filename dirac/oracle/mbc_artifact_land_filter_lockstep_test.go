package oracle

import "testing"

// CR 605.1a, 602.2b: Disciple of Kangee's {U},{T} activation is affordable when the
// only source of blue is Unknown Shores' "{1},{T}: add one mana of any color", paid
// from floating mana. Gorge offers the activation at priority; dirac must too.
func TestMBCDiscipleOfKangeeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "artifact_land_filter_pays_blue_activation")
}
