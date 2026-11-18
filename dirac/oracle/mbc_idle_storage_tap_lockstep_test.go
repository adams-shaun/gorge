package oracle

import "testing"

// CR 601.2g-h, 605.1a, 107.3: Treasure Trove's {2}{U}{U} beside The World
// Tree's any-colour grant on every land and a Dwarven Hold holding no storage
// counters. The Hold is a grant holder, so gorge's one prefix search lists it,
// and its activation ask offers the printed storage ability (X = 0: the {T}
// alone, no mana) before the grant: the search takes that branch, taps the
// Hold for nothing and goes on through the lands in battlefield order, so
// Sunlit Marsh and a Mountain tap too and a red unit floats. The kernel's
// search took the grant's mana from the Hold and stopped two taps earlier
// (decision 725, tail-08 pair 10, autopay).
func TestMBCIdleStorageTapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "idle_storage_battery_tap_beside_grants")
}
