package oracle

import "testing"

// TestMBCNecroticOozeGainedOffZoneStallReprosLockstep: Necrotic Ooze
// (GainsAbilitiesOfZones$
// Graveyard) sits on the battlefield when Oliphaunt's cycling activation
// discards it into the graveyard (tail-61-c001, pair 9, decision 1006). The
// cycling ability names the hand as the only zone it functions from (CR
// 702.29a, CR 113.6b), so the copy the Ooze gains on the battlefield
// functions nowhere and neither engine offers it; the kernel's gains-of walk
// once stalled StallGainsOf on it instead of skipping it. The recording ends
// in that stall; the replay continues past its 1005 scripted decisions on
// FirstPolicy and must reach the game's next ask without one. The
// TestMBCNecroticOozeReprosLockstep name is taken by the pair's earlier
// defect (the offer window, mbc_gained_ability_window_lockstep_test.go), so
// this one names the cause.
func TestMBCNecroticOozeGainedOffZoneStallReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gained_offzone_ability_stalls")
}
