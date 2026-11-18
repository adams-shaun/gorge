package oracle

import "testing"

// CR 702.36b: a creature with fear can be blocked by an artifact creature, and
// a land that Mishra's Groundbreaker made a 3/3 artifact creature is one. The
// gorge side tested the printed face for the artifact type, so Sleeper's Robe
// on a Cunning Survivor left the animated Island no block to offer while dirac
// (reading the derived types) offered it (decision 377 of tail-35-c000, pair 0).
func TestMBCCunningSurvivorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fear_blocked_by_animated_artifact_land")
}
