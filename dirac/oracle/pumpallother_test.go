package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A Pump of one creature you control beside a PumpAll of each OTHER creature
// you control (Phalanx Tactics +2/+1 and +1/+1; Sigil Blessing +3/+3 and
// +1/+1): the target gets its own pump only, the rest the sweep's (CR 608.2c;
// Forge reads Targeted.Other as "other than the target"). gorge played the
// other creatures unpumped before filter.go bound the Targeted base, and the
// kernel lowers the pair as one PumpAll over every such creature plus the
// target's difference. The expected answer at every decision is gorge's.
var kpumpAllOtherDecks = [2][]cardset.Entry{
	{
		{Name: "Phalanx Tactics", Count: 4}, {Name: "Sigil Blessing", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Phalanx Tactics", Count: 3}, {Name: "Sigil Blessing", Count: 3},
		{Name: "Hill Giant", Count: 5}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 3},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 8},
	},
}

func TestPumpAllOtherLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "pumpallother", kpumpAllOtherDecks, 200, "KPUMPALLOTHER_SEEDS")
}
