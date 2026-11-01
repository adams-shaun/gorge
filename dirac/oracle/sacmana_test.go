package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The sacrifice/tap-cost vs mana lockstep (sacmana): constructed matchups
// over the generated sacmana pool, whose activations and spells sacrifice
// or tap a permanent that can also be a mana source (Treasures, Basal
// Thrull, mana creatures, Mind Stone). gorge prices the mana with those
// candidates held back (potential_plan.go planComposedCost); the expected
// answer at every decision is gorge's.
var ksacmanaDecks = [2][]cardset.Entry{
	{
		{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4}, {Name: "Prosperous Innkeeper", Count: 4},
		{Name: "Makeshift Munitions", Count: 4}, {Name: "Fountainport", Count: 3}, {Name: "Hill Giant", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Forest", Count: 9}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Treasure Dredger", Count: 4}, {Name: "Deadly Dispute", Count: 4}, {Name: "Basal Thrull", Count: 3},
		{Name: "Mind Stone", Count: 3}, {Name: "Fountainport", Count: 3}, {Name: "Savannah Lions", Count: 4},
		{Name: "Ornithopter", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 9},
	},
}

// sacCarriers are the pool's carriers: the activation (ability index) or
// the cast (-1) whose sacrifice or tap-another part can take a mana source.
var sacCarriers = map[string]int{"Fountainport": 1, "Makeshift Munitions": 0, "Deadly Dispute": -1}

func TestSacmanaLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("sacmana")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ksacmanaDecks {
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
	tbl, _, _, err := set.Matchup(ksacmanaDecks[0], ksacmanaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(512)
	if s := os.Getenv("KSACMANA_SEEDS"); s != "" {
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
		m, err := NewMatchup(reg, [2]string{"ksacmana-a", "ksacmana-b"}, decks, 1)
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
			ab, ok := sacCarriers[inst[x]]
			if !ok {
				continue
			}
			if k := a.Kind(); (ab < 0 && k == ActCast) || (ab >= 0 && k == ActActivate && int(a.Mode()) == ab) {
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
	t.Logf("%d seeds; carrier activations/casts: lowering surface %v, autopay surface %v", seeds, surf[0], surf[1])
	for n := range sacCarriers {
		for si, s := range surf {
			if s[n] < 50 {
				t.Errorf("%s: %d activations/casts on surface %d (want >= 50)", n, s[n], si)
			}
		}
	}
}
