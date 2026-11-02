package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The eachfromsource lockstep: constructed matchups over the generated eachfromsource
// pool, one carrier class per kgen lift unblocked at gorge pin
// 81dd3f940 -- SacrificeAll Defined$ Self Controller$ You (the
// end-of-combat sacrifices), PutCounter CounterType$ EachFromSource (the
// dies triggers), the modified property (Kami's Flare, Ambitious Assault,
// Temperamental Oozewagg, Upriser Renegade beside an Equipment and an
// Aura; Orochi Merge-Keeper's modified-gated granted mana ability is left out), RemoveCounter
// All/All (Vampire Hexmage, Blitz Leech) and api RemoveCounterAll
// (Inhumaniac, Pestilent Haze) -- so the expected answer at every decision
// is gorge's. The frozen FDN and BLB pair pools exercise none of these.
var eachFromSourceDecks = [2][]cardset.Entry{
	{
		{Name: "Star Pupil", Count: 3}, {Name: "Dockworker Drone", Count: 2}, {Name: "Graft Surgeon", Count: 2},
		{Name: "Essence Channeler", Count: 2}, {Name: "Vampire Hexmage", Count: 3}, {Name: "Blitz Leech", Count: 2},
		{Name: "Inhumaniac", Count: 2}, {Name: "Pestilent Haze", Count: 1}, {Name: "Doom Blade", Count: 2},
		{Name: "Savannah Lions", Count: 2}, {Name: "Short Bow", Count: 2}, {Name: "Walking Corpse", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Fog Elemental", Count: 2}, {Name: "Mardu Blazebringer", Count: 2}, {Name: "Crumbling Colossus", Count: 1},
		{Name: "Runaway Carriage", Count: 2}, {Name: "Temperamental Oozewagg", Count: 2}, {Name: "Upriser Renegade", Count: 2},
		{Name: "Kami's Flare", Count: 2}, {Name: "Ambitious Assault", Count: 2}, {Name: "Blanchwood Armor", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 7}, {Name: "Island", Count: 4},
	},
}

// eachFromSourceCarriers are the pool's carriers and the canonical action that
// counts as one occurrence: a cast (-1), an activation (the ability
// index), or an attack (-2: the end-of-combat sacrifice triggers), with
// the least count asserted per surface.
var eachFromSourceCarriers = map[string]struct{ act, min int }{
	"Fog Elemental": {-2, 10}, "Mardu Blazebringer": {-2, 10}, "Crumbling Colossus": {-2, 3}, "Runaway Carriage": {-2, 5},
	"Star Pupil": {-1, 50}, "Dockworker Drone": {-1, 30}, "Graft Surgeon": {-1, 20}, "Essence Channeler": {-1, 30},
	"Kami's Flare": {-1, 20}, "Ambitious Assault": {-1, 20}, "Temperamental Oozewagg": {-1, 20}, "Upriser Renegade": {-1, 30},
	"Vampire Hexmage": {0, 20}, "Blitz Leech": {-1, 10}, "Inhumaniac": {-1, 30}, "Pestilent Haze": {-1, 5},
}

func TestEachFromSourceAndModifiedLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("eachfromsource")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range eachFromSourceDecks {
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
	tbl, _, _, err := set.Matchup(eachFromSourceDecks[0], eachFromSourceDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(512)
	if s := os.Getenv("KLIFT8_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	var inst []string
	for _, d := range decks {
		for _, c := range d {
			inst = append(inst, cardName(c))
		}
	}
	var surf [2]map[string]int
	surf[0], surf[1] = map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"deck-a", "deck-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		si := 0
		if seed%4 == 0 {
			cfg, si = Config{GorgeAutopaySurface: true}, 1
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		for _, a := range res.Actions {
			x := int(a.Src())
			if x >= len(inst) {
				continue
			}
			c, ok := eachFromSourceCarriers[inst[x]]
			if !ok {
				continue
			}
			switch k := a.Kind(); {
			case c.act == -1 && k == ActCast, c.act == -2 && k == ActAttack, c.act >= 0 && k == ActActivate && int(a.Mode()) == c.act:
				surf[si][inst[x]]++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds; carrier occurrences: lowering surface %v, autopay surface %v", seeds, surf[0], surf[1])
	if seeds < 512 {
		return // a short run reports the counts without asserting them
	}
	for n, c := range eachFromSourceCarriers {
		for si, s := range surf {
			want := c.min
			if si == 1 {
				want = c.min / 4 // 128 of the 512 seeds run the autopay surface
			}
			if s[n] < want {
				t.Errorf("%s: %d occurrences on surface %d (want >= %d)", n, s[n], si, want)
			}
		}
	}
}
