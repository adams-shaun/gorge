package oracle

import "testing"

// CR 603.2 with Forge's FirstTime$ True on Mode$ LifeLost: Intermediate
// Chirography's level-2 trigger ("whenever you lose life for the first time
// each turn") fires once for a turn's first loss. The kernel enforced the
// first-time gate on life GAIN only, so a combat hit (one loss per damage
// event) queued the trigger twice and posed a trigger-order decision gorge
// never did (tail-04 pair 3, evidence loop ev-tail-04-c001).
func TestMBCIntermediateChirographyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_lost_first_time_each_turn")
}
