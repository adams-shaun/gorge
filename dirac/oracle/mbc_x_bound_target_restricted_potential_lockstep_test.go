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
	// CR 601.2c: the target pool. gorge offered activate(#N,-,0) at every
	// priority window with the Engine untapped even though no graveyard held a
	// creature card of any affordable mana value X, so no value of X has a
	// legal target and the activation is not a legal action; the kernel
	// offered only pass (tail-29, decision 410).
	stdReproNamed(t, "activate_x_offer_without_legal_target")
	// gorge offered the activation at a priority decision where the kernel
	// offered only pass (tail-29-c000 pair 17, decision 325).
	stdReproNamed(t, "sequence_engine_activation_offer")
	// CR 601.2b, 601.2c, 602.2b: gorge offered the activation at a priority
	// decision where the kernel offered only pass (tail-29-c000 pair 44,
	// decision 338): the Electrostatic Pummeler (mana value 3) in a graveyard.
	stdReproNamed(t, "x_bound_graveyard_target_counts_elf_mana")
}
