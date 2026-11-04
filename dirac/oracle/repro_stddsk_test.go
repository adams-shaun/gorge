package oracle

import (
	"path/filepath"
	"testing"
)

// Games from std-dsk's synthetic pool that once ended in an unmodelled stall.
// The name ends in "Stall" because land already declares its own
// TestStdDskReprosLockstep replay table; stdReproLockstep checks the stall as
// well as the divergence, so one table serves both. The -run pattern the
// ticket names still matches this test.
func TestStdDskReprosLockstepStall(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"optional_discard_two_exile_sources", // CR 616.1: an optional discard of a card two Leylines of the Void would exile is ordered, not moved at once
	} {
		t.Run(name, func(t *testing.T) {
			stdReproLockstep(t, filepath.Join("testdata", "repro", name+".repro.json"), "std-dsk")
		})
	}
}
