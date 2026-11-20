package oracle

import "testing"

// CR 704.5a: Molten Rebuke (a sorcery) beside Manabarbs at 3 life, six untapped
// lands and seven Eldrazi Spawn. Manabarbs defers every land, so gorge's prefix
// search taps the lands as gaps and then passes to resolve the queued damage
// triggers before the sorcery is offered. A pass chain whose damage kills the
// payer ends at the fatal resolution (the visit returns at the resulting
// priority's G.Over); dirac charged the whole queued chain, ran past gorge's
// 160-node budget before the pass node that offers the cast, and dropped it
// (decision 358 of tail-34-c000, pair 27).
func TestMBCMoltenRebukeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapsdefer_pass_chain_ends_at_lethal_resolution")
}
