package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestCmd5ReprosLockstep replays 1v1 Commander games from the cmd5 pool (the
// Foundations Commander precons) on the surface each was found on, and 200
// decisions past its script.
func TestCmd5ReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Set(t, "cmd5")
	for _, name := range []string{
		"cmd5_ancient_tomb_two_colourless_fund_a_spell", // kfuzz cmd5 heuristic pair 10: Ancient Tomb's one activation adds {C}{C} (a pain-only source, 2 damage), so beside two Mountains it funds Archwing Dragon's {2}{R}{R} and the cast is offered (CR 106.1, 605.1a)
	} {
		t.Run(name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
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
			rc.MaxDecisions = len(r.Actions) + 200
			k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if res.Decisions <= len(r.Actions) {
				t.Fatalf("%d decisions: the game ended inside the script (%d actions)", res.Decisions, len(r.Actions))
			}
		})
	}
}
