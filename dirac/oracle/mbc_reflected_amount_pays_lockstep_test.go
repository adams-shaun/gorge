package oracle

import "testing"

// CR 605.1a, 602.2b, 106.7: Incubation Druid (a ManaReflected ability with a computed Amount$)
// is the only untapped source of {B} for Wall of Bone's {B} regeneration activation, beside
// a Joraga Treespeaker grant. gorge's PotentialMana bound prices the Druid's reflected colours,
// so its prefix search runs and offers the activation: dirac's grant search must tap the Druid
// for its reflected colour, not as a Combo Any source (pair 38 of tail-55: dirac's relaxed
// proof dropped the activation, then its replay floated {W} where gorge floated {B}).
func TestMBCWallOfBoneRegenerateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_amount_mana_pays_regenerate")
}
