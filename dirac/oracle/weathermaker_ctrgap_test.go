package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestWeatherMakerCounterGapMutation: Weather Maker's "{T}, Remove two charge
// counters: Add {C}{C}" is a counter-cost gap beside its plain {T} ability
// (kernel mana_countercost.go). Both std-tmt repros of it diverge when the
// counter gaps are hidden (dirac.DebugNoCtrMana), so each depends on the gap,
// and each diverges at the decision where gorge offers a play only the
// counters pay, not earlier. (The first repro's play is Frog Butler's {2}
// with every land tapped: Weather Maker's lone any-colour mana cannot pay it,
// so gorge offering it proves Weather Maker holds at least two charge
// counters then.)
func TestWeatherMakerCounterGapMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCtrMana = true
	defer func() { dirac.DebugNoCtrMana = false }()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tmt")
	for _, c := range []struct {
		name    string
		autopay bool
	}{
		{"second_ability_counter_cost_taps_source_for_generic", true},
		{"second_ability_counter_gap_tries_plain_ability_first", false},
	} {
		r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
		if err != nil {
			t.Fatal(err)
		}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		entries := func(names []string) []cardset.Entry {
			es := make([]cardset.Entry, len(names))
			for i, n := range names {
				es[i] = cardset.Entry{Name: n, Count: 1}
			}
			return es
		}
		tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.Config.GorgeAutopaySurface = c.autopay
		rc.MaxDecisions = len(r.Actions) + 200
		k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
		CloseSide(k)
		if res.Div == nil {
			t.Errorf("%s: no divergence without the counter-cost gap", c.name)
		}
	}
}
