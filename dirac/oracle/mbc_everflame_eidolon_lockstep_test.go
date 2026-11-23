package oracle

import "testing"

// Everflame Eidolon's bestow cast: dirac's autopay found no plan where gorge
// pays (decision 295, tail-15-c000 pair 9).
func TestMBCEverflameEidolonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "everflame_eidolon_bestow_autopay_no_plan")
}
