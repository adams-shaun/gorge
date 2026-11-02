package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Bonecache Overseer's draw is offered only when three or more cards left
// the graveyard this turn or a Food was sacrificed this turn: the count is the
// cards that left plus three per Food.
var kleftgyDecks = [2][]cardset.Entry{{
	{Name: "Bonecache Overseer", Count: 6}, {Name: "Raise Dead", Count: 3}, {Name: "Gravedigger", Count: 3},
	{Name: "Regrowth", Count: 2}, {Name: "Bake into a Pie", Count: 4}, {Name: "Thought Scour", Count: 4},
	{Name: "Cauldron Familiar", Count: 3}, {Name: "Gravecrawler", Count: 3}, {Name: "Swamp", Count: 8}, {Name: "Island", Count: 4},
}, {
	{Name: "Bonecache Overseer", Count: 4}, {Name: "Tragic Slip", Count: 4}, {Name: "Duress", Count: 3}, {Name: "Mulldrifter", Count: 3},
	{Name: "Bake into a Pie", Count: 3}, {Name: "Gravedigger", Count: 4}, {Name: "Cauldron Familiar", Count: 3},
	{Name: "Gravecrawler", Count: 4}, {Name: "Swamp", Count: 9}, {Name: "Island", Count: 3},
}}

func TestLeftGraveyardGateLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "leftgraveyardgate", kleftgyDecks, 300, "KLEFTGRAVEYARDGATE_SEEDS")
}

// TestLeftGraveyardGateMutation: with the activation condition dropped, the
// draw is offered at other times than gorge's.
func TestLeftGraveyardGateMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "leftgraveyardgate", kleftgyDecks, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbActivated {
					abs[j].Trig.Cond = 0
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the gate dropped")
	}
}

// TestLeftGraveyardGateFoodMutation: with the sacrificed Foods subtracted
// instead of added, the draw is offered at other times than gorge's.
func TestLeftGraveyardGateFoodMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "leftgraveyardgate", kleftgyDecks, 300, false, func(tbl *kop.Table) {
		tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
		for i := range tbl.Exprs {
			if tbl.Exprs[i].Kind == kop.ExSacrificedThisTurn {
				tbl.Exprs[i].Neg = true
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the Foods subtracted")
	}
}
