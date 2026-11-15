package oracle

import "testing"

// Join forces (Minds Aglow): "each player may pay any amount of mana" poses a
// choose-number ask whose options are the amounts the payer can afford. The
// recording is tail-06 pair 141 (decision 168 of 168), where gorge offered
// only mode(-,-,0) and the kernel offered mode(-,-,0) and mode(-,-,1).
func TestMBCMindsAglowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "minds_aglow_join_forces_amount_offer")
}
