package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The cost tokens costHand now lowers, locked against gorge: Abandon Hope's
// announced-X Discard<X/Card/card> (the cast's X announced once, then that
// many discard picks) and Vexing Devil's literal DamageYou<4> unless cost (the
// resolving source deals 4 damage to the payer). The deck is only those two
// spells and the basics that pay them, so every cast exercises an arm.
func kcostTokensDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Abandon Hope", Count: 8},
		{Name: "Vexing Devil", Count: 8},
		{Name: "Swamp", Count: 10},
		{Name: "Mountain", Count: 10},
		{Name: "Island", Count: 4},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

func TestCostTokensLockstep(t *testing.T) {
	t.Parallel()
	lockstepRunPolicy(t, "costtokens", kcostTokensDecks(), 60, true, nil, nil, castFirstPolicy)
}

// TestCostTokensLockstepMutationDiscardX: with the announced-X marker dropped
// (CostDiscard N 0, i.e. discard nothing) Abandon Hope's cost stops asking
// gorge's discard picks and a seed must diverge.
func TestCostTokensLockstepMutationDiscardX(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, kcostTokensDropDiscardX)
	if n == 0 {
		t.Fatal("no seed diverged with the Discard<X> marker dropped")
	}
}

// TestCostTokensLockstepMutationDamageYou: with the damage sentinel dropped
// the unless cost pays by drawing instead of dealing damage and a seed must
// diverge.
func TestCostTokensLockstepMutationDamageYou(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, kcostTokensDropDamageYou)
	if n == 0 {
		t.Fatal("no seed diverged with the DamageYou sentinel dropped")
	}
}

func kcostTokensDropDiscardX(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		cards[i].SpellCost.DiscardX = false
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			abs[j].Cost.DiscardX = false
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}

func kcostTokensDropDamageYou(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			prog := append([]kop.Instr(nil), abs[j].Prog...)
			for k := range prog {
				if prog[k].Op == kop.OpUnlessPaid || prog[k].Op == kop.OpUnlessSwitched {
					if prog[k].C&0xF == kop.PayDraw && prog[k].B.N == kop.PayBSubDamage {
						prog[k].B = kop.Const(0)
					}
				}
			}
			abs[j].Prog = prog
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}
