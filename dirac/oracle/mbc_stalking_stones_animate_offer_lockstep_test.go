package oracle

import "testing"

// CR 106.6, 601.2g: Stalking Stones' {6} animate beside Woodland Weavemaster's
// Elf-only dynamic mana and Citanul Hierophants' granted {T}: Add {G}. gorge's
// prefix search visits the Weavemaster's printed branch (its mana floats
// unspendable) before the granted {G} answer, whose pool is the same payer
// state; the node still offers the activation from the pool and the printed
// sources, so the kernel must offer it too (decision 587 of 587, tail-72 pair
// 9 seed 222108).
func TestMBCStalkingStonesAnimateOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stalking_stones_animate_offer_restricted_source_grant")
}
