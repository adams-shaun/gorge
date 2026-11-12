package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Effect Continuous lockstep: constructed matchups over the generated
// effcont pool -- the until-end-of-turn CantBlock / ExtraLandPlay emblems and
// the until-your-next-turn NoMaxHandSize / MustAttack emblems -- so the
// expected answer at every decision is gorge's. The frozen FDN and BLB pair
// pools exercise none of these arms.
var effcontDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Magmatic Chasm", Count: 4}, {Name: "Falter", Count: 4}, {Name: "Seismic Stomp", Count: 4},
			{Name: "Goblin Locksmith", Count: 4}, {Name: "Fire of Orthanc", Count: 4}, {Name: "Mountain", Count: 20},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
			{Name: "Giant Spider", Count: 4}, {Name: "Mournwillow", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Swamp", Count: 8},
		},
	},
	{
		{
			{Name: "Explore", Count: 4}, {Name: "Enter the Unknown", Count: 4}, {Name: "Scale the Heights", Count: 4},
			{Name: "Summer Bloom", Count: 4}, {Name: "Urban Evolution", Count: 4}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Forest", Count: 8}, {Name: "Island", Count: 8},
		},
		{
			{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4}, {Name: "Birds of Paradise", Count: 4},
			{Name: "Sakura-Tribe Elder", Count: 4}, {Name: "Rampant Growth", Count: 4}, {Name: "Wood Elves", Count: 4},
			{Name: "Farhaven Elf", Count: 4}, {Name: "Forest", Count: 12},
		},
	},
	{
		{
			{Name: "Kardur, Doomscourge", Count: 4}, {Name: "Sol Ring", Count: 4},
			{Name: "Urban Evolution", Count: 4}, {Name: "Explore", Count: 4},
			{Name: "Island", Count: 10}, {Name: "Forest", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
			{Name: "Doom Blade", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Pacifism", Count: 4},
			{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 4},
		},
	},
	{
		{
			{Name: "Kardur, Doomscourge", Count: 4}, {Name: "Sol Ring", Count: 4},
			{Name: "Urban Evolution", Count: 4}, {Name: "Explore", Count: 4},
			{Name: "Island", Count: 10}, {Name: "Forest", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 5},
		},
		{
			{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4}, {Name: "Birds of Paradise", Count: 4},
			{Name: "Sakura-Tribe Elder", Count: 4}, {Name: "Rampant Growth", Count: 4}, {Name: "Wood Elves", Count: 4},
			{Name: "Farhaven Elf", Count: 4}, {Name: "Forest", Count: 12},
		},
	},
	{
		{
			{Name: "Enter the Infinite", Count: 4}, {Name: "Sol Ring", Count: 4}, {Name: "Island", Count: 32},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
			{Name: "Giant Spider", Count: 4}, {Name: "Mournwillow", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Swamp", Count: 8},
		},
	},
}

// effcontShapes names the carriers whose casts the test counts, grouped by
// the kernel record they exercise. Every shape needs at least 20
// applications across the matchups.
var effcontShapes = map[string][]string{
	"cantblock": {"Magmatic Chasm", "Falter", "Seismic Stomp", "Goblin Locksmith", "Fire of Orthanc", "Mournwillow"},
	"landplay":  {"Explore", "Enter the Unknown", "Scale the Heights", "Summer Bloom", "Urban Evolution"},
	"nextturn":  {"Enter the Infinite", "Kardur, Doomscourge"},
}

// effcontIgnored returns a copy of t whose emblems have no abilities: the
// mutation makes the kernel behave as if the Effect statics did nothing. It
// only copies the emblem table, so the original table stays usable.
func effcontIgnored(t *kop.Table) *kop.Table {
	m := *t
	m.Emblems = append([]kop.Emblem(nil), t.Emblems...)
	for i := range m.Emblems {
		m.Emblems[i].Abilities = nil
	}
	return &m
}

func TestEffcontLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effcont")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	mutationDiverged := false
	for mi, md := range effcontDecks {
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
		ignored := effcontIgnored(tbl)
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"effcont-a", "effcont-b"}, decks, 1)
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
			if seed <= 8 {
				m2, err := NewMatchup(reg, [2]string{"effcont-a", "effcont-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k2, err := SetSide(set, ignored, cfg)(m2, seed)
				if err != nil {
					t.Fatal(err)
				}
				res2 := Lockstep(m2, seed, k2, castActivatePlayedPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k2)
				if res2.Div != nil {
					mutationDiverged = true
				}
			}
		}
	}
	t.Logf("%d games, carrier plays %v", games, played)
	for shape, names := range effcontShapes {
		n := 0
		for _, name := range names {
			n += played[name]
		}
		if n < 20 {
			t.Errorf("shape %s: %d applications, want at least 20 (%v)", shape, n, played)
		}
	}
	if !mutationDiverged {
		t.Error("mutation 'effect ignored' never diverged")
	}
}
