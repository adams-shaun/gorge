package oracle

import "testing"

// TestMBCTail36ReprosLockstep replays the tail-36 pool's outsidetype stall:
// Xenograft and Kindred Discovery chose the same outside creature type in a
// pool with more outside types than free byte codes, so the kernel's
// compressed code was aliased and it stopped instead of comparing the two
// choices. CR 205.3m, 613.1d: the chosen type is a per-object characteristic
// (layer 4); the per-game exact code proves the two sources equal, and two
// different outside types unequal.
func TestMBCTail36ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "outsidetype")
}
