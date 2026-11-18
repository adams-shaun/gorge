package oracle

import "testing"

// CR 616.1 with CR 704.3: three exile-instead Samurai of the Pale Curtain
// around a combat that kills five creatures at once. The Aura on one of them
// is orphaned by the first answer, and a Golem token loses Master Splicer's
// bonus and becomes lethally damaged by the fourth. Gorge builds the next
// batch only after the first is answered, so the Golem (a creature casualty)
// is asked before the Aura, and with the sources still on the battlefield by
// then. The kernel queued the Aura at the first recheck, ahead of the Golem and
// with the already exiled Samurai among its sources (dr-mbc-baaa pair 41).
func TestMBCSamuraiOfThePaleCurtainReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_order_next_batch_sorted_by_family")
}
