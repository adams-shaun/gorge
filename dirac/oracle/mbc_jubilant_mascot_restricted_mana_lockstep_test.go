package oracle

import "testing"

// Jubilant Mascot's beginning-of-combat "you may pay {3}{W}" trigger (CR 118.3)
// should offer only the decline when one floating restricted mana batch (CR
// 106.6) and two untapped Plains cannot reach the cost. dirac's canPayGen
// counted the admitted batch twice — the same block survived in both merge
// parents — so it priced {3}{W} payable and offered trigger_cost_pay where
// gorge offered only trigger_cost_decline (decision 647, pool tail-62, seed
// 5391878975346934601, pair 4).
func TestMBCJubilantMascotReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "jubilant_mascot_restricted_mana")
}
