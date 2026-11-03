package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The targeted investigate (CR 701.36): Panther Pounce's "target player
// investigates" makes the Clue for the player the spell targets, which the
// random policy picks as either seat. The fixture pool is
// dirac/cardset/pools/fixtures/lowinvest.

var lowInvestDecks = []fixtureDecks{{
	{
		{Name: "Panther Pounce", Count: 10}, {Name: "Savannah Lions", Count: 6}, {Name: "Raging Goblin", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Panther Pounce", Count: 6}, {Name: "Savannah Lions", Count: 8}, {Name: "Shock", Count: 6},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	},
}}

var lowInvestWatch = map[string]bool{"Panther Pounce": true}

func TestLowInvestLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "lowinvest", lowInvestDecks, lowInvestWatch, 300, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Panther Pounce"] < 100 {
		t.Errorf("only %d Panther Pounce casts", run.casts["Panther Pounce"])
	}
}

// TestLowInvestMutation makes every Clue go to the spell's controller, as
// the untargeted form would: the lockstep must diverge when the target is
// the opponent.
func TestLowInvestMutation(t *testing.T) {
	run := runFixtureLockstep(t, "lowinvest", lowInvestDecks, lowInvestWatch, 64, false, func(tbl *kop.Table) {
		mutateCard(t, tbl, "Panther Pounce", func(a *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpCreateToken && in.Obj == kop.RefTarget0 {
				in.Obj = kop.RefController
			}
		})
	})
	run.requireDivergence(t, "the Clue goes to the controller")
}
