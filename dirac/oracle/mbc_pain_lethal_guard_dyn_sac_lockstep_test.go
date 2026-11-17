package oracle

import "testing"

// CR 119.4 with CR 605.3b: City of Brass's own "becomes tapped" trigger deals
// 1 damage to its controller as it resolves, and the payer is at 1 life. The
// priority offer walk counts a dynamic-amount source beside Vitu-Ghazi, the
// City-Tree's {2}{G}{W} activation; the walk's dynamic-sac flag shared a bit
// with the manual cast window's no-lethal-guard mode, so dirac counted the
// painland's colours and offered the activation gorge's planner refuses.
func TestMBCVituGhaziTheCityTreeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pain_lethal_guard_dyn_sac")
}
