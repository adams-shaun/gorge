package oracle

import "testing"

// CR 616.1: an edict's sacrificed creature goes to the graveyard under two
// Rest in Peace, so its owner orders the two exile-instead replacements, in
// gorge and in the kernel alike (dr-mbc-aaba pair 30; the kernel stalled at
// replleave).
func TestMBCDrMbcAabaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "edict_sac_two_rest_in_peace_asks_order")
}
