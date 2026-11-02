package oracle

import (
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Restless creature-land lockstep: the seven Restless lands'
// "becomes a <P/T> <colour> and <colour> <type> creature until end of turn"
// (Animate Colors$ with OverwriteColors$ True: layer 5 replaces the land's
// colourlessness, CR 613.1e) and their attack triggers, beside colour-
// sensitive removal (Doom Blade's nonblack, Ultimate Price's monocoloured),
// burn and vanilla creatures. The expected answer at every decision is gorge's.
var krestlessDecks = [2][]cardset.Entry{
	{
		{Name: "Restless Bivouac", Count: 3}, {Name: "Restless Fortress", Count: 3}, {Name: "Restless Prairie", Count: 3},
		{Name: "Restless Ridgeline", Count: 3}, {Name: "Plains", Count: 3}, {Name: "Mountain", Count: 2}, {Name: "Forest", Count: 1},
		{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Swords to Plowshares", Count: 2}, {Name: "Ultimate Price", Count: 2},
		{Name: "Shock", Count: 4},
	},
	{
		{Name: "Restless Cottage", Count: 4}, {Name: "Restless Reef", Count: 4}, {Name: "Restless Vinestalk", Count: 4},
		{Name: "Island", Count: 2}, {Name: "Swamp", Count: 2}, {Name: "Forest", Count: 2},
		{Name: "Wind Drake", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Doom Blade", Count: 4}, {Name: "Ultimate Price", Count: 2}, {Name: "Hill Giant", Count: 4},
	},
}

func krestlessSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	return p3Setup(t, "restless", krestlessDecks)
}

// p3Setup loads the corpus, the named set and the two 40-card decks, and
// builds the matchup table (the lockstep pools).
func p3Setup(t *testing.T, name string, dl [2][]cardset.Entry) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(name)
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range dl {
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
	tbl, _, _, err := set.Matchup(dl[0], dl[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func TestRestlessLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := krestlessSetup(t)
	p3Lockstep(t, "restless", reg, set, decks, tbl, 600)
}

// p3Lockstep plays seeds games of the pool against gorge (every fourth seed
// on gorge's autopay surface, every eighth with the random policy) and
// fails on any divergence; <NAME>_SEEDS overrides the seed count.
func p3Lockstep(t *testing.T, name string, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seeds uint64) {
	t.Helper()
	if s := os.Getenv(strings.ToUpper(name) + "_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	acts, clean := 0, 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{name + "-a", name + "-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
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
			if a.Kind() == ActActivate {
				acts++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
		clean++
	}
	t.Logf("%d seeds, %d clean; %d activations", seeds, clean, acts)
}

// TestRestlessLockstepMutation proves the pool exercises the colour half of
// the animation: with every override's colour set dropped from the table (an
// animated Cottage or Reef stays colourless, so Doom Blade can target it),
// some seed diverges from gorge.
func TestRestlessLockstepMutation(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := krestlessSetup(t)
	mut := *tbl
	mut.Overrides = append([]kop.Override(nil), tbl.Overrides...)
	n := 0
	for i := range mut.Overrides {
		if mut.Overrides[i].HasColors {
			mut.Overrides[i].HasColors, mut.Overrides[i].SetColors = false, 0
			n++
		}
	}
	if n == 0 {
		t.Fatal("no override in the table sets colours")
	}
	for seed := uint64(1); seed <= 200; seed++ {
		m, err := NewMatchup(reg, [2]string{"krestless-a", "krestless-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, &mut, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverges without the animation colours (decision %d: %s)", seed, res.Div.At, res.Div.What)
			return
		}
	}
	t.Fatal("no seed diverged with the animation colours dropped: the pool does not exercise them")
}
