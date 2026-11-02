package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorgeDiscardedFixTime is gorge cr-discarded-cost-value-ref (79fe01b4f,
// 2026-10-26 11:07:01 UTC): before it the Discarded$ value ref read 0, so
// Grab the Prize never dealt its damage.
const gorgeDiscardedFixTime = "20261026110701"

// The Grab the Prize lockstep: "As an additional cost, discard a
// card. Draw two cards. If the discarded card wasn't a land card, deal 2
// damage to each opponent" -- RefDiscarded, the cost-discarded card
// recorded in the spell entry (Discarded$Valid Card.nonLand/Times.2),
// against land and nonland discards; and DamageAll ValidPlayers$
// Player.Opponent lowered as damage to each opponent (Serrated Scorpion's
// dies trigger, Fanatic of Mogis's devotion). The expected answer at every
// decision is gorge's.
var kgrabprizeDecks = [2][]cardset.Entry{
	{
		{Name: "Grab the Prize", Count: 12}, {Name: "Raging Goblin", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Mountain", Count: 16},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Serrated Scorpion", Count: 4}, {Name: "Fanatic of Mogis", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Grab the Prize", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 9},
	},
}

func TestGrabprizeLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeDiscardedFixTime) {
		t.Skip("gorge before cr-discarded-cost-value-ref (79fe01b4f): Discarded$ reads 0")
	}
	lockstepSet(t, "grabprize", kgrabprizeDecks, 500, "KGRABPRIZE_SEEDS")
}

// TestGrabprizeLockstepMutation proves the pool reads the discarded card:
// with RefDiscarded unread (the pre-fix gorge's 0), some seed diverges.
func TestGrabprizeLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeDiscardedFixTime) {
		t.Skip("gorge before cr-discarded-cost-value-ref (79fe01b4f)")
	}
	n := lockstepRun(t, "grabprize", kgrabprizeDecks, 100, false, func(tbl *kop.Table) {
		tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...) // the pool's Exprs is shared with the parallel lockstep
		for i := range tbl.Exprs {
			if tbl.Exprs[i].Ref == kop.RefDiscarded {
				tbl.Exprs[i].Ref = kop.RefNone
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the discarded card unread")
	}
}
