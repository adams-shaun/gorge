package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The second-mana-ability lockstep (dirac/cardset/pools/fixtures/manasecond):
//
//   - Baxter Building: "{T}: Add {C}" beside "{4}, {T}: Add four mana in any
//     combination of colors" (Produced$ Combo Any | Amount$ 4). gorge's
//     planner census prices only the free {C} ability, so a play the census
//     cannot pay takes the prefix search, which activates the filter (paying
//     its {4} from four Wastes) and allocates the four mana over the
//     colours the cost needs. Breya, Etherium Shaper costs {W}{U}{B}{R}, so
//     it is payable only through the filter.
//   - Haunted Screen: "{T}: Add {W} or {B}" beside "{T}, Pay 1 life: Add {G},
//     {U}, or {R}". gorge prices the first as its normal alternatives and
//     the second as a last-resort Consequence{Life: 1}, as it does a
//     painland's damage; the kernel's pain source follows. Moss Viper's {G}
//     is payable only through the life ability; Valiant Guard's {W}
//     through the free one.
//
// Each deck holds exactly the colourless lands one activation of its source
// needs (four for the filter's fee, three for Haunted Screen's {3}), so no
// payment ever chooses between two identical sources: the comparison is the
// shapes, not the source tie-break.
//
// The expected answer at every decision is gorge's.
var kmanasecondDecks = [2][]cardset.Entry{
	{
		{Name: "Baxter Building", Count: 1}, {Name: "Breya, Etherium Shaper", Count: 35},
		{Name: "Wastes", Count: 4},
	},
	{
		{Name: "Haunted Screen", Count: 1}, {Name: "Moss Viper", Count: 18},
		{Name: "Valiant Guard", Count: 18}, {Name: "Wastes", Count: 3},
	},
}

func TestSecondManaAbilityLockstep(t *testing.T) {
	t.Parallel()
	// The precondition: the fixture plays these cards (a table refuses a deck
	// holding a card it did not lower), and each intended payment happens.
	exercised := map[string]int{}
	manasecondRun(t, 24, func(m *Matchup, res *Result) {
		for _, a := range res.Actions {
			if a.Kind() == ActCast {
				if src := a.Src(); src != RefNone {
					exercised[m.InstanceCard(int(src))]++
				}
			}
		}
	})
	for _, name := range []string{"Breya, Etherium Shaper", "Moss Viper", "Valiant Guard"} {
		if exercised[name] == 0 {
			t.Fatalf("no %s cast in 24 seeds: the pool does not exercise the shape (%v)", name, exercised)
		}
	}
	lockstepSet(t, "manasecond", kmanasecondDecks, 200, "KMANASECOND_SEEDS")
}

// manasecondRun plays seeds lockstep games of the fixture, failing on a
// divergence, and hands each clean result to check.
func manasecondRun(t *testing.T, seeds uint64, check func(m *Matchup, res *Result)) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manasecond")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kmanasecondDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kmanasecondDecks[0], kmanasecondDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"manasecond-a", "manasecond-b"}, gd, 1)
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
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nlast actions %v (implicates %v)",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions,
				actionStrings(res.Actions[lo:d.At]), d.Implicates(m))
		}
		if check != nil {
			check(m, &res)
		}
	}
}
