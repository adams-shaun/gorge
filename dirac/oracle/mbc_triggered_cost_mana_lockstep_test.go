package oracle

import "testing"

// Lapis Lazuli Talisman's "you may pay {3}" trigger cost, funded by mana
// abilities in the manual payment window, once livelocked gorge on a repeating
// mana_add / choose cycle (tail-49-c000 pair 47, decision 351).
func TestMBCLapisLazuliTalismanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "triggered_cost_mana_window_livelock")
}
