package oracle

import "testing"

// tail-08 pair 10 (seed 13196970261271822978): p1's Dwarven Hold holds two
// storage counters, and its "{T}, Remove X storage counters: Add {R} for each
// removed" funds The World Tree's "{W}{W}{U}{U}{B}{B}{R}{R}{G}{G}, {T},
// Sacrifice The World Tree: Search your library for any number of God cards":
// eight other untapped lands make eight mana of any colour (the Tree's own
// static grants every land the ability) and the battery's two red mana
// complete the ten pips, with the Tree's {T} paid by its own tap (CR 601.2g,
// 601.2h). gorge's payment planner priced no SubCounter<X> cost shape, so its
// prefix search (potentialPrefixScript) could not reach the funding line
// within its node budget and withheld the activation the kernel offered
// (decision 652); the search's subsume run narrows the activation asks enough
// to walk it. (The name is not TestMBCTheWorldTreeReprosLockstep: that one
// mbc_grant_store_gap_lockstep_test.go already took, for the same pair's
// kernel-side divergence at decision 648.)
func TestMBCStorageBatteryFundsActivationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_activation")
}
