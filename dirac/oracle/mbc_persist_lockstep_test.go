package oracle

import "testing"

// CR 702.77: Persist is gorge kwPersist's dies trigger (kgen expandedKeywords),
// not a battlefield characteristic, so the chars projection lists it in
// gorgeKwElsewhere like its mirror Undying. Before it did, Scuzzback Marauders
// entering the battlefield was a projection error (decision 383 of tail-33
// pair 439: gorge keyword "Persist" has no kernel bit).
func TestMBCScuzzbackMaraudersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "persist_has_no_chars_bit")
}
