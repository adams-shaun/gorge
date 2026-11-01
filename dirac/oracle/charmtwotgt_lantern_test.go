package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The hybrid lowering witness beside an any-colour mana grant: Chromatic
// Lantern grants "{T}: Add one mana of any color" to lands, so the offer
// gate's availability counts each Forest as an any-colour unit, while the
// planner's census taps a Forest's printed {G} only (the granted ability is
// a prefix gap). Burning-Tree Emissary's {R/G}{R/G} was witnessed {R}{R}
// (affordable by availability) and the planner, with the Lantern its only
// red source, found no plan (seed 37: kernel panic "autopay found no plan"
// at decision 222). gorge's planHybridCost resolves by planning each
// resolution and pays {G}{R}; the kernel copies it (tapForPK,
// hybResolveTappable). The decks are the charmtwotgt lists with two basics
// replaced by two Lanterns each.
var kcharmtwotgtLanternDecks = [2][]cardset.Entry{
	{
		{Name: "Epic Fight", Count: 3}, {Name: "Go Nuts!", Count: 3}, {Name: "HULK SMASH!", Count: 2},
		{Name: "Iridescent Tiger", Count: 2}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Craw Wurm", Count: 1}, {Name: "Gray Ogre", Count: 1}, {Name: "Darksteel Relic", Count: 2},
		{Name: "Chromatic Lantern", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Epic Fight", Count: 2}, {Name: "Go Nuts!", Count: 2}, {Name: "HULK SMASH!", Count: 3},
		{Name: "Iridescent Tiger", Count: 2}, {Name: "Burning-Tree Emissary", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 3}, {Name: "Gray Ogre", Count: 2},
		{Name: "Earth Elemental", Count: 1}, {Name: "Craw Wurm", Count: 1}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Darksteel Relic", Count: 2},
		{Name: "Chromatic Lantern", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Mountain", Count: 7},
	},
}

func TestCharmtwotgtLantern(t *testing.T) {
	t.Parallel()

	env := kcharmtwotgtSetup(t)
	for s, d := range kcharmtwotgtLanternDecks {
		env.decks[s] = env.decks[s][:0:0]
		for _, e := range d {
			card, ok := env.reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("deck %d: %s not in the corpus", s, e.Name)
			}
			for i := 0; i < e.Count; i++ {
				env.decks[s] = append(env.decks[s], card)
			}
		}
		if len(env.decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(env.decks[s]))
		}
	}
	tbl, _, _, err := env.set.Matchup(kcharmtwotgtLanternDecks[0], kcharmtwotgtLanternDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	// seed 37 (odd: GorgeAutopaySurface false) is the reported panic; the
	// rest alternate the surface (env.game: seed%2 == 0).
	diverged := 0
	for seed := uint64(1); seed <= 40; seed++ {
		res, err := env.game(seed, tbl, &kcharmtwotgtTally{})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if res.Div != nil {
			diverged++
			lo := max(res.Div.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:res.Div.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			t.Errorf("seed %d: stalled: %s after %d turns", seed, res.Stall, res.Turns)
		}
	}
	t.Logf("%d/40 diverged", diverged)
}
