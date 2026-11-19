package oracle

import "testing"

// Moltensteel Dragon's two {R/P} pips at 3 life: one pip takes its {R} face,
// the other its two-life face, and the mana half {4} is funded by the plan's
// witness (four lands). The witness leaves Selesnya Signet untapped, and its
// "{1}: Add {W}{G}" activation is legal in the CR 601.2g window with the {1}
// fee paid from the witness's floated mana, so the pip check can settle the
// cost. The gorge side once priced the witness over the four free lands alone
// and let the window fund the Signet's fee only from the payer's real (empty)
// pool, so the pip check proved the cast unpayable and withheld it (decision
// 399 of 399; tail-37-c001 pair 55, seed 801202), while the kernel offered it.
func TestMBCMoltensteelDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "moltensteel_pip_fee_funded_by_plan")
}
