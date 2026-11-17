package oracle

import (
	"path/filepath"
	"testing"
)

// CR 601.2g, 602.2b, 605.3b: Throne of Bone's "you may pay {1}" trigger cost is
// offered on gorge's potential-mana bound, and a storage battery (Subterranean
// Hangar: {T}, remove any number of storage counters: add {B} for each) counts
// there whatever counters it holds. The kernel's trigger-cost gate counts the
// battery as a mana source too, and the oracle answers the battery's window X
// at the maximum, so both sides offer the same choose:trigger_cost_pay and the
// recorded game (tail-16 pair 32, decision 277) replays clean on a table of its
// own decks' cards.
func TestMBCThroneOfBoneReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "throne_of_bone_pay_over_a_storage_battery_unmodelled.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res := replayReproValue(t, synthReg(t), printedDelveSet(t, r.Cards), r)
	stallCheck(t, r, res, true)
}
