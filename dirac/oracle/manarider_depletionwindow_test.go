package oracle

import (
	"path/filepath"
	"testing"
)

// The depletion rider activated inside a manual payment window, not at
// priority (kernel mana_window.go windowPick's rider arm): gorge's window
// lists every usable mana ability in its battlefield place and its answer
// takes the first listed, so the window taps the rider source there --
// removing the counter, adding its mana and running the rider -- beside the
// plain lands. The recorded games cover the three windows that meet the
// class: a madness cast's CR 601.2g window, a plain cast's, and a ward
// payment's (payWindow). The tables are built from each recording's own
// decks (pool "custom"); the games were recorded by the kfuzz evidence sweep
// over dr-mbc-aabb, gorge leading, and replay clean.
func TestManaRiderDepletionWindowRepros(t *testing.T) {
	reg := synthReg(t)
	for _, name := range []string{
		"mana_rider_hickory_woodlot_pays_a_madness_cast_through_the_window",
		"mana_rider_hickory_woodlot_pays_a_cast_through_the_window",
		"mana_rider_hickory_woodlot_pays_a_ward_through_the_window",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			if r.Surface != "autopay" {
				t.Fatalf("recorded on %q, want autopay: the class is a surface question", r.Surface)
			}
			stallCheck(t, r, replayReproValue(t, reg, globSet(t, r.Pool, r), r), false)
		})
	}
}
