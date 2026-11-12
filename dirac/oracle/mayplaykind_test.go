package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated mayplaykind pool --
// play permission records with a kind rider: gorge's MayPlayFree
// (Stolen Goods: cast without paying the mana cost)
// and MayPlayIgnoreType (Lidless Gaze, Rakdos, the Muscle: mana of any
// type). Matchup 0 carries only the free grants and matchup 1 only the
// any-type ones, so each matchup's play-permission casts count one kind.
// The frozen FDN and BLB pair pools carry neither.

var mayplaykindDecks = [][2][]cardset.Entry{{
	{
		{Name: "Stolen Goods", Count: 6}, {Name: "Coral Merfolk", Count: 4},
		{Name: "Furtive Homunculus", Count: 4}, {Name: "Gray Ogre", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Walking Corpse", Count: 4}, {Name: "Vampire Cutthroat", Count: 4}, {Name: "Gray Ogre", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Coral Merfolk", Count: 2}, {Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 9},
		{Name: "Island", Count: 2},
	},
}, {
	{
		{Name: "Lidless Gaze", Count: 4}, {Name: "Rakdos, the Muscle", Count: 2}, {Name: "Walking Corpse", Count: 3},
		{Name: "Vampire Cutthroat", Count: 3}, {Name: "Gray Ogre", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Swamp", Count: 11}, {Name: "Mountain", Count: 11},
	},
	{
		{Name: "Coral Merfolk", Count: 4}, {Name: "Furtive Homunculus", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
	},
}, {
	// the any-colour rider (MayPlayIgnoreColor): Hurl Through Hell exiles a
	// creature and grants a cast of it, Stolen Strategy exiles each
	// opponent's top card at upkeep; the deck's mana is off-colour for the
	// exiled cards, so only the rider can pay them
	{
		{Name: "Hurl Through Hell", Count: 6}, {Name: "Stolen Strategy", Count: 4},
		{Name: "Coral Merfolk", Count: 6}, {Name: "Furtive Homunculus", Count: 4},
		{Name: "Gray Ogre", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
		{Name: "Island", Count: 2},
	},
	{
		{Name: "Walking Corpse", Count: 4}, {Name: "Vampire Cutthroat", Count: 4}, {Name: "Coral Merfolk", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Island", Count: 8},
		{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
}, {
	// the MayPlayLimit cap (Flameskull): it dies, exiles itself and the top
	// card of its library, and grants one may-play this turn; removal and
	// blockers make it die repeatedly
	{
		{Name: "Flameskull", Count: 8}, {Name: "Lightning Bolt", Count: 4}, {Name: "Murder", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Wind Drake", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Murder", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
}}

func TestMayPlayKindRiderLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mayplaykind")
	if err != nil {
		t.Fatal(err)
	}
	games, casts := 0, 0
	var mayplays [4]int
	for mi, md := range mayplaykindDecks {
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
			m, err := NewMatchup(reg, [2]string{"b10-a", "b10-b"}, decks, 1)
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
					if CastModeName(a.Mode()&15) == "mayplay" {
						mayplays[mi]++
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
	t.Logf("%d games, %d casts, %d through a free permission, %d through an any-type permission, %d through an any-colour one, %d through a limited one", games, casts, mayplays[0], mayplays[1], mayplays[2], mayplays[3])
	if mayplays[0] < 20 || mayplays[1] < 20 || mayplays[2] < 20 || mayplays[3] < 20 {
		t.Fatalf("fewer than 20 plays through a permission kind: free %d, any-type %d, any-colour %d, limit %d", mayplays[0], mayplays[1], mayplays[2], mayplays[3])
	}
}
