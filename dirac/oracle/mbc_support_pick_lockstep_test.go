package oracle

import "testing"

// CR 701.41a: support puts counters on up to N creatures other than the
// permanent whose ability it is. Expedition Raptor's enters trigger posed
// a counter_pick option for the Raptor itself on one side (dr-mbc-abab pair
// 17, decision 513 of the recording).
func TestMBCExpeditionRaptorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "support_counter_pick_other_than_source")
}
