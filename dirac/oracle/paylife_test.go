package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The life-cost lockstep (paylife): constructed matchups over the
// generated paylife pool, whose spells and abilities pay PayLife<N> beside
// mana. gorge's cast planner declines a spell whose own Cost$ carries a
// life part ("shape:additional_cost"), so Fumarole and Withering Boon are
// lowered or offered by the floating pool only; the activations are
// planned with their life part. The expected answer at every decision is
// gorge's.
var kpaylifeDecks = [2][]cardset.Entry{
	{
		{Name: "Fumarole", Count: 4}, {Name: "Withering Boon", Count: 3}, {Name: "Bitter Triumph", Count: 3},
		{Name: "Greed", Count: 1}, {Name: "Grimclaw Bats", Count: 3}, {Name: "Foul Familiar", Count: 3},
		{Name: "Book of Rass", Count: 1}, {Name: "Reckless Assault", Count: 1}, {Name: "Phyrexian Reclamation", Count: 1},
		{Name: "Hill Giant", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Fumarole", Count: 2}, {Name: "Withering Boon", Count: 2}, {Name: "Final Payment", Count: 3},
		{Name: "Stirge", Count: 2}, {Name: "Treasure Dredger", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Ornithopter", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Fountainport", Count: 2}, {Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 4},
	},
}

// lifeCarriers are the pool's life-cost carriers (their casts and
// activations are counted).
var lifeCarriers = map[string]bool{"Fumarole": true, "Withering Boon": true, "Bitter Triumph": true,
	"Final Payment": true, "Greed": true, "Grimclaw Bats": true, "Foul Familiar": true, "Book of Rass": true,
	"Reckless Assault": true, "Phyrexian Reclamation": true, "Stirge": true, "Treasure Dredger": true,
	"Fountainport": true}

func TestPaylifeLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("paylife")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kpaylifeDecks {
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
	tbl, _, _, err := set.Matchup(kpaylifeDecks[0], kpaylifeDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(512)
	if s := os.Getenv("KPAYLIFE_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	casts := map[string]int{}
	var surf [2]map[string]int
	surf[0], surf[1] = map[string]int{}, map[string]int{}
	var inst []string
	for _, d := range decks {
		for _, c := range d {
			inst = append(inst, cardName(c))
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kpaylife-a", "kpaylife-b"}, decks, 1)
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
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
				if x := int(a.Src()); x < len(inst) && lifeCarriers[inst[x]] {
					casts[inst[x]]++
					surf[si][inst[x]]++
				}
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
	t.Logf("%d seeds; life-cost carrier casts/activations %v (lowering surface %v, autopay surface %v)", seeds, casts, surf[0], surf[1])
	for n := range lifeCarriers {
		if casts[n] < 50 {
			t.Errorf("%s was cast or activated %d times (want >= 50)", n, casts[n])
		}
	}
}
