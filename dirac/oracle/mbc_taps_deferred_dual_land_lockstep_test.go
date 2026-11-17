package oracle

import "testing"

// Blight on Haunted Mire: the enchanted dual land's tap fires an unvetted
// Taps watcher (destroy it), so gorge's planner defers it and its autopay
// surface never scripts a plain cast through a deferred source. Geralf,
// Visionary Stitcher ({2}{U}) is payable only with the Mire beside an Island
// and a Swamp; gorge did not offer it, dirac counted the Mire as an ordinary
// census unit (decision 172 of 172). The recording is on its own decks'
// table, since the tail pool it came from is re-dealt as cards are lowered.
func TestMBCGeralfVisionaryStitcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_dual_land_is_a_census_gap")
}
