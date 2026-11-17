package oracle

import "testing"

// TestStorageBatteryCounterMana replays the pair-149 recording (tail-42, seed
// 1008573527069939009): gorge's Mite Overseer activation ({3}{W/P}) is funded
// by its Subterranean Hangar's storage battery ("{T}, Remove any number of
// storage counters: Add {B} for each", SubCounter<X/STORAGE>), the census gap
// the Phyrexian offer gate prices through the script route
// (rules/potential_script.go potentialPrefixScript): the script's prefix
// activates the battery at priority, before the pip asks, and gorge announces
// X at the counters it holds. The kernel's plain probes (plainOptions) read
// the battery's units through the same offer gate (mana_phyrexian.go
// phyStoreUnits) and listed the play as plain-priced, so no prefix ran and the
// pip ask was posed with the Hangar untapped, its counter and gorge's {B}
// missing (the recording's divergence: Cards[77].Flags, Counters[62],
// Pool[1][2]). The probes now read without the batteries (availPlain, the
// probe walks' mark in plainOptions) and
// the tapX search's replay activates the battery (mana_countercost.go
// storeGap) exactly where gorge's script does.
func TestStorageBatteryCounterMana(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "repro/storage_battery_counter_mana", "tail-42", autopay)
}
