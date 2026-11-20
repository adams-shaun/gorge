package oracle

import "testing"

// CR 508.1d with CR 302.6: Utvara Scalper ("attacks each combat if able") is
// summoning sick on p1's side, and p0's Frenzied Saddlebrute lets creatures
// attack only p0's opponents as though they had haste, so against p0 the
// Scalper is exempt from no defender and can't attack. It binds nothing, so
// p1's other creature (Roc Hatchling) isn't required either and the attackers
// decision offers Done. The kernel read the exemption defender-blind and
// withheld Done (decision 756 of tail-71-c003, seed 719438).
func TestMBCHearthKamiReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sick_must_attacker_no_haste_defender_not_required")
}
