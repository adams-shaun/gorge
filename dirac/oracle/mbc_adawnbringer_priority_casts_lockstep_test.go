package oracle

import "testing"

// CR 601.2g: a cast a seat's floating pool cannot cover is still legal when
// its payment can activate mana abilities while casting, so gorge's canonical
// priority vocabulary offers it as a potential play (Engine.PotentialPaymentPlans,
// doc.go). A dynamic {T}, sacrifice source in play -- Lotus Blossom's
// "sacrifice: add X mana of any one color, X is the verse counters" -- funds
// the white creatures in p1's hand over three untapped Mountains, and the
// kernel priced none of them (options options, pair 7 of synth tail-74 seed
// 108741 at land 96bfdd6f5b).
func TestMBCADawnbringerClericReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "adawnbringer_priority_casts")
}
