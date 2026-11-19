package oracle

import "testing"

// CR 118.3 with CR 106.5: a trigger's own optional mana cost is offered on the
// payer's potential mana (rules respay.go resPayNeedsMana over potential.go),
// and gorge's potential walk prices a mana ability whose Amount$ it cannot
// statically price -- Lotus Blossom's "Add X mana of any one color, where X is
// the number of petal counters on Lotus Blossom" (Amount$ X, Count$CardCounters)
// -- potentialUnbounded in every colour, so the payer's end-step Jerren,
// Corrupted Bishop "may pay {4}{B}{B}" ask offers pay beside decline even
// though every source on the board is spent or makes nothing yet: the payer
// elects pay, fails the charge and declines. The kernel's offer gate
// (interp_ask.go trigCostPotentialPays) prices the census's current amount --
// zero petals is zero mana and no planner unit -- so the ask was decline-only
// while gorge offered pay and decline (decision 831; synth tail-74 pair 16).
func TestMBCJerrenCorruptedBishopReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trig_cost_dynamount")
}
