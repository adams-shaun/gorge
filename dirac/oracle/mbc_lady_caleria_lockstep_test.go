package oracle

import "testing"

// CR 601.2g, 605.3a: Lady Caleria ({3}{G}{G}{W}{W}) beside a Wooded Bastion
// and a Flooded Grove (hybrid-fee filter lands, each with a free {C} ability)
// is payable only by activating a filter. gorge's prefix search poses a
// filter's flattened ask with the colour the cost names ahead of {C}; the
// kernel walked {C} first, spent the 160-node budget there and withheld the
// cast (decision 403, tail-15-c000 pair 12).
func TestMBCLadyCaleriaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_needed_colour_ask_first")
}
