package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// TestHBotParitySynth holds the port's agreement with gorge's bot on the synthetic
// pools whose cards reach a rule the FDN decks barely touch. Each row names the
// botpolicy behaviour that keeps its kind above the floor.
func TestHBotParitySynth(t *testing.T) {
	// serial: AddDecks writes the corpus the parallel tests share

	c, _ := testCorpus(t)
	for _, tc := range []struct {
		pool, kind string
		full       bool // dirac.SurfaceFull, the surface the search plays
		floor      float64
		why        string
	}{
		{pool: "std-eoe", kind: "priority", floor: 0.98, why: "Station is a \"station\" option, which chooseAbility never ranks"},
		{pool: "std-dft", kind: "priority", floor: 0.99, why: "a max speed ability is a \"granted\" option, which chooseAbility never ranks"},
		{pool: "std-woe", kind: "priority", floor: 0.98, why: "paymentActionsForPriority plans neither a face cast nor an alternative cost, and manual taps float before the ranking"},
		{pool: "std-hob", kind: "priority", floor: 0.98, why: "the same, for Adventures and Omens"},
		{pool: "std-dsk", kind: "priority", floor: 0.985, why: "an impended cast has no plan"},
		{pool: "std-dft", kind: "attackers", floor: 0.98, why: "the census reads effective types: a crewed Vehicle attacks"},
		{pool: "std-tla", kind: "blockers", floor: 0.97, why: "the census reads effective types: an animated land blocks"},
		{pool: "std-dsk", kind: "attackers", floor: 0.95, why: "a face-down noncreature card stays out of the census"},
		{pool: "std-dsk", kind: "target", floor: 0.98, why: "attachRanking's polarity aims a boon Aura at the own board"},
		{pool: "std-dsk", kind: "choose", floor: 0.97, why: "a yes/no confirmation takes its first offer, yes"},
		{pool: "std-lci", kind: "modes", floor: 0.98, why: "an optional play from a zone takes its first Min, none"},
		{pool: "std-msh", kind: "arrange", floor: 0.98, why: "a dig's rest tops up in offered order"},
		{pool: "std-tmt", kind: "choose", full: true, floor: 0.90, why: "a hybrid pip takes the symbol's first colour"},
	} {
		set := cardsettest.Synth(t, tc.pool)
		pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
		if err != nil {
			t.Fatal(err)
		}
		c.AddDecks(files)
		mk := func(tbl *kop.Table) KernelBot { return hbot.New(HBotFactsSet(c.Reg, tbl, nil, set)) }
		n, agree := 0, 0
		for _, p := range PairingsFrom(pool, 0, 40, 78) {
			res, err := HBotParityGameOpts(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: !tc.full, PassElision: true}, 20000, mk, HBotParityOptions{Set: set})
			if err != nil {
				continue // a pairing the pool refuses by name
			}
			for _, r := range res.Recs {
				if !r.Unoffered && r.Kind == tc.kind {
					n++
					if r.Agree {
						agree++
					}
				}
			}
		}
		if n < 20 {
			t.Errorf("%s %s: only %d decisions", tc.pool, tc.kind, n)
			continue
		}
		rate := float64(agree) / float64(n)
		t.Logf("%s %-10s %5d %.4f", tc.pool, tc.kind, n, rate)
		if rate < tc.floor {
			t.Errorf("%s %s: agreement %.4f < %.2f over %d decisions (%s)", tc.pool, tc.kind, rate, tc.floor, n, tc.why)
		}
	}
}
