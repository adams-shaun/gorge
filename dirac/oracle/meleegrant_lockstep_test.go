package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Glory to the Empire! gives the creatures its controller controls melee
// (CR 702.121a): each attacks with a +1/+1 pump trigger, one per Glory (CR
// 702.121b). gorge queues a granted Melee in its own keyword walk, so its
// place beside an AddTrigger$ grant (Bio-Asset Allocator's on an artifact
// creature, The Sword of Kahless's on the equipped creature) depends on the
// holder's printed face: Reclamation Specialist and Defense Force Aggressor
// have no line that observes the attack and list the Melee trigger first,
// Qo'noS Quartermaster and Gnarling Targ have an attack trigger and list it
// after the granted ones. The trigger-order choice indexes that list.
func TestMeleeGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Glory to the Empire!", Count: 4}, {Name: "Bio-Asset Allocator", Count: 3},
		{Name: "Qo'noS Quartermaster", Count: 3}, {Name: "Defense Force Aggressor", Count: 3},
		{Name: "Warship Flight Crew", Count: 2}, {Name: "Reclamation Specialist", Count: 3},
		{Name: "The Sword of Kahless", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	}
	b := []cardset.Entry{
		{Name: "Glory to the Empire!", Count: 4}, {Name: "Blood-Sport Champion", Count: 4},
		{Name: "Mek'leth Berserker", Count: 3}, {Name: "Gnarling Targ", Count: 3},
		{Name: "Bio-Asset Allocator", Count: 2}, {Name: "Reclamation Specialist", Count: 3},
		{Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 10},
	}
	fixtureLockstep(t, "std-trc", [2][]cardset.Entry{a, b}, 300)
}
