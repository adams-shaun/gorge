package oracle

import "testing"

// TestMBCJaceSProjectionReprosLockstep replays the synthetic tail-49 game the
// evidence loop recorded (tail-49-c000 pair 52, seed 12793204730852197255).
// Evil Twin (#6) entered as a copy of Thorncaster Sliver, so its granted
// "{U}{B}, {T}: Destroy target creature with the same name as this creature."
// may target only #6 itself and #75 (Thorncaster Sliver). At decision 923
// ("options options target decision for p0") gorge offers exactly those two;
// the kernel also offered #16 (Jace's Projection), #29 (Stone Catapult) and
// both players — the granted ability's target filter lowered to none.
func TestMBCJaceSProjectionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "clone_grant_samename_target_any")
}
