package oracle

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated standardclasses pool, one
// matchup per class sweep 1 (Standard-weighted) lifted, so the
// expected answer at every decision is gorge's. The frozen FDN and BLB
// pair pools exercise none of these arms.

// standardclassesCarrier is a carrier card and the canonical action that counts as one
// occurrence: a cast (-1), an activation (the ability index), or an attack
// (-2), with the least count asserted on the lowering surface (min) and on
// the autopay surface (ap: 0 for a spell gorge's planner never prices,
// {X} or hybrid, which that surface drops).
type standardclassesCarrier struct{ act, min, ap int }

type standardclassesMatchup struct {
	class    string
	decks    [2][]cardset.Entry
	carriers map[string]standardclassesCarrier
}

var standardclassesMatchups = []standardclassesMatchup{{
	class: "HasCounters",
	decks: [2][]cardset.Entry{{
		{Name: "Bulwark Ox", Count: 3}, {Name: "Federation Field Medic", Count: 2}, {Name: "Delta Bloodflies", Count: 3},
		{Name: "Yathan Tombguard", Count: 3}, {Name: "Scolding Administrator", Count: 3}, {Name: "Iron Apprentice", Count: 3},
		{Name: "Doom Blade", Count: 2}, {Name: "Savannah Lions", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 9},
	}, {
		{Name: "Kulrath Knight", Count: 3}, {Name: "Tyrant Guard", Count: 3}, {Name: "Dragonscale Boon", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Iron Apprentice", Count: 2}, {Name: "Hill Giant", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	}},
	carriers: map[string]standardclassesCarrier{
		"Bulwark Ox": {0, 100, 30}, "Federation Field Medic": {-1, 60, 20}, "Delta Bloodflies": {-2, 100, 50}, "Yathan Tombguard": {-2, 100, 30},
		"Scolding Administrator": {-1, 80, 30}, "Iron Apprentice": {-1, 150, 40}, "Kulrath Knight": {-1, 60, 0}, "Tyrant Guard": {-1, 80, 0},
	},
}, {
	class: "DB$ Mana Amount$",
	decks: [2][]cardset.Entry{{
		{Name: "Akki Rockspeaker", Count: 4}, {Name: "Priest of Urabrask", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	}, {
		{Name: "Priest of Gix", Count: 4}, {Name: "Gray Ogre", Count: 6}, {Name: "Doom Blade", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Savannah Lions", Count: 3}, {Name: "Swamp", Count: 11}, {Name: "Plains", Count: 8},
	}},
	carriers: map[string]standardclassesCarrier{
		"Akki Rockspeaker": {-1, 60, 15}, "Priest of Urabrask": {-1, 60, 15}, "Priest of Gix": {-1, 60, 15},
	},
}, {
	class: "trigger PresentDefined$",
	decks: [2][]cardset.Entry{{
		{Name: "Cautious Survivor", Count: 4}, {Name: "Savior of the Small", Count: 3}, {Name: "Shrewd Storyteller", Count: 4},
		{Name: "Glimmer Seeker", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 10},
	}, {
		{Name: "Howling Mine", Count: 3}, {Name: "Cynical Loner", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Doom Blade", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 9},
	}},
	carriers: map[string]standardclassesCarrier{
		"Cautious Survivor": {-2, 40, 10}, "Savior of the Small": {-2, 20, 5}, "Shrewd Storyteller": {-2, 40, 10},
		"Glimmer Seeker": {-2, 20, 5}, "Howling Mine": {-1, 40, 10}, "Cynical Loner": {-2, 30, 5},
	},
}}

func TestStandardSweepClassesLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("standardclasses")
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(128) // 32 of them on the autopay surface
	if s := os.Getenv("C8_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	for _, mu := range standardclassesMatchups {
		var decks [2][]*cards.Card
		for s, d := range mu.decks {
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
				t.Fatalf("%s deck %d: %d cards", mu.class, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(mu.decks[0], mu.decks[1])
		if err != nil {
			t.Fatal(err)
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
			if os.Getenv("C8_ONLY") != "" && os.Getenv("C8_ONLY") != fmt.Sprint(seed) {
				continue
			}
			m, err := NewMatchup(reg, [2]string{"c8-a", "c8-b"}, decks, 1)
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
				c, ok := mu.carriers[inst[x]]
				if !ok {
					continue
				}
				switch k := a.Kind(); {
				case c.act == -1 && k == ActCast, c.act == -2 && k == ActAttack, c.act >= 0 && k == ActActivate && int(a.Mode()) == c.act:
					surf[si][inst[x]]++
				}
			}
			if d := res.Div; d != nil {
				lo := d.At - 40
				if lo < 0 {
					lo = 0
				}
				t.Logf("%s seed %d last actions %v", mu.class, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mu.class, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		t.Logf("%s: %d seeds; carrier occurrences: lowering surface %v, autopay surface %v", mu.class, seeds, surf[0], surf[1])
		if seeds < 128 {
			continue // a short run reports the counts without asserting them
		}
		for n, c := range mu.carriers {
			for si, s := range surf {
				want := c.min
				if si == 1 {
					want = c.ap
				}
				if s[n] < want {
					t.Errorf("%s: %s: %d occurrences on surface %d (want >= %d)", mu.class, n, s[n], si, want)
				}
			}
		}
	}
}
