package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Station / named-counter lockstep (kcounters): constructed matchups
// over the generated station pool -- Spacecraft and Planets (CR 702.184a,
// gorge's "station" special action and its KChoose "station" tap pick),
// and the unfeaturized counter kinds (CHARGE, OIL, SPORE, QUEST, VERSE,
// GROWTH, FUSE, STORY) compared per kind against gorge's counters. The
// frozen FDN and BLB pair pools have none of these cards. The Eternity
// Elevator is in the pool but in no deck: its {T}: Add {C}{C}{C} leaves a
// floating {C} after which gorge's autopay planner prices no cast while the
// kernel offers them (a mana-planner gap of three-mana sources, not
// Station; HANDOFF-kcounters open items).

var stationDecks = [][2][]cardset.Entry{{
	{
		{Name: "Warmaker Gunship", Count: 2}, {Name: "Larval Scoutlander", Count: 2}, {Name: "Debris Field Crusher", Count: 2},
		{Name: "Atmospheric Greenhouse", Count: 1}, {Name: "Galvanizing Sawship", Count: 1}, {Name: "Wurmwall Sweeper", Count: 2},
		{Name: "Thallid", Count: 2}, {Name: "Thallid Germinator", Count: 2}, {Name: "War Dance", Count: 1}, {Name: "Momentum", Count: 1},
		{Name: "Kuldotha Cackler", Count: 2}, {Name: "Sawblade Scamp", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Kavaron, Memorial World", Count: 1}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Wedgelight Rammer", Count: 2}, {Name: "Rescue Skiff", Count: 1}, {Name: "Fell Gravship", Count: 2},
		{Name: "Susurian Dirgecraft", Count: 2}, {Name: "Mycologist", Count: 2}, {Name: "Staff of the Storyteller", Count: 2},
		{Name: "Quest for the Gravelord", Count: 2}, {Name: "Golden Urn", Count: 2}, {Name: "Bladed Ambassador", Count: 2},
		{Name: "Necrosquito", Count: 1}, {Name: "Savannah Lions", Count: 3}, {Name: "Walking Corpse", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
}, {
	{
		{Name: "Uthros Scanship", Count: 2}, {Name: "Pinnacle Kill-Ship", Count: 1},
		{Name: "Extinguisher Battleship", Count: 2}, {Name: "Wurmwall Sweeper", Count: 2}, {Name: "Spawning Pit", Count: 2},
		{Name: "Grindclock", Count: 1}, {Name: "Shriekhorn", Count: 1}, {Name: "Trigon of Rage", Count: 1}, {Name: "Lilting Refrain", Count: 1},
		{Name: "Gitaxian Raptor", Count: 2}, {Name: "Exuberant Fuseling", Count: 2}, {Name: "Urabrask's Anointer", Count: 1},
		{Name: "Incendiary", Count: 1}, {Name: "Coral Merfolk", Count: 3}, {Name: "Gray Ogre", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Atmospheric Greenhouse", Count: 2}, {Name: "Larval Scoutlander", Count: 2}, {Name: "Rescue Skiff", Count: 1},
		{Name: "Wedgelight Rammer", Count: 2}, {Name: "Pallid Mycoderm", Count: 2}, {Name: "Thallid", Count: 2}, {Name: "Mycologist", Count: 2},
		{Name: "Midsummer Revel", Count: 1}, {Name: "Staff of the Storyteller", Count: 1}, {Name: "Giant Spider", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Savannah Lions", Count: 2}, {Name: "Wall of Wood", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
	},
}}

// stationPolicy takes a special action (station) or an activation whenever
// one is offered, so the games reach the station picks and the counter
// costs, else plays randomly.
func stationPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActSpecial || a.Kind() == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestStationLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("station")
	if err != nil {
		t.Fatal(err)
	}
	stationPick, _ := chooseCode("station")
	special, _ := specialCode("station")
	seeds := uint64(24) // ~25 s for both matchups (the per-test budget is a minute)
	if testing.Short() {
		seeds = 6
	}
	stations, picks, games, stalls := 0, 0, 0, 0
	for mi, md := range stationDecks {
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
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"station-a", "station-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := stationPolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if res.Stall != "" {
				stalls++
				t.Logf("matchup %d seed %d: %s at turn %d", mi, seed, res.Stall, res.Turns)
			}
			for _, a := range res.Actions {
				switch {
				case a.Kind() == ActSpecial && a.Mode() == special:
					stations++
				case a.Kind() == ActChoose+CanonKind(stationPick):
					picks++
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
	t.Logf("%d games (%d stalled), %d stations, %d station picks", games, stalls, stations, picks)
	if stations == 0 || picks != stations {
		t.Fatalf("stations %d, picks %d: every station must be answered by one pick", stations, picks)
	}
}
