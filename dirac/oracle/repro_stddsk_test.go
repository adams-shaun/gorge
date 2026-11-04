package oracle

import (
	"path/filepath"
	"testing"
)

// Games from std-dsk's synthetic pool that once diverged: each replayed game
// now agrees, and its script reaches no unmodelled stall.
func TestStdDskReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"optional_discard_two_exile_sources", // CR 616.1: an optional discard of a card two Leylines of the Void would exile is ordered, not moved at once
	} {
		t.Run(name, func(t *testing.T) {
			stdReproLockstep(t, filepath.Join("testdata", "repro", name+".repro.json"), "std-dsk")
		})
	}
}
