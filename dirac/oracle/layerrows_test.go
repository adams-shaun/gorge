package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated layerrows pool -- the
// BLB layer-row carriers. Azure Beastbinder is the until-your-next-turn
// override, Tightening Coils the layer-6 RemoveKeyword beside a Continuous
// pump, Control Magic the control-changing Aura, Ensoul Artifact the
// add-only AddType (a layer-4 card type), and Kasmina's Transmutation the
// base-P/T override with LoseAbilities. The frozen FDN and BLB pair pools
// exercise none of these arms. Starforged Sword (the BLB RemoveKeyword
// carrier) is left out: its Gift additional cost diverges from the kernel
// independently of the layer row.
var layerrowsDecks = [][2][]cardset.Entry{{
	{
		{Name: "Azure Beastbinder", Count: 3}, {Name: "Tightening Coils", Count: 3},
		{Name: "Control Magic", Count: 3}, {Name: "Ensoul Artifact", Count: 3},
		{Name: "Kasmina's Transmutation", Count: 3}, {Name: "Darksteel Relic", Count: 3},
		{Name: "Wind Drake", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Island", Count: 11}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Shock", Count: 3}, {Name: "Doom Blade", Count: 3},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Goblin Piker", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}}

// layerrowsCarriers names the cards whose casts and activations the test counts
// (each must be played at least once).
var layerrowsCarriers = []string{"Azure Beastbinder", "Tightening Coils", "Control Magic", "Ensoul Artifact", "Kasmina's Transmutation"}

func TestLayerRowCarriersLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("layerrows")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range layerrowsDecks {
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
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"b11-a", "b11-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castActivatePlayedPolicy(seed, played)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier plays %v", games, played)
	for _, n := range layerrowsCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
}
