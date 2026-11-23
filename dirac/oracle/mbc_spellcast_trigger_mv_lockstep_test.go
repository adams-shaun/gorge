package oracle

import "testing"

// CR 608.2h with CR 202.3e: Aetherflux Car's "whenever you cast a spell, put
// charge counters equal to four times that spell's mana value" still reads the
// X the spell was cast with when a counterspell removes the spell before the
// trigger resolves (Death Denied cast with X=2, countered by Failed
// Inspection: 16 counters). The kernel read the card's printed value, 8
// (decision 305 of tail-36-c000 pair 27).
func TestMBCAetherfluxCarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spellcast_trigger_mv_keeps_x_after_counter")
}
