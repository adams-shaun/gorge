package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Two value heads in lockstep with gorge, on one fixture pool
// (fixtures/tpyardmost):
//
//   - TargetedPlayer$ValidGraveyard <spec> (kgen targetedPlayerGraveyard:
//     the YouOwn clauses become the OwnedBy Targeted role, kernel
//     cardRoleF2; gorge evalPlayerRefProperty with the targeted player as
//     You): Corpse Augur's dies trigger, The Mouth of Sauron's amass.
//   - Count$MostCardName <spec> (kgen mostCardNameValue, kernel ExValidAgg
//     kop.AggMostName; gorge countMostCardName): Endless Atlas's activation
//     gate, Chrome Replicator's intervening-if.

var tpYardMostWatch = map[string]bool{
	"Corpse Augur": true, "The Mouth of Sauron": true, "Liliana, Death Mage": true,
	"Endless Atlas": true, "Chrome Replicator": true, "Mechanized Production": true,
}

var tpYardMostUB = []cardset.Entry{
	{Name: "Corpse Augur", Count: 7}, {Name: "The Mouth of Sauron", Count: 5}, {Name: "Liliana, Death Mage", Count: 2},
	{Name: "Walking Corpse", Count: 8}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 6},
}

var tpYardMostArt = []cardset.Entry{
	{Name: "Endless Atlas", Count: 8}, {Name: "Chrome Replicator", Count: 5}, {Name: "Mechanized Production", Count: 3},
	{Name: "Walking Corpse", Count: 6}, {Name: "Island", Count: 10}, {Name: "Swamp", Count: 8},
}

var tpYardMostDecks = []fixtureDecks{{tpYardMostUB, tpYardMostArt}, {tpYardMostArt, tpYardMostUB}}

func TestTargetedYardMostNameLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "tpyardmost", tpYardMostDecks, tpYardMostWatch, 160, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	for _, n := range []string{"Corpse Augur", "The Mouth of Sauron", "Endless Atlas", "Chrome Replicator"} {
		if run.casts[n] < 10 {
			t.Errorf("%s cast %d times", n, run.casts[n])
		}
	}
	// the most-name fold read as the distinct-name count
	mut := runFixtureLockstep(t, "tpyardmost", tpYardMostDecks, tpYardMostWatch, 160, false, func(tbl *kop.Table) {
		tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
		n := 0
		for i := range tbl.Exprs {
			if x := &tbl.Exprs[i]; x.Kind == kop.ExValidAgg && x.N == kop.AggMostName {
				x.N = kop.AggDistinctNames
				n++
			}
		}
		if n == 0 {
			t.Fatal("precondition: no table expression is AggMostName")
		}
	})
	mut.requireDivergence(t, "MostCardName read as DifferentCardNames")
	// the targeted player's graveyard read as the other seat's
	dirac.DebugReferentOwnerSwap = true
	mut = runFixtureLockstep(t, "tpyardmost", tpYardMostDecks, tpYardMostWatch, 160, false, nil)
	dirac.DebugReferentOwnerSwap = false
	mut.requireDivergence(t, "TargetedPlayer$ValidGraveyard read on the other owner")
}
