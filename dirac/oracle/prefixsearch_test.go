package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// lockstepClean replays a std-tla repro and fails on any divergence.
func lockstepClean(t *testing.T, file string) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tla")
	r, err := ReadRepro(file)
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	var es [2][]cardset.Entry
	for i := range es {
		for _, n := range r.Cards[i] {
			es[i] = append(es[i], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, _, err := set.Matchup(es[0], es[1])
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	CloseSide(k)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %v", res.Div.At, res.Div.Field, res.Div.Diff)
	}
}

// TestPrefixSearchDoesNotPriceACardItDiscarded: Gran-Gran's loot trigger,
// fired by a tap in the prefix search, can discard the very spell being
// priced; the card is then not in hand to cast, so the search goes on
// (gorge prices a cast from a card no longer in its origin zone as
// unsupported).
func TestPrefixSearchDoesNotPriceACardItDiscarded(t *testing.T) {
	lockstepClean(t, "testdata/repro/prefix_search_discards_the_play.repro.json")
}

// TestRelaxedCensusCountsTheTapExtra: a Humble Merchant made by Honest Work
// taps for {C} plus a Badgermole Cub's {G}, so a {1}{G} creature is offered.
func TestRelaxedCensusCountsTheTapExtra(t *testing.T) {
	lockstepClean(t, "testdata/repro/relaxed_census_counts_tap_extra.repro.json")
}

// TestGrantedTapExtraPaysAFlashback: four Allies tapping for the any-colour
// grant, each beside a Badgermole Cub's {G}, pay Rockalanche's {5}{G}
// flashback; the walk's relaxed census counts the tap's extra, so the cast is
// searched and offered (CR 605.4a).
func TestGrantedTapExtraPaysAFlashback(t *testing.T) {
	lockstepClean(t, "testdata/repro/flashback_paid_through_granted_tap_extra.repro.json")
}
