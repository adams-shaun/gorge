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
}
