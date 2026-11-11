package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Paladin of Atonement lockstep: "at the beginning of each upkeep, if
// you lost life last turn, put a +1/+1 counter on it" (gorge
// Engine.LifeLostLastTurn, the previous turn's window of the log; the
// kernel's lastLost, latched as the turn changes) and "when it dies,
// you gain life equal to its toughness", against burn and attackers that
// make its controller lose life on the opponent's turn. The expected answer
// at every decision is gorge's.
var kpaladinatoneDecks = [2][]cardset.Entry{
	{
		{Name: "Paladin of Atonement", Count: 14}, {Name: "Savannah Lions", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Hill Giant", Count: 10},
		{Name: "Mountain", Count: 18},
	},
}

func TestPaladinAtonementLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "paladinatone", kpaladinatoneDecks, 400, "KPALADINATONE_SEEDS")
}

// TestPaladinAtonementLockstepMutation proves the pool reads last turn's
// loss: reading this turn's tally instead makes some seed diverge (at an
// upkeep, this turn's loss is almost always 0).
func TestPaladinAtonementLockstepMutation(t *testing.T) {
	t.Parallel()
	hit := false
	n := lockstepRun(t, "paladinatone", kpaladinatoneDecks, 200, false, func(tbl *kop.Table) {
		tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...) // the set's table is shared with the parallel lockstep
		for i := range tbl.Exprs {
			if tbl.Exprs[i].Kind == kop.ExLifeLostLastTurn {
				tbl.Exprs[i].Kind = kop.ExLifeLostThisTurn
				hit = true
			}
		}
	})
	if !hit {
		t.Fatal("no ExLifeLostLastTurn in the pool's table: Paladin of Atonement's condition was not lowered to it")
	}
	if n == 0 {
		t.Error("no seed diverged with this turn's loss read for last turn's: the pool never exercises it")
	}
}
