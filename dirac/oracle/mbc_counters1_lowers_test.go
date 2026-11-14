package oracle

import (
	"slices"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMBCCounters1Lowers plays fixture decks of MBC cards in lockstep with
// gorge leading, over a table built from the decks' own cards (a dr-mbc split
// no longer holds a whole deck), one per spelling the mbc-counters-1 family lowers:
//
//   - the DESCENT counter kind (Descent into Avernus: two counters each
//     upkeep, then X Treasures and X damage to each player);
//   - Count$YourCountersExperience over the experience player counter
//     (Otharri, Suns' Glory's attack trigger; Ezuri, Claw of Progress's
//     enters trigger and its begin-combat X +1/+1 counters);
//   - DividedAsYouChoose$ over a counter kind other than +1/+1 (Contagion's
//     two -2/-1 counters among one or two target creatures).
//
// The COMPONENT counter kind lowers beside them (Component Pouch's {T} +
// remove-a-component-counter mana ability is the dead census gap the
// dead-mana-gap ticket models). Each carrier must be played by some game; the
// comparison at every decision is gorge's.
func TestMBCCounters1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	e := func(n string, c int) cardset.Entry { return cardset.Entry{Name: n, Count: c} }
	cases := []struct {
		name     string
		a, b     []cardset.Entry
		carriers []string
	}{
		{"descent",
			[]cardset.Entry{e("Descent into Avernus", 8), e("Young Pyromancer", 4), e("Dwarven Miner", 4), e("Brazen Wolves", 4), e("Voldaren Epicure", 4), e("Mountain", 16)},
			[]cardset.Entry{e("Soul Warden", 4), e("Kami of False Hope", 4), e("Nyx-Fleece Ram", 4), e("Devout Witness", 4), e("Plains", 24)},
			[]string{"Descent into Avernus"}},
		{"experience-otharri",
			[]cardset.Entry{e("Otharri, Suns' Glory", 8), e("Reckless Barbarian", 4), e("Accorder Paladin", 4), e("Bloodlust Inciter", 4), e("Mountain", 10), e("Plains", 10)},
			[]cardset.Entry{e("Augur of Bolas", 4), e("Horseshoe Crab", 4), e("Diplomatic Escort", 4), e("Amphin Pathmage", 4), e("Island", 24)},
			[]string{"Otharri, Suns' Glory"}},
		{"experience-ezuri",
			[]cardset.Entry{e("Ezuri, Claw of Progress", 8), e("Quirion Ranger", 4), e("Dreampod Druid", 4), e("Omenspeaker", 4), e("Sejiri Merfolk", 4), e("Forest", 8), e("Island", 8)},
			[]cardset.Entry{e("Gideon's Lawkeeper", 4), e("Goldmeadow Harrier", 4), e("Syndic of Tithes", 4), e("Kor Celebrant", 4), e("Plains", 24)},
			[]string{"Ezuri, Claw of Progress"}},
		{"divided-m2m1",
			[]cardset.Entry{e("Contagion", 12), e("Deathgreeter", 4), e("Butcher Ghoul", 4), e("Feral Shadow", 4), e("Swamp", 16)},
			[]cardset.Entry{e("Accorder Paladin", 4), e("Ajani's Pridemate", 4), e("Helpful Hunter", 4), e("Temple Acolyte", 4), e("Geist of the Moors", 4), e("Plains", 20)},
			[]string{"Contagion"}},
	}
	reg := synthReg(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			decks := [2][]cardset.Entry{c.a, c.b}
			var names [2][]string
			for s, d := range decks {
				for _, en := range d {
					names[s] = append(names[s], en.Name)
				}
			}
			set := printedDelveSet(t, names)
			var cs [2][]*cards.Card
			for s, d := range decks {
				n := 0
				for _, en := range d {
					card, ok := reg.Lookup(en.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", en.Name)
					}
					for i := 0; i < en.Count; i++ {
						cs[s] = append(cs[s], card)
					}
					n += en.Count
				}
				if n != 40 {
					t.Fatalf("deck %d has %d cards", s, n)
				}
			}
			var mu sync.Mutex
			played := map[string]int{}
			tbl, _, _, err := set.Matchup(decks[0], decks[1])
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			for seed := uint64(1); seed <= 60; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
				for _, name := range res.RecentSources(m, len(res.Actions)) {
					if slices.Contains(c.carriers, name) {
						mu.Lock()
						played[name]++
						mu.Unlock()
					}
				}
			}
			for _, name := range c.carriers {
				if played[name] == 0 {
					t.Errorf("%s was never played in 60 games: the pool does not reach it", name)
				}
			}
			t.Logf("%s: carriers played %v", c.name, played)
		})
	}
}
