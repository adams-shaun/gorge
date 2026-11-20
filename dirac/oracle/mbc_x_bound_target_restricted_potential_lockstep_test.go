package oracle

import "testing"

// CR 106.6: Base Camp's party-only any-colour mana cannot pay Sequence
// Engine's artifact activation. With only two Forests and Base Camp's
// colourless ability untapped, an announced X=4 has no usable source, so the
// ability is not offered. The kernel priced its X-bound target sweep from the
// unfiltered potential pool (which reads an open production as unlimited) and
// offered the activation anyway (tail-29 pair 82, decision 205): gorge's
// options were [pass].
func TestMBCSequenceEngineReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_bound_target_restricted_potential")
	// CR 601.2b/601.2c/601.2h and 602.2b: the mirror case. One Treasure Token
	// makes ONE mana of any colour, but gorge's potential bound priced the
	// open production as unbounded in every slot, so it offered X=3 with a
	// mana value 3 creature card in the graveyard (tail-29 pair 33, decision
	// 588) while the kernel, which prices the announcement exactly, did not.
	stdReproNamed(t, "sequence_engine")
	// gorge offered the activation at a priority decision where the kernel
	// offered only pass (tail-29-c000 pair 17, decision 325).
	stdReproNamed(t, "sequence_engine_activation_offer")
}
