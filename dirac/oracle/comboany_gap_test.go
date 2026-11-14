package oracle

import "testing"

// TestComboAnyGapLockstep: Axebane Guardian's {T}: Add X mana in any combination of colors is
// a census gap in gorge's payment planner (no colour list for an any-combination production),
// so a play it must fund is priced by a prefix script. The recorded game, where only the
// kernel offered a plain cast the Axebane mana funds, replays clean on a raw table.
func TestComboAnyGapLockstep(t *testing.T) {
	res := replayRawRepro(t, "axebane_combo_any_census_gap")
	if res.Decisions == 0 {
		t.Fatal("the recorded game posed no decision")
	}
	if !res.Clean() {
		t.Fatalf("diverged or stalled unmodelled: div=%+v stall=%q", res.Div, res.Stall)
	}
}
