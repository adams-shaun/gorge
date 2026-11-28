package oracle

import "testing"

// A planned other-sacrifice payment may spend the spell's only target: CR
// 601.2c-h announces the target before the costs are paid, and gorge's
// payment planner names the victim before the target is asked. The kernel's
// keep-targets census withheld the cast instead.
//
// tail-71 pair 67, decision 263, seed 7626018148801502969, surface autopay:
// Ashnod's Altar sacrifices Mephitic Ooze, Clinging Darkness's only target,
// and gorge offers the cast where the kernel offers only pass.
// tail-71 pair 68, decision 136, seed 16104784413530517364, surface autopay:
// the same plan funds Clinging Darkness and Ob Nixilis's Cruelty over
// Strongarm Thug, their only target.
// tail-71 pair 96, decision 215, seed 13969559076747832278, surface autopay:
// the plan sacrifices Molten Hydra, Thunderous Might's only target.
func TestMBCSacOnlyTargetLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "altar_victim_is_the_only_target_cast_offer")
	stdReproNamed(t, "altar_victim_is_the_only_target_two_casts")
	stdReproNamed(t, "altar_victim_is_the_only_aura_target_cast_offer")
}
