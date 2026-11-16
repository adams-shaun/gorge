package oracle

import "testing"

// Lae'zel, Vlaakith's Champion rewrites the counters its controller gets as
// well as those on their creatures and planeswalkers ("or on yourself"): the
// experience counter Otharri, Suns' Glory's attack trigger gives comes with
// one more, so the trigger makes one more 2/2 Rebel tapped and attacking
// (CR 122.6). The kernel ignored the player half and made one token fewer
// (decision 40: projection NInst, ref 46 vs kernel 45). The recording is on a
// custom pool: the table its own decks need, since no tail pool holds both.
func TestMBCOtharriReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "otharri_experience_counter_replaced_by_laezel")
}
