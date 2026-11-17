package oracle

import "testing"

// CR 715.4 with CR 603.3d: Young Red Dragon rests in the adventure zone and
// its owner's only fourth mana is an Eldrazi Scion, whose sacrifice (the
// planner's witness) triggers the opponent's Falkenrath Noble. The Noble's
// target is the opponent's decision, asked mid-lowering. The gorge side
// aborted the lowering on that foreign decision, dropped the exile recast as
// "lower dead end", and left the kernel's castface the only offer (dr-mbc-bbaa
// pair 96, decision 339).
func TestMBCYoungRedDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lowering_answers_other_seat_trigger_target")
}
