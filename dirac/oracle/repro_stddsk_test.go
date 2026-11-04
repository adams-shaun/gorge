package oracle

import (
	"path/filepath"
	"testing"
)

// Games from std-dsk's synthetic pool that once ended in a stall. The name
// ends in "Stall" because land already declares its own TestStdDskReprosLockstep
// replay table. Any stall, declared or not, fails: a capacity stall from the
// queue helpers is still a regression. The -run pattern the ticket names still
// matches this test.
func TestStdDskReprosLockstepStall(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"optional_discard_two_exile_sources", // CR 616.1: an optional discard of a card two Leylines of the Void would exile is ordered, not moved at once
	} {
		t.Run(name, func(t *testing.T) {
			res := stdReproPlay(t, filepath.Join("testdata", "repro", name+".repro.json"), "std-dsk")
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v\n gorge %v\n dirac %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
			}
			if res.Stall != "" {
				t.Fatalf("dirac stopped at a stall: %s", res.Stall)
			}
		})
	}
}
