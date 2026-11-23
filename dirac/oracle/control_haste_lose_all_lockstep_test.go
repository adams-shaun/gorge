package oracle

import "testing"

// Edea's "gain control of target creature an opponent controls ... It gains
// haste until end of turn" (a control record's GcHasteEOT grant), then a later
// Dragonshift on the stolen Polygoyf (Animate RemoveAllAbilities$ plus Flying):
// the loss is newer than the haste grant, so CR 613.1f / 613.7 strips Haste and
// leaves only Flying. gorge did; the kernel's non-walk keyword path
// (kw / kwGen) added the control record's haste back after the loss (kfuzz
// tail-01 pair 175, `projection Cards[i].Chars.Keywords` ref=1 kernel=257).
func TestControlHasteGrantRemovedByLaterLoseAllLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "control_haste_grant_removed_by_later_lose_all")
}
