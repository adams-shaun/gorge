package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The equip cost-reduction lockstep: "equip abilities you activate that
// target this creature cost {N} less" (Cloud, Planet's Champion; Dwarven
// Mauler; Helitrooper; Fervent Champion) beside "equip abilities you activate
// cost {1} less" (Auriok Steelshaper), with Bonesplitter's {1} equip to move
// around (CR 601.2c, 601.2f, 702.6a). The offer prices the cheapest target,
// the target menu each candidate, the charge the chosen one; the expected
// answer at every decision is gorge's.
var kequipDecks = [2][]cardset.Entry{
	{
		{Name: "Cloud, Planet's Champion", Count: 3}, {Name: "Dwarven Mauler", Count: 3},
		{Name: "Auriok Steelshaper", Count: 2}, {Name: "Fervent Champion", Count: 2},
		{Name: "Bonesplitter", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Cloud, Planet's Champion", Count: 2}, {Name: "Dwarven Mauler", Count: 2},
		{Name: "Helitrooper", Count: 2}, {Name: "Bonesplitter", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

// 40 seeds fit the 2-minute test cap; KREDUCEEQUIP_SEEDS=1000 is the full
// sweep. AbReducedPriced is shared with the other lockstep tests, so its
// delta is logged evidence; TestReduceCostEquipMutation is the proof.
func TestReduceCostEquipLockstep(t *testing.T) {
	t.Parallel()
	before := dirac.AbReducedPriced.Load()
	lockstepSet(t, "reduceequip", kequipDecks, 40, "KREDUCEEQUIP_SEEDS")
	t.Logf("%d ability pricings lowered by a StatReduceAbility static", dirac.AbReducedPriced.Load()-before)
}

// TestReduceCostEquipMutation proves the pool charges a reduced equip: with
// the reduction dropped, some seed diverges.
func TestReduceCostEquipMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoAbReduce = true
	defer func() { dirac.DebugNoAbReduce = false }()
	if !lockstepDiverges(t, "reduceequip", kequipDecks, 40) {
		t.Fatal("no seed diverged with the ability reduction dropped: the pool never activates a reduced equip")
	}
}
