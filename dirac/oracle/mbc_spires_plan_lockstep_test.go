package oracle

import "testing"

// Hybrid pips announced {B}{B}{W} for Suture Spirit's regenerate with a Cryptic
// Spires untapped (tail-06 pair 3, seed 774113, decision 1128): gorge's planned
// payment tapped the Spires, which adds no mana (ProducedCounts claims none for
// the circled-colours word), while the kernel leaves it untapped.
func TestMBCStrongholdTaskmasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spires_tapped_for_no_mana_in_plan")
}
