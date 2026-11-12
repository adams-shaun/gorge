package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The aggregate-tap lockstep: constructed matchups over the
// generated crew pool -- Crew Vehicles, Saddle Mounts, Exhaust abilities --
// so the expected answer at every decision is gorge's. The frozen FDN and
// BLB pair pools have none of these cards.

var crewDecks = [][2][]cardset.Entry{{
	{
		{Name: "Sleek Schooner", Count: 2}, {Name: "Renegade Freighter", Count: 2}, {Name: "Untethered Express", Count: 2},
		{Name: "Smuggler's Copter", Count: 2}, {Name: "Sky Skiff", Count: 2}, {Name: "Mobile Garrison", Count: 1},
		{Name: "Thundering Chariot", Count: 1}, {Name: "Rocketeer Boostbuggy", Count: 2}, {Name: "Savannah Lions", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Wall of Wood", Count: 2}, {Name: "Prowcatcher Specialist", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Bridled Bighorn", Count: 2}, {Name: "Trained Arynx", Count: 2}, {Name: "Gloryheath Lynx", Count: 2},
		{Name: "Seraphic Steed", Count: 2}, {Name: "Brightfield Mustang", Count: 2}, {Name: "Drover Grizzly", Count: 2},
		{Name: "Venomsac Lagac", Count: 2}, {Name: "Unswerving Sloth", Count: 1}, {Name: "Stampeding Scurryfoot", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Coral Merfolk", Count: 1}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Fleetwheel Cruiser", Count: 2}, {Name: "Ovalchase Dragster", Count: 2}, {Name: "Spire Mechcycle", Count: 2},
		{Name: "Quilled Charger", Count: 2}, {Name: "Caustic Bronco", Count: 3}, {Name: "Gray Ogre", Count: 2},
		{Name: "Walking Corpse", Count: 3}, {Name: "Keen Buccaneer", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 6},
		{Name: "Island", Count: 4}, {Name: "Sky Skiff", Count: 2}, {Name: "Rocketeer Boostbuggy", Count: 2},
	},
	{
		{Name: "Hazard of the Dunes", Count: 2}, {Name: "Skystreak Engineer", Count: 2}, {Name: "Giant Spider", Count: 2},
		{Name: "Sleek Schooner", Count: 2}, {Name: "Mobile Garrison", Count: 2}, {Name: "Venomsac Lagac", Count: 2},
		{Name: "Drover Grizzly", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Coral Merfolk", Count: 3},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 8},
	},
}}

func TestCrewLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("crew")
	if err != nil {
		t.Fatal(err)
	}
	tapcost, _ := chooseCode("tapcost")
	picks, dones, games := 0, 0, 0
	for mi, md := range crewDecks {
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
			m, err := NewMatchup(reg, [2]string{"crew-a", "crew-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := activateOnlyPolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			prevTap := false
			for _, a := range res.Actions {
				if a.Kind() == ActChoose+CanonKind(tapcost) {
					picks++
					prevTap = true
					continue
				}
				if a.Kind() == ActDone && prevTap {
					dones++
				}
				prevTap = false
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
	t.Logf("%d games, %d tap-cost picks, %d closed by Done", games, picks, dones)
	if picks == 0 || dones == 0 {
		t.Fatal("no tap-cost pick or Done was played")
	}
}
