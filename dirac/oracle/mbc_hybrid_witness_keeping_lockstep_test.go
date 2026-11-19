package oracle

import "testing"

// CR 601.2b with CR 601.2c-h: Shattering Blow ({1}{R/W}, "Exile target
// artifact") is a plain hybrid hand cast. Its only legal target beside the
// game's untapped mana is the one Treasure, whose sac-for-mana ability pays
// the {R/W} pip as {R}: gorge's potential witness, and its
// potentialWitnessReaches walk refuses it (the payment consumes the cast's
// only target). potentialWitnessKeeping then replans with planKeepSources,
// where planHybridCost treats a consuming plan as insufficient and tries the
// next pip colouring ({W}: the Plains pays the pip, the Island the {1}), so
// the cast is offered. The kernel's plainSacKeepsTarget once stopped at the
// refusal and never modelled the keep-sources colouring retry, and offered
// only pass. (plain_cast_pays_with_its_only_target is its sibling: there no
// keeping plan exists, and both sides refuse.)
func TestMBCHybridWitnessKeepingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_witness_keeping")
}
