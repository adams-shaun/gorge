package oracle

import "testing"

// CR 601.2a/601.2h: Moment of Defiance is cast from the seat's hand when its
// {2}{B}{B} cost is payable; gorge offers the cast while dirac omits it
// (dr-mbc-bbbb pair 90 at land 468a13cf01, decision 235).
func TestMBCMomentOfDefianceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "moment_of_defiance_options")
}
