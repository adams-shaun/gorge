package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Curzon Dax, Bound by Blood deals 1 damage to each opponent whenever another
// Klingon or Warrior its controller controls dies. Indelicate Healer gives the
// creatures its controller controls lifelink until end of turn, so a Curzon
// that dies beside other Klingons, in a Wrath of God or in a combat's
// state-based batch, triggers once per other one, and each trigger resolves
// with Curzon in the graveyard: its damage reads Curzon's last known
// characteristics, lifelink included (CR 603.10a, 113.7a, 702.15b). gorge
// moves a batch's members one by one, and a trigger queued after Curzon's own
// move takes the same departure snapshot as one queued before it.
func TestBatchLookBackLifelinkLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Curzon Dax, Bound by Blood", Count: 4}, {Name: "Indelicate Healer", Count: 4},
		{Name: "Defense Force Aggressor", Count: 4}, {Name: "Qo'noS Quartermaster", Count: 4},
		{Name: "Mek'leth Berserker", Count: 2}, {Name: "Wrath of God", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	}
	b := []cardset.Entry{
		{Name: "Wrath of God", Count: 4}, {Name: "Sickbay Orderly", Count: 4},
		{Name: "Operations Officer", Count: 4}, {Name: "Indelicate Healer", Count: 4},
		{Name: "Plains", Count: 24},
	}
	fixtureLockstep(t, "std-trc", [2][]cardset.Entry{a, b}, 300)
}
