package oracle

import "testing"

// Young Blue Dragon's Adventure, Sand Augury {1}{U}, beside the opponent-free
// board of Island, two Treasures and Furnace Celebration ("whenever you
// sacrifice another permanent, you may pay {2}"): gorge lowers the witness
// before the cast, the sacrificed Treasure's trigger resolves with the mana
// floating and the first option pays it, so the cast is never made. The
// kernel offered castface anyway (tail-67-c000, autopay, seed 248371, pair
// 176, decision 245, options); it now withholds it (face_lowereats.go).
func TestMBCYoungBlueDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "young_blue_dragon_castface")
}
