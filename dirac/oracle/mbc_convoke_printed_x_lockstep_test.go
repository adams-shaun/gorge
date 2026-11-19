package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// A printed Convoke on an {X} cost (Chord of Calling, March of the
// Multitudes, Return to the Ranks, Worldsoul Colossus): gorge announces the
// convoke picks before X (its tentative pre-X generic picks) and prices X
// against them; the kernel's announcement and X ask (kw_convoke.go
// convokeXCost, convokeXHi) must pose the same picks and X values, decision
// by decision, in two-seat games and four-seat pods. The decks carry cheap
// green and white creatures to tap and to fetch.
var convokePrintedXDeck = []cardset.Entry{
	{Name: "Chord of Calling", Count: 4}, {Name: "March of the Multitudes", Count: 3},
	{Name: "Return to the Ranks", Count: 3}, {Name: "Worldsoul Colossus", Count: 2},
	{Name: "Llanowar Elves", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Elite Vanguard", Count: 3}, {Name: "Hill Giant", Count: 1},
	{Name: "Forest", Count: 7}, {Name: "Plains", Count: 5},
}

var convokePrintedXCards = []string{"Chord of Calling", "March of the Multitudes", "Return to the Ranks", "Worldsoul Colossus"}

func convokePrintedXTally(g *state.Game) int {
	n := cedhZoneCount(g, state.ZBattlefield, "Worldsoul Colossus")
	for _, c := range convokePrintedXCards[:3] {
		n += cedhZoneCount(g, state.ZGraveyard, c)
	}
	return n
}

func TestConvokePrintedXLockstep(t *testing.T) {
	t.Parallel()
	d := convokePrintedXDeck
	two := cedhPodRun(t, "convokex", [][]cardset.Entry{d, d}, convokePrintedXCards, 40, convokePrintedXTally)
	four := cedhPodRun(t, "convokex", [][]cardset.Entry{d, d, d, d}, convokePrintedXCards, 16, convokePrintedXTally)
	t.Logf("convoke {X} spells resolved: %d in two-seat games, %d in pods", two, four)
	if two < 20 || four < 20 {
		t.Fatalf("too little evidence: %d two-seat, %d four-seat (want 20 each)", two, four)
	}
}

// The blue and red-white carriers: Transcendent Message ({X}{U}{U}{U}{U})
// and Wildfire Awakener ({X}{1}{R}{W}, a creature), beside creatures of
// every colour the costs name.
var convokePrintedXDeckB = []cardset.Entry{
	{Name: "Transcendent Message", Count: 4}, {Name: "Wildfire Awakener", Count: 4},
	{Name: "Savannah Lions", Count: 4}, {Name: "Coral Merfolk", Count: 4}, {Name: "Gray Ogre", Count: 4},
	{Name: "Elite Vanguard", Count: 4}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5},
}

func TestConvokePrintedXBlueRedLockstep(t *testing.T) {
	t.Parallel()
	d := convokePrintedXDeckB
	names := []string{"Transcendent Message", "Wildfire Awakener"}
	tally := func(g *state.Game) int {
		return cedhZoneCount(g, state.ZGraveyard, names[0]) + cedhZoneCount(g, state.ZBattlefield, names[1])
	}
	two := cedhPodRun(t, "convokexb", [][]cardset.Entry{d, d}, names, 40, tally)
	four := cedhPodRun(t, "convokexb", [][]cardset.Entry{d, d, d, d}, names, 16, tally)
	t.Logf("resolved: %d in two-seat games, %d in pods", two, four)
	if two < 10 || four < 10 {
		t.Fatalf("too little evidence: %d two-seat, %d four-seat (want 10 each)", two, four)
	}
}
