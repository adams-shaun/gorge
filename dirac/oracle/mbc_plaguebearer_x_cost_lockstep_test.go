package oracle

import "testing"

// Plaguebearer's "{X}{X}{B}: Destroy target nonblack creature with mana value
// X" activation needs one announcement that both pays and reaches a target.
// On this board the only nonblack creature is a mana value 4 Lightning Hounds,
// and the controller has three storage counters plus three floating mana, so
// the {B}{8} at X=4 is unpayable. gorge offered the activation anyway (its
// potential bound read Fountain of Cho's storage mana as unbounded) while the
// kernel had already withheld it: the first divergence of tail-67 pair 40 on
// the full surface. gorge was wrong (CR 107.3i announces X before CR 601.2b's
// targets, and CR 601.2h's payment must reach it), and this recording keeps the
// pair's game as the lockstep regression.
func TestMBCPlaguebearerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "plaguebearer_x_cost")
}
