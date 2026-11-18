package oracle

import "testing"

// CR 119.4: a payment's summed life never reaches the payer's life, so the
// offer census holds a sole life-only mana unit (Treasonous Ogre's "Pay 3
// life: Add {R}") only while life - the earlier life-only units' committed
// costs > its own. The kernel's offer avail counted each life-only unit
// alone (painAvailMask's per-source guard), so with two Ogres and 4 life it
// priced two activations into a {2}{B} may-play cast (Moment of Defiance's,
// granted by Act on Impulse) that a real payment cannot fund: gorge's
// planner proved it insufficient and dropped the cast (decision 607 of 607;
// dr-mbc-bbbb pair 32, seed 18425834651941190307).
// The recording is on a custom pool, the table its own decks need: the tail
// buckets a synthetic pool deals re-cut whenever the lowered card set moves.
func TestMBCLifeOnlySummedLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_only_sum_lethal")
}
