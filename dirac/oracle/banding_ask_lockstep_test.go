package oracle

import "testing"

// TestBatteringRamBandingLockstep: Battering Ram's UntilEndOfCombat grant of
// Banding (kop.Override.Banding) is read by the kernel's CR 702.22c band
// declaration ask (dirac/combat_band.go), which must pose gorge's ask at the
// same decision with the same options: a Ram that attacks beside another
// creature offers the decline and one candidate per other attacker. The
// probe declines (kpecDeclineBand); TestBandFormationLockstep accepts. Seeds beyond TestPumpEndCombatLockstep's range, so
// the two never replay a game.
func TestBatteringRamBandingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := kpecRunFrom(t, 1001, 64, true)
	t.Logf("%d games, %+v, %d divergences, %d stalls", games, st, divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.bandAsks < 5 {
		t.Errorf("only %d band asks seen (want at least 5): no Ram attacked beside another creature", st.bandAsks)
	}
}
