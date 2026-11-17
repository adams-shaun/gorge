package oracle

import "testing"

// CR 400.7 with CR 612: an activated ability minted from a text-changed face
// keeps projecting its Face.Abilities index after the source leaves play (its
// cost exiles or sacrifices it and the zone change clears TextChanges), as the
// kernel's stack entry does. gorge once projected 255 (Campfire, Mind Bend in
// the deck; decision 167 of 167; dr-mbc-aaab pair 40).
// The recording is pooled custom (the table its own decks need): the synthetic
// bucket it was found on re-cuts whenever the lowered card set moves.
func TestMBCActSourceLeavesTextChangedFaceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_source_leaves_text_changed_face")
}
