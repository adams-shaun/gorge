package oracle

import "testing"

// Battletide Alchemist's "you may prevent X of that damage" against damage to
// a player (pool tail-76 pair 90, surface autopay): at decision 391 the two
// engines offered the replacement decision different menus, gorge only the
// Alchemist and the kernel the Alchemist plus a source-less entry. The
// recording once ended at the kernel's declared replplus capacity stall; the
// combat walk poses the may at the event now, so the game replays past it.
func TestMBCALanternOfRevealingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "optional_damage_prevention_decline")
}
