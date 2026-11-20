package oracle

import (
	"testing"
)

// CR 702.103b (bestow): a bestowed Aura is an Aura enchantment, not a
// creature, so its printed creature subtypes are gone with the Creature card
// type (CR 205.1a). Ixidor's Will counters target spell unless its controller
// pays {2} for each Wizard; its SVar counts Wizard.YouCtrl from the caster's
// side, and the caster's bestowed Triton Wavebreaker is not a Wizard. gorge's
// type filter answered Creature=no but let the subtype word fall through to
// the printed face, counting the bestowed Aura as a Wizard, one {2} too many.
func TestMBCIxidorSWillReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestowed_creature_subtype_count")
}
