package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Count$Adamant at a spell's own resolution (Cauldron's Gift's mill gate,
// Foreboding Fruit's Food count) and at the card's own ETB replacement (the
// Paladins' K:etbCounter Adamant$): kgen lower_evoke.go adamantValue, kernel
// kw_evoke.go exAdamant reading the resolving spell entry's spend. The decks
// split their lands so three mana of one colour is spent on some casts and
// not on others, and Cauldron's Gift returns a Paladin that was not cast.
var adamantSitesDecks = [2][]cardset.Entry{
	{
		{Name: "Ardenvale Paladin", Count: 8}, {Name: "Foreboding Fruit", Count: 6}, {Name: "Cauldron's Gift", Count: 4},
		{Name: "Plains", Count: 11}, {Name: "Swamp", Count: 11},
	},
	{
		{Name: "Garenbrig Paladin", Count: 8}, {Name: "Foreboding Fruit", Count: 4}, {Name: "Cauldron's Gift", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 8},
	},
}

func TestAdamantSitesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.EvokeStats
	dirac.DebugEvokeStats = &st
	defer func() { dirac.DebugEvokeStats = nil }()
	lockstepRun(t, "adamantsites", adamantSitesDecks, 80, true, nil)
	if st.GateTrue == 0 || st.GateFalse == 0 {
		t.Fatalf("Adamant reads held %d times and failed %d times; the lockstep must see both", st.GateTrue, st.GateFalse)
	}
	t.Logf("Adamant reads: %+v", st)
}
