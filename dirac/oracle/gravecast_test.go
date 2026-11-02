package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The graveyard cast trigger's lockstep: a SpellCast trigger with TriggerZones$
// Graveyard functions from its owner's graveyard (CR 113.6k) and comes after the
// battlefield's triggers in the queue.
var kgravecastDecks = [][2][]cardset.Entry{{
	{
		{Name: "Shambling Cie'th", Count: 8}, {Name: "Vengevine", Count: 4}, {Name: "Lingering Phantom", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
}, {
	// a Clue's sacrifice returns Curious Cadaver from the graveyard
	{
		{Name: "Curious Cadaver", Count: 10}, {Name: "Thraben Inspector", Count: 10},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
}}

func TestGravecastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gravecast")
	if err != nil {
		t.Fatal(err)
	}
	for _, md := range kgravecastDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 192; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kgravecast-a", "kgravecast-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, poisonhighPolicy(seed, map[string]int{}, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
			CloseSide(k)
			if d := res.Div; d != nil {
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			}
		}
	}
}
