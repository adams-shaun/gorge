package oracle

import (
	"path/filepath"
	"slices"
	"testing"
)

// TestComboAnyGapLockstep: Axebane Guardian's {T}: Add X mana in any combination of colors is
// a census gap in gorge's payment planner (no colour list for an any-combination production),
// so a play it must fund is priced by a prefix script. The recorded game, where only the
// kernel offered a plain cast the Axebane mana funds, replays clean on a raw table.
func TestComboAnyGapLockstep(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "axebane_combo_any_census_gap.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.Cards[0], "Axebane Guardian") && !slices.Contains(r.Cards[1], "Axebane Guardian") {
		t.Fatal("precondition: the recorded decks hold no Axebane Guardian")
	}
	res := replayRawRepro(t, "axebane_combo_any_census_gap")
	if res.Decisions == 0 {
		t.Fatal("the recorded game posed no decision")
	}
	if !res.Clean() {
		t.Fatalf("diverged or stalled unmodelled: div=%+v stall=%q", res.Div, res.Stall)
	}
}
