package oracle

import (
	"testing"
)

// TestMBCElasIlKorSadisticPilgrimReprosLockstep replays the recorded
// dr-mbc-abbb game (pair 45, seed 298377): p1 holds Evincar's Justice
// ({2}{B}{B}) over a lucky Gemstone Caverns (a Condition-split source the
// payment planner defers) and a Plains beside its own Mana Flare. The Caverns'
// tap makes one mana of any colour and the reflection one more of that colour
// (CR 106.12a), the Plains' {W} and its reflection pay the {2}, so the cast is
// legal. gorge's planner folds the reflected extra into the tap's production,
// so it offers the cast; the kernel's Taps-deferred gap search floated the
// production alone and stalled on the pick-priced extra, dropping the offer
// (options options, decision 1014). The recording plays the deck cards' table
// (pool custom, the ballista_x_deferred_extra shape).
func TestMBCElasIlKorSadisticPilgrimReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/tapsdefer_reflected_pick_float.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "custom" {
		t.Fatalf("the recording plays pool %s, want custom", r.Pool)
	}
	stdReproNamed(t, "tapsdefer_reflected_pick_float")
}
