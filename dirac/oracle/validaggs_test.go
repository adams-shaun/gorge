package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated validaggs pool --
// printed skulk, the Count$Valid battlefield aggregates (Shimmercreep's
// colours, Rubblebelt Rioters' greatest power, Towering Gibbon's greatest
// mana value), the remembered-set MayPlay (multi-card and nonland impulse
// draws), Condition$ Kicked, ConditionPlayerTurn$ and activation
// PlayerTurn$ -- so the expected answer at every decision is gorge's. The
// frozen FDN and BLB pair pools exercise none of these arms.

var validaggsDecks = [][2][]cardset.Entry{{
	{
		{Name: "Furtive Homunculus", Count: 3}, {Name: "Act on Impulse", Count: 2}, {Name: "Seize Opportunity", Count: 2},
		{Name: "Wrenn's Resolve", Count: 2}, {Name: "Abbot of Keral Keep", Count: 3}, {Name: "Blink of an Eye", Count: 2},
		{Name: "Knacksaw Clique", Count: 2}, {Name: "Into the Roil", Count: 2}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Gray Ogre", Count: 2}, {Name: "Aqueous Form", Count: 1}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Pale Rider of Trostad", Count: 2}, {Name: "Vampire Cutthroat", Count: 3}, {Name: "Shimmercreep", Count: 2},
		{Name: "Towering Gibbon", Count: 2}, {Name: "Gnarlid Colony", Count: 3}, {Name: "Aggressive Sabotage", Count: 2},
		{Name: "Disrupting Scepter", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Walking Corpse", Count: 2},
		{Name: "Giant Spider", Count: 2}, {Name: "Swamp", Count: 9}, {Name: "Forest", Count: 8},
	},
}, {
	{
		{Name: "Rubblebelt Rioters", Count: 3}, {Name: "Tuya Bearclaw", Count: 2}, {Name: "Fated Conflagration", Count: 2},
		{Name: "Pyreswipe Hawk", Count: 2}, {Name: "Territorial Bruntar", Count: 2}, {Name: "Gray Ogre", Count: 1},
		{Name: "Gila Courser", Count: 2}, {Name: "Hill Giant", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Seize Opportunity", Count: 2}, {Name: "Gnarlid Colony", Count: 1}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Furtive Homunculus", Count: 3}, {Name: "Pale Rider of Trostad", Count: 3}, {Name: "Vampire Cutthroat", Count: 3},
		{Name: "Shimmercreep", Count: 2}, {Name: "Blink of an Eye", Count: 2}, {Name: "Knacksaw Clique", Count: 2},
		{Name: "Disrupting Scepter", Count: 2}, {Name: "Aqueous Form", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Coral Merfolk", Count: 2}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 8},
	},
}}

func TestValidAggregatesAndImpulseLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("validaggs")
	if err != nil {
		t.Fatal(err)
	}
	games, casts, mayplays, kicked := 0, 0, 0, 0
	for mi, md := range validaggsDecks {
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
			m, err := NewMatchup(reg, [2]string{"c7a-a", "c7a-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castActivatePolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			for _, a := range res.Actions {
				if a.Kind() == ActCast {
					casts++
					switch CastModeName(a.Mode() & 15) {
					case "mayplay":
						mayplays++
					case "kicked":
						kicked++
					}
				}
			}
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
	t.Logf("%d games, %d casts, %d through a play permission, %d kicked", games, casts, mayplays, kicked)
	if mayplays == 0 || kicked == 0 {
		t.Fatal("no play-permission or kicked cast was played")
	}
}
