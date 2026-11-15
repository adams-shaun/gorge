package oracle

import "testing"

// Big Bertha ({X}{G}) is castable at X=0 with one untapped Forest; gorge
// offered the cast at priority and the kernel did not (decision 349 of
// tail-57 pair 351). The decks are the pair's own, so the table is built from
// them (tail-57 no longer holds every card in them).
func TestMBCBigBerthaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_spell_cast_offer_one_land")
}
