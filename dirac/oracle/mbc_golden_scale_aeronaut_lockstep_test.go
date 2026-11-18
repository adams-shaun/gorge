package oracle

import "testing"

// CR 106.6: a Powerstone's {C} can't be spent on a nonartifact spell but pays
// a trigger's own cost. Vigil for the Lost's "you may pay {X}" announces X up
// to everything the payer can produce, the untapped Powerstone included (tail-12
// pair 2, decision 468: gorge offered X 0..7, dirac 0..6 and left the
// restricted source out of its bound). The kfuzz class named Golden-Scale
// Aeronaut, whose creature dealt the combat damage that killed the Piper.
func TestMBCGoldenScaleAeronautReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigx_restricted_source_bound")
}
