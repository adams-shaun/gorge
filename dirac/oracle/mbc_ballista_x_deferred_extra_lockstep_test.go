package oracle

import (
	"testing"
)

// TestMBCWalkingBallistaReprosLockstep replays the recorded dr-mbc-abbb game
// (pool deal focus_00027_WBR vs focus_00329_WBR, seed 531060, pair 32): p0
// casts Walking Ballista ({X}{X}) with three untapped Swamps, a Three Tree
// City and a Gemstone Caverns (no luck counter) beside an opponent's Mana
// Flare. Gemstone Caverns is a Condition-split source the payment planner
// defers, so the kernel's X window read it through its deferred-source arm
// (lockstepconfig.go availXWindow) -- and that arm priced the tap's reflected
// extra (CR 106.12a: Mana Flare adds one mana of the type the land produced)
// on none of the arm's units. gorge's window census folds the extra into every
// priced alternative (cast_window.go windowTapExtras), so the X ask's reach
// was one {C} short: gorge offered X 0..5, the kernel 0..4.
func TestMBCWalkingBallistaReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/ballista_x_deferred_extra.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "custom" {
		t.Fatalf("the recording plays pool %s, want custom", r.Pool)
	}
	stdReproNamed(t, "ballista_x_deferred_extra")
}
