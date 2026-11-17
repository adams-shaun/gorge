package oracle

import "testing"

// CR 613.1c with CR 613.3 (colour in layer 5, keywords in layer 6): Aphotic
// Wisps' "target creature becomes black and gains fear until end of turn"
// stops Bloodmark Mentor's own "Red creatures you control have first strike"
// from applying to the Mentor itself, because the layer-6 static sees the
// post-layer-5 colour. gorge's projection once kept the stale first strike
// (decision 784 of 784; dr-mbc-bbab full, m0).
// The recording is pooled custom (the table its own decks need): the dr-mbc
// bucket it was found on re-cuts whenever the lowered card set moves.
func TestMBCKutzilMalametExemplarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kw_grant_survives_colour_set")
}
