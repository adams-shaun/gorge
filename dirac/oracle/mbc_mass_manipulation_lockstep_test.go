package oracle

import "testing"

// Mass Manipulation ({X}{X}{U}{U}{U}{U}, X targets) was offered as a cast by gorge
// but not by the kernel at priority (tail-24 pair 125, decision 337).
func TestMBCMassManipulationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mass_manipulation_cast_offer")
}
