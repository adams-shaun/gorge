package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The ChangeZoneAll lockstep: Finale of Revelation
// (UseAllOriginZones$: both libraries shuffle, the receiving one first) and
// Pinnacle Starcage (Duration$ UntilHostLeavesPlay: the swept cheap
// artifacts and creatures return when it leaves; Shatter kills it). Random Encounter
// (ForgetOtherRemembered$: the milled creatures enter, a delayed trigger
// returns up to four of them to hand at the next end step, DpMany).
var kczall3Decks = [2][]cardset.Entry{
	{
		{Name: "Pinnacle Starcage", Count: 4}, {Name: "Finale of Revelation", Count: 3}, {Name: "Divination", Count: 3},
		{Name: "Ornithopter", Count: 4}, {Name: "Memnite", Count: 4},
		{Name: "Island", Count: 11}, {Name: "Plains", Count: 11},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Random Encounter", Count: 4},
		{Name: "Elvish Mystic", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Shock", Count: 2}, {Name: "Shatter", Count: 4},
		{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7},
	},
}

func TestChangeZoneAllParamsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "czallparams", kczall3Decks, 1000, "KCZALL3_SEEDS")
}

// TestChangeZoneAllParamsLockstepMutation proves the two kernel halves matter: with the
// ForgetOther clear and the until-leaves links skipped, some seed diverges
// (Pinnacle's exiled cards never return); likewise with the delayed
// many-object return moving only its first creature (Random Encounter).
func TestChangeZoneAllParamsLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCzall3Off = true
	defer func() { dirac.DebugCzall3Off = false }()
	if !lockstepDiverges(t, "czallparams", kczall3Decks, 1000) {
		t.Fatal("no seed diverged with the czall3 links off: no Pinnacle Starcage exile returned")
	}
}

// TestChangeZoneAllParamsManyMutation: with the delayed many-object return moving
// only its first creature, some seed diverges (two or more milled creatures
// stay on the battlefield past the end step).
func TestChangeZoneAllParamsManyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDpManyOff = true
	defer func() { dirac.DebugDpManyOff = false }()
	if !lockstepDiverges(t, "czallparams", kczall3Decks, 1000) {
		t.Fatal("no seed diverged with the many-object return cut to one: no Random Encounter milled two creatures")
	}
}
