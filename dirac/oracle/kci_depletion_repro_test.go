package oracle

import "testing"

// TestReproKCIDepletionLandLockstep replays the recorded Krark-Clan Ironworks
// games on the dr-mbc-bb pool: a no-{T} sacrifice-an-artifact mana source
// beside a depletion land (the pool's SubCounter riders, Peat Bog and
// Sandstone Needle among them) offers and pays the same on both sides — the
// tap-another search branches the deferred gap like gorge's one search, and
// its node reads gorge's offer. The recordings were the run's divergent pairs
// 9 and 31 (gorge offers the cast the kernel did not) and 41, 46 and 48 (the
// two searches paid with different sources, the depletion land sacrificed in
// gorge, Krark-Clan Ironworks in its graveyard); each replays clean now.
func TestReproKCIDepletionLandLockstep(t *testing.T) {
	stdReprosGlob(t, "dr-mbc-bb")
}
