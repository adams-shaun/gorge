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
	// CR 601.2b, 601.2c: Hearth Kami's {X}, Sacrifice: destroy target artifact
	// with mana value X, with one untapped land and the only artifact (Mine
	// Worker) at mana value 2, has no announcement of X that both pays and
	// leaves a legal target. Gorge offered it anyway (decision 421 of
	// tail-71-c000 pair 3, seed 967027, autopay).
	stdReproNamed(t, "x_bound_target_unpayable_x_sac_self")
}
