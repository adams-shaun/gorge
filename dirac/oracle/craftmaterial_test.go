package oracle

import "testing"

// A crafted back face counts the material of its latest craft only: Jadeheart
// Attendant gains life equal to the mana value of the card exiled to craft it
// (CR 702.167a), and an Attendant that was bounced, recast as Jade Seedstones
// and crafted again is a new object (CR 400.7) whose first craft's material is
// no longer exiled with it.
func TestCraftedAgainCountsLatestMaterialLockstep(t *testing.T) {
	t.Parallel()
	stdReproLockstep(t, "testdata/craft_second_material.repro.json", "std-lci")
}
