package oracle

import "testing"

// CR 616.1: two Worships on the battlefield both floor the damage an attacker
// deals to their controller. Every order leaves the life total at the same
// floor, so gorge poses no replacement-order ask for them and the kernel's scan
// order agrees (Agent of Acquisitions' attack with two Worships in play; gorge
// once asked "replacement" where the kernel gave priority, decision 803).
func TestMBCAgentOfAcquisitionsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "two_life_floors_need_no_order_ask")
}
