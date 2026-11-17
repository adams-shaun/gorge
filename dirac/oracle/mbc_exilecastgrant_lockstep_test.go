package oracle

import "testing"

// Vivien, Champion of the Wilds' -2 exiles one of the top three cards face
// down; "for as long as it remains exiled, you may look at that card and you
// may cast it if it's a creature spell" (CR 406.3, 601.2). gorge offers the
// cast of the exiled Chieftain en-Dal; the kernel lowered the permanent Effect
// to an emblem static over the resolution's remembered set (emptied by the
// chain's Cleanup), so it never offered the cast (decision 382 of 382,
// options options, tail-20-c000 pair 7). The grant is now a per-card record
// made only for a creature card.
func TestMBCChieftainEnDalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_cast_grant_creature_filter")
}
