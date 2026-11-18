package oracle

import "testing"

// CR 611.2b / 400.7: a steal that lasts "as long as Cytoplast Manipulator
// remains on the battlefield" ends the moment it leaves, even when an Essence
// Flux brings it straight back inside the same resolution. The stolen War
// Scout is its owner's again before Cytoplast re-enters, so the Scout's
// "another creature enters" trigger goes on the stack under its owner.
// Dirac settled control only at the next state-based check, after the
// re-entry, and put the trigger under the thief.
func TestMBCCytoplastManipulatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "control_ends_when_source_blinks_in_resolution")
}
