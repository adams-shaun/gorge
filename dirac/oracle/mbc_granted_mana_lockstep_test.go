package oracle

import "testing"

// CR 305.1 / 605.1a: Citanul Hierophants gives every creature its controller
// has "{T}: Add {G}", so an untapped Woodland Weavemaster and Prosperous
// Innkeeper each pay one {G} of Courser of Kruphix's {1}{G}{G} alongside a
// fresh Forest. gorge offered the cast; the kernel offered only pass.
func TestMBCCourserOfKruphixReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_creature_mana_ability_pays_cost")
}
