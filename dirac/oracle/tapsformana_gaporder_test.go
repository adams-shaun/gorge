package oracle

import (
	"testing"
)

// TestTapsForManaRikalaGapKeyOrder plays the Rikala cases' own random games
// (the PINS_RECORD search's seeds) and requires no divergence. Seed 4 casts a
// second Rikala, Homarid King with a gated Island and a spare Forest beside
// two Islands: gorge's second pass lists its gap rows with Colours 0, so the
// plans {Forest, Island, Island} and {Forest, Forest, Island} tie on keys 1
// to 7 and key 8 takes the first by source ID, and its payment stops once the
// first Island's floated extra pays. The kernel counted the gap Islands'
// colour in keys 6 and 7, preferred the plan that leaves an Island, and tapped
// the second Forest.
func TestTapsForManaRikalaGapKeyOrder(t *testing.T) {
	reg := synthReg(t)
	cases := tapsForManaCases()
	for _, c := range []effectLongCase{cases[3], cases[4]} {
		set := printedDelveSet(t, c.decks)
		for seed := uint64(1); seed <= 12; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom"}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			res, _ := effectLongPlay(t, reg, c, r, set, effectLongPolicy(m, c, seed))
			if res.Div != nil {
				t.Fatalf("%s seed %d: divergence at %d: %s %s %v", c.name, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if seed == 4 && len(res.Actions) < 64 {
				t.Fatalf("%s seed 4 played %d decisions: it no longer reaches the second Rikala's cast (decision 64)", c.name, len(res.Actions))
			}
		}
	}
}
