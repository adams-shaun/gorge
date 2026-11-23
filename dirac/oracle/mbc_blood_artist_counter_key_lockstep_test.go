package oracle

import "testing"

// A prefix search that sacrifices two Prism tokens (Diamond Kaleidoscope) with
// A-Blood Artist and Gourmand's Talent out stacks Blood Artist twice; the
// Gourmand trigger that puts a +1/+1 counter on each creature leaves the pool,
// the stack size and every tapped bit as an earlier node had them, so only the
// counters tell the two payer states apart (gorge's scriptStateKey keys them).
// dirac dropped the node as a visited one and priced neither Thanos, Death's
// Consort nor Uurg, Spawn of Turg at decision 820 (tail-12 pair 1, seed 787572).
func TestMBCThanosDeathSConsortReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blood_artist_gourmand_counter_key")
}
