package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MageZero gorge-fix lockstep, for the four cards gorge
// mis-played at the f9191e666 pin:
//   - Siege Dragon: the attack trigger's intervening-if counts the Walls the
//     defending player controls (IsPresent$ Wall.DefenderCtrl), at queue
//     time and on resolution, and its damage sweeps that player's creatures
//     without flying;
//   - Tactical Advantage: target blocking or blocked creature you control
//     (gorge "blocked", kernel PredBlocked);
//   - Cut Down: target creature with total power and toughness 5 or less
//     (gorge totalPT_LE5, kernel PredTotalPT);
//   - Sleep: tap all creatures target player controls; those creatures (only
//     those) don't untap during that player's next untap step (the
//     OpSkipNextUntap sweep).
//
// The expected answer at every decision is gorge's.
var kstdfixDecks = [2][]cardset.Entry{
	{
		{Name: "Siege Dragon", Count: 5}, {Name: "Tactical Advantage", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Serra Angel", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Sleep", Count: 4}, {Name: "Cut Down", Count: 4}, {Name: "Wall of Air", Count: 4},
		{Name: "Wall of Spears", Count: 4}, {Name: "Walking Corpse", Count: 6},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
	},
}

func TestStdfixLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stdfix", kstdfixDecks, 1000, "KSTDFIX_SEEDS")
}

// TestStdfixLockstepMutation proves the pool exercises each fix: widening
// the blocked or totalPT target to any creature, or Sleep's sweep to both
// players' creatures, makes some seed diverge.
func TestStdfixLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Table)
	}{
		{"blocked", func(tbl *kop.Table) { widenPred(tbl, kop.PredBlocked) }},
		{"totalPT", func(tbl *kop.Table) { widenPred(tbl, kop.PredTotalPT) }},
		{"sleep", func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					p := tbl.Cards[i].Abilities[j].Prog
					for k := range p {
						if p[k].Op == kop.OpSkipNextUntap && p[k].F != 0 {
							p[k].Obj = kop.RefNone
						}
					}
				}
			}
		}},
	} {
		if lockstepRun(t, "stdfix", kstdfixDecks, 300, false, m.fn) == 0 {
			t.Errorf("%s: no seed diverged with the mutation: the pool never exercises it", m.name)
		}
	}
}

// widenPred drops predicate p from every filter of the table.
func widenPred(tbl *kop.Table, p kop.Pred) {
	for i := range tbl.Filters {
		if tbl.Filters[i].Pred == p {
			tbl.Filters[i].Pred, tbl.Filters[i].PredN = kop.PredNone, 0
		}
	}
}
