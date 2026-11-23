package oracle

import "testing"

// CR 601.2b, 702.33c: Batroc the Leaper's multikicked cast was offered by the
// kernel at priority and not by gorge (tail-49-c000 pair 29, decision 1396 of
// 1396, main fd13a69db1) until the fee-repeat pricing of a repeatable fee-only
// mana ability reached the cast-offer gate. The recording is the shrunk game.
func TestMBCBatrocTheLeaperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "multikicked_cast_offer_batroc")
}
