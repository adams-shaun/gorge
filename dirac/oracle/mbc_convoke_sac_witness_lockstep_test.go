package oracle

import "testing"

// A convoke cast (Conclave Tribunal, {3}{W}) payable only by sacrificing the
// three Eldrazi Scions beside the Plains: gorge's planner declines the convoke
// shape but admits the witness (targetPaidNeedsPlannedPayment) when the lowering
// cannot reach the play, and its witness execution gives up once the
// sacrifices' triggers (two Lifelines per dying Scion, six in all) pass the
// four-pass wait budget before the sorcery-speed enchantment is offered again
// (potential_script.go witnessReachesMode, waits >= 4). The kernel priced the
// pool that the witness floats and so called the lowering reaching, leaving the
// cast out. The recording is kfuzz -synth tail-46 over a Conclave Tribunal
// focus, pair 105.
func TestMBCConclaveTribunalReprosLockstep(t *testing.T) {
	stdReproNamed(t, "convoke_sac_witness_stacks_past_wait_budget")
}
