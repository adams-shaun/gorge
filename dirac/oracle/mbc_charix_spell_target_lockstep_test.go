package oracle

import "testing"

// CR 601.2c with 601.2f: Charix, the Raging Isle taxes spells its opponents cast that
// target it, so a creature that can't be paid for with the tax is no target of that spell.
// The two engines parted at the target decision of tail-34-c001 pair 91 (decision 642),
// where dirac offered Charix and gorge did not.
func TestMBCCharixTheRagingIsleSpellTargetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "charix_tax_spell_target_option")
}
