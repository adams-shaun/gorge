package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// budgetSeeds replay the granted_prefix_tap_trigger_pass fixture's matchup
// under castFirstPolicy. The ticket's eleven are the seeds its dependency
// ticket priced (one play, one 160-node prefix search); 2 and 74 are the
// priority-over-offer seeds this ticket's re-measured sweep added (the
// budget's continuation into scriptOffers' filter search and the
// granted-mana float). 35, 99, 177 and 193 hold different classes (a tap's
// extra on an anyTapExtra board; the waterbend ask's missing done) and are
// deliberately not in the table.
var budgetSeeds = []uint64{2, 7, 13, 38, 72, 74, 85, 89, 109, 123, 128, 153, 183}

// TestGrantedFilterPrefixBudgetLockstep: on the fixture matchup under
// castFirstPolicy the kernel must offer the plays gorge's one prefix search
// per play offers -- grantCap's grant search and scriptOffers' filter search
// run on one budget per play (script_gen.go capSpent), and the filter search
// of an activation floats no granted mana (mana_mixgaps.go otherGaps), so no
// play gorge's exhausted search refused comes back on a second one.
func TestGrantedFilterPrefixBudgetLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/granted_prefix_tap_trigger_pass.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tla")
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range budgetSeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			gseed := seed*7919 + 13
			k, err := SetSide(set, tbl, Config{})(m, gseed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			res := Lockstep(m, gseed, k, castFirstPolicy(gseed), RunConfig{Config: Config{}, MaxDecisions: 3000, MaxTurns: 100})
			if res.Div != nil {
				t.Errorf("seed %d diverged at %d: %s %s implicates %v\n gorge %v\n dirac %v", seed, res.Div.At,
					res.Div.What, res.Div.Field, res.Div.Implicates(m), res.Div.RefOptions, res.Div.KerOptions)
			}
		})
	}
}

// TestGrantedPrefixCastFirstTailLockstep: the fixture's own replay, its
// scripted actions followed by a castFirst tail -- the tail is where the
// grant-and-filter boards come up, and the priority surface must hold there
// too. At this branch's base the run is already div-free (measured); the
// pinned regressions are the budgetSeeds table above.
func TestGrantedPrefixCastFirstTailLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/granted_prefix_tap_trigger_pass.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tla")
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, Config{})(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pol := &ScriptPolicy{Actions: r.Actions, Tail: castFirstPolicy(r.Seed)}
	res := Lockstep(m, r.Seed, k, pol, RunConfig{Config: Config{}, MaxDecisions: 3000, MaxTurns: 100})
	if pol.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", pol.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s implicates %v\n gorge %v\n dirac %v", res.Div.At,
			res.Div.What, res.Div.Field, res.Div.Implicates(m), res.Div.RefOptions, res.Div.KerOptions)
	}
}
