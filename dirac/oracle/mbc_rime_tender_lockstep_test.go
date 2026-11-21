package oracle

import "testing"

// CR 205.3 with CR 613.1d: Fowl Play on Rime Tender (decision 309 of tail-39-c000
// pair 30) removes the Human Druid creature types and adds Bird. The two engines
// parted on the first subtype in the projection.
func TestMBCRimeTenderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fowl_play_replaces_creature_subtypes")
}
