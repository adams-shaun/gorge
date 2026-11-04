package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Mirror Universe Intrusion's two target slots are sized by its kicker: one
// creature an opponent controls unkicked, one opponent kicked (CR 601.2c,
// 702.33d), whose creatures are then each copied with menace through the
// Valid Creature.TargetedPlayerCtrl set. The opponents' creatures include
// legendary ones, so a copy faces the legend rule only beside its own
// controller's original (CR 704.5j). Some kicked cast must copy two or more.
func TestCopyTargetedPlayerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Mirror Universe Intrusion", Count: 10}, {Name: "Tenacious Drone", Count: 4},
		{Name: "Island", Count: 13}, {Name: "Swamp", Count: 13},
	}
	b := []cardset.Entry{
		{Name: "Academy Instructor", Count: 4}, {Name: "Savvy Profiteer", Count: 4},
		{Name: "Gwyndala, the Progeny", Count: 4}, {Name: "Gray and Adira, Forever Joined", Count: 4},
		{Name: "Mirror Universe Intrusion", Count: 2},
		{Name: "Island", Count: 11}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 4},
	}
	var multi uint32
	dirac.DebugDefValidMulti = &multi
	defer func() { dirac.DebugDefValidMulti = nil }()
	fixtureLockstep(t, "std-trk", [2][]cardset.Entry{a, b}, 300)
	if multi == 0 {
		t.Fatal("no kicked cast copied two or more creatures")
	}
	t.Logf("kicked casts that copied two or more creatures: %d", multi)
}
