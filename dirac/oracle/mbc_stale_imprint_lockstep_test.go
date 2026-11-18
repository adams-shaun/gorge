package oracle

import "testing"

// CR 400.7 with CR 607.2a: Phyrexian Ingester's enters trigger was on the stack
// when Blur exiled and returned it, so the returned Ingester is a new object.
// The old trigger still resolves and exiles a creature, but that exiled card is
// linked to the old object: the new Ingester gets no +X/+Y from it. Gorge once
// imprinted the card on the new Ingester (toughness 7 against dirac's 3, decision
// 1100 of 1100; dr-mbc-aaab pair 33).
func TestMBCPhyrexianIngesterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stale_imprint_trigger_after_blink")
}
