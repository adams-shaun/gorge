package oracle

import "testing"

// CR 707.2 with CR 614.12: Auton Soldier enters as a copy of Phenomenon
// Investigators, whose copiable values carry "As this enters, choose Believe
// or Doubt". The copy makes that choice as it enters: the kernel poses the
// mode decision, and gorge once skipped it (decision 394 of 394).
func TestMBCPhenomenonInvestigatorsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "clone_inherits_entry_mode_choice")
}
