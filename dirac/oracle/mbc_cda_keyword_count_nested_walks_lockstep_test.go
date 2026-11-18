package oracle

import (
	"testing"
	"time"
)

// CR 613 with CR 604.3: a layer-7 amount that counts by keyword (Pride of the
// Clouds' "+1/+1 for each other creature with flying") reads the keywords
// layer 6 left on the candidates. Building that table derived each candidate
// with a FULL Derived walk, so every candidate carrying its own layer-7 CDA
// re-ran its own count -- which built another keyword table over the whole
// board again, the walks multiplying in the number of Prides on the board
// (tail-20-c001 pair 72: 54s CPU, past the lockstep run's 30s game watchdog
// twice, recorded as a hang by both sides). The keywords layer 7 cannot feed,
// so the table now reads each candidate's keyword-only walk and the game
// replays in under a second. The recording replays clean either way (the
// answers were never wrong), so the bound is the pin: a regression past it
// means the table is deriving candidates fully again.
func TestMBCTail20ReprosLockstep(t *testing.T) {
	t.Parallel()
	start := time.Now()
	stdReproNamed(t, "cda_keyword_count_nested_walks")
	if d := time.Since(start); d > 30*time.Second {
		t.Fatalf("the CDA keyword-count game replayed in %s: a layer-7 amount's keyword table derived candidates with full walks again (each nested CDA count re-deriving the board)", d)
	}
}
