package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated spellmanavalue pool -- the
// SpellCast carriers of TriggeredSpellAbility$CardManaCostLKI (the pre-cast
// card's mana value: split halves summed, {X} excluded, a copy's copied face
// plus X) beside split, X, copy and Adventure fixtures, cheap removal and
// vanilla creatures, so the expected answer at every decision is gorge's.

var (
	spellmanavalueDeckA = []cardset.Entry{
		{Name: "Prismari Apprentice", Count: 2}, {Name: "Unsummon", Count: 2},
		{Name: "Divination", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Mind Spring", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	}
	spellmanavalueDeckB = []cardset.Entry{
		{Name: "Manaplasm", Count: 3}, {Name: "Erratic Cyclops", Count: 2}, {Name: "Kaervek the Merciless", Count: 2},
		{Name: "Skittering Cicada", Count: 2}, {Name: "Hammerhead Tyrant", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Lightning Bolt", Count: 3}, {Name: "Hill Giant", Count: 2},
		{Name: "Fire", Count: 3}, {Name: "Twincast", Count: 2},
		{Name: "Island", Count: 5}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4}, {Name: "Swamp", Count: 4},
	}
)

// Two 40-card matchups, each way round, 32 seeds each (64 games): the
// carriers are cast from both seats and on both play/draw paths.
var spellmanavalueDecks = [][2][]cardset.Entry{
	{spellmanavalueDeckA, spellmanavalueDeckB},
	{spellmanavalueDeckB, spellmanavalueDeckA},
}

// spellmanavalueCarriers names the cards whose casts the test counts. Each must be
// played at least once; the minimums are set below the counts the seeded run
// reaches (derived from the run, not guessed). Twincast exercises the copy of
// a divided-damage spell and its new-target ask.
var spellmanavalueCarriers = []struct {
	name string
	min  int
}{
	{"Manaplasm", 55}, {"Erratic Cyclops", 35}, {"Kaervek the Merciless", 20},
	{"Skittering Cicada", 38}, {"Hammerhead Tyrant", 23}, {"Prismari Apprentice", 48},
	{"Twincast", 20},
}

func TestTriggeredSpellManaValueLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("spellmanavalue")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range spellmanavalueDecks {
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
		for seed := uint64(1); seed <= 32; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"b14b-a", "b14b-b"}, decks, 1)
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
	for _, c := range spellmanavalueCarriers {
		if played[c.name] < c.min {
			t.Errorf("carrier %s played %d times, want at least %d", c.name, played[c.name], c.min)
		}
	}
}
