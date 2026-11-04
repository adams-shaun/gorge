package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Khan, Vengeful Heart's static is a self permission that works from exile
// (S:Mode$ Continuous | Affected$ Card.Self | MayPlay$ True | AffectedZone$
// Exile | EffectZone$ Exile): once Swords to Plowshares exiles it, its owner
// may cast it again. The fixture pool is
// dirac/cardset/pools/fixtures/khan.

var khanDecks = []fixtureDecks{{
	{
		{Name: "Khan, Vengeful Heart", Count: 4}, {Name: "Walking Corpse", Count: 8},
		{Name: "Swamp", Count: 28},
	},
	{
		{Name: "Swords to Plowshares", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12},
	},
}}

var khanWatch = map[string]bool{"Khan, Vengeful Heart": true}

func TestKhanLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "khan", khanDecks, khanWatch, 300, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Khan, Vengeful Heart"] < 60 {
		t.Errorf("only %d Khan, Vengeful Heart casts", run.casts["Khan, Vengeful Heart"])
	}
}

// TestKhanMutation drops the own-exile permission: a Khan that Swords to
// Plowshares exiles can no longer be cast, so the lockstep must diverge.
func TestKhanMutation(t *testing.T) {
	run := runFixtureLockstep(t, "khan", khanDecks, khanWatch, 300, false, func(tbl *kop.Table) {
		mutateStatic(t, tbl, "Khan, Vengeful Heart", func(a *kop.Ability) {
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom {
				a.Static.StFlags &^= kop.StFunctionsInExile
			}
		})
	})
	run.requireDivergence(t, "the card is no longer castable from exile")
}

// mutateStatic rewrites one card's static abilities in a matchup table, on
// copies so the set's own table is not touched. Static abilities carry no
// program, so mutateCard (which walks programs) cannot reach them.
func mutateStatic(t *testing.T, tbl *kop.Table, name string, fn func(*kop.Ability)) {
	t.Helper()
	found := false
	for i := range tbl.Cards {
		cd := &tbl.Cards[i]
		if cd.Name != name {
			continue
		}
		found = true
		cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
		for j := range cd.Abilities {
			fn(&cd.Abilities[j])
		}
	}
	if !found {
		t.Fatalf("%s is not in the table", name)
	}
}
