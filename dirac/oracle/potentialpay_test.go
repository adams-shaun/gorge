package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The potential pool lockstep: a trigger's own mana cost (Thousand Moons
// Crackshot), its count (Hawkeye) and its {X} (Wildborn Preserver) are offered
// against gorge's potential pool, an over-bound (CR 118.3 then refuses what the
// sources cannot fund). A dual counts in each colour, a land's further abilities
// count on their own, a paid ability joins once other mana covers its cost, and
// an open production is unlimited unless its restriction refuses the payment.
var kpotentialpayDecks = [][2][]cardset.Entry{{
	{
		{Name: "Thousand Moons Crackshot", Count: 8}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Captivating Cave", Count: 4}, {Name: "Restless Bivouac", Count: 4}, {Name: "Training Compound", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	{
		{Name: "Hawkeye, Master Marksman", Count: 8}, {Name: "Wildborn Preserver", Count: 8}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Secluded Courtyard", Count: 4}, {Name: "Captivating Cave", Count: 4}, {Name: "Training Compound", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Elite Vanguard", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Plains", Count: 14}, {Name: "Island", Count: 14},
	},
}}

func TestPotentialpayLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("potentialpay")
	if err != nil {
		t.Fatal(err)
	}
	divs := 0
	for mi, md := range kpotentialpayDecks {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 192; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kpotentialpay-a", "kpotentialpay-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := poisonhighPolicy(seed, map[string]int{}, map[string]int{}) // casts and activates whenever it can
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
			CloseSide(k)
			if d := res.Div; d != nil {
				divs++
				if divs <= 3 {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail)
				}
			}
		}
	}
	if divs != 0 {
		t.Fatalf("%d games diverged", divs)
	}
}
