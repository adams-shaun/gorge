package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Hidden Grotto is each deck's only source of the second colour of its hybrid
// pips, so a face of that colour is payable only through the filter (CR 605.3a,
// 601.2g). The oracle's window answers the Grotto's free {C} first, so taking
// such a face strands the payment and reverses it (CR 733.1), keeping the taps.
// Past these seeds gorge also offers a face the filter's fee leaves unpayable:
// its face gate does not charge the fee.
var filterFaceDecks = [2][]cardset.Entry{
	{
		{Name: "Hidden Grotto", Count: 6}, {Name: "Mountain", Count: 18},
		{Name: "Cindering Cutthroat", Count: 16},
	},
	{
		{Name: "Hidden Grotto", Count: 6}, {Name: "Plains", Count: 12}, {Name: "Forest", Count: 6},
		{Name: "Moonrise Cleric", Count: 16},
	},
}

func TestHybridFaceThroughFilterLockstep(t *testing.T) {
	t.Parallel()
	reg, _ := kcastSetup(t)
	set, err := cardset.ByName("blb")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, filterFaceDecks)
	tbl, _, _, err := set.Matchup(filterFaceDecks[0], filterFaceDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	for _, seed := range []uint64{2, 5, 6, 10, 11, 18, 23, 24, 25, 28, 30, 33, 36, 37, 40, 47} {
		m, err := NewMatchup(reg, [2]string{"grotto-br", "grotto-wb"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("seed %d: diverged at %d: %s %s %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
		}
	}
}
