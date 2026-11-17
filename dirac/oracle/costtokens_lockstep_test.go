package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The cost tokens costHand now lowers, locked against gorge: Abandon Hope's
// announced-X Discard<X/Card/card> (the cast's X announced once, then that
// many discard picks), Vexing Devil's literal DamageYou<4> unless cost (the
// resolving source deals 4 damage to the payer), Rot Farm Skeleton's
// activated Mill<4> (the payer mills four from their own library's top to
// return it from their graveyard), Deep Spawn's UnlessCost$ Mill<2> (the
// sacrifice-it-unless-you-mill election: paying mills two from the payer's
// library's top, payUnlessCost's shared pay.PayMillCost), Priest of the
// Wakening Sun's UnlessCost$ Reveal<1/Creature.Dinosaur> (the reveal-pick
// "if you do" gain-life election: the card stays in hand) and the forecast
// family's activated Reveal<1/CARDNAME> (Pride of the Clouds, Sky Hussar and
// Karlov's Crossbow: the cost's only candidate is the source itself, CR 201.5,
// so a battlefield copy can never pay it and the ability is withheld; gorge
// never offers these abilities from the hand either — no ActivationZone$ Hand
// — so the forecast costs stay unpayable on both engines and the offer gates
// must agree) and Millikin's "{T}, Mill a card: Add {C}" (the mana ability's
// own Mill<1>: no CR 605.1a mana ability's cost moves a library card,
// cards.IsManaAbilitySA's movesLibraryCard, so gorge resolves the whole
// ability on the stack as an ordinary activated ability — offered at
// priority, its activation pays the tap and the mill through the same Cost.Mill
// arm every activated ability pays, and its resolution adds the {C} — never
// a manual-window source, a census unit or a planner one; the kernel lowers
// it through lower_stackmana.go stackManaAbility the same way).
func kcostTokensDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Abandon Hope", Count: 4},
		{Name: "Vexing Devil", Count: 4},
		{Name: "Rot Farm Skeleton", Count: 4},
		{Name: "Deep Spawn", Count: 2},
		{Name: "Priest of the Wakening Sun", Count: 2},
		{Name: "Pride of the Clouds", Count: 2},
		{Name: "Sky Hussar", Count: 2},
		{Name: "Karlov's Crossbow", Count: 2},
		{Name: "Tresserhorn's Lord, Returned", Count: 2},
		{Name: "Swamp", Count: 4},
		{Name: "Forest", Count: 1},
		// (one Island and one Forest became the Millikins; the lands' colour
		// mix stays so every card still casts and every unless election is
		// still paid within the seed run)
		{Name: "Island", Count: 5},
		{Name: "Millikin", Count: 2},
		{Name: "Plains", Count: 2},
		{Name: "Mountain", Count: 2},
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

// TestCostTokensLockstepMutationMill: with the Mill cost byte dropped the
// Skeleton returns from the graveyard without milling and a seed must
// diverge (the libraries' sizes differ).
func TestCostTokensLockstepMutationMill(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, kcostTokensDropMill)
	if n == 0 {
		t.Fatal("no seed diverged with the Mill cost byte dropped")
	}
}

// TestCostTokensLockstepMutationReveal: with the RevealSelf byte dropped the
// kernel offers the forecast ability on the battlefield, where gorge withholds
// it (the cost's only candidate is the source itself, CR 201.5), and a seed
// must diverge.
func TestCostTokensLockstepMutationReveal(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, kcostTokensDropReveal)
	if n == 0 {
		t.Fatal("no seed diverged with the RevealSelf cost byte dropped")
	}
}

// TestCostTokensLockstepMutationUnlessMill: with the PayBSubMill sentinel
// dropped Deep Spawn's unless mill becomes a plain PayDraw — the payer draws
// two instead of milling two — and a seed must diverge (the libraries and
// graveyards differ).
func TestCostTokensLockstepMutationUnlessMill(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, kcostTokensDropUnlessMill)
	if n == 0 {
		t.Fatal("no seed diverged with the PayBSubMill sentinel dropped")
	}
}

// TestCostTokensLockstepMutationPayReveal: with the PayBReveal sentinel
// dropped Priest's unless reveal becomes a plain PayDiscard — the payer
// discards the picked hand card where gorge reveals it and nothing moves —
// and a seed must diverge.
func TestCostTokensLockstepMutationPayReveal(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "costtokens", kcostTokensDecks(), 60, false, kcostTokensDropPayReveal)
	if n == 0 {
		t.Fatal("no seed diverged with the PayBReveal sentinel dropped")
	}
}

// kcostTokensDropUnlessMill clears the PayBSubMill sentinel off every
// pay instruction (kop.PayBSubDamage's model: the B.N sentinel is the only
// carrier of the unless-mill shape).
func kcostTokensDropUnlessMill(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			prog := append([]kop.Instr(nil), abs[j].Prog...)
			for k := range prog {
				if prog[k].Op == kop.OpUnlessPaid || prog[k].Op == kop.OpIfPaid || prog[k].Op == kop.OpUnlessSwitched {
					if int(prog[k].C&0xF) == kop.PayDraw && prog[k].B.N == kop.PayBSubMill {
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

// kcostTokensDropPayReveal clears the PayBReveal sentinel off every pay
// instruction: the unless reveal becomes a PayDiscard of the picked hand card.
func kcostTokensDropPayReveal(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			prog := append([]kop.Instr(nil), abs[j].Prog...)
			for k := range prog {
				if prog[k].Op == kop.OpUnlessPaid || prog[k].Op == kop.OpIfPaid || prog[k].Op == kop.OpUnlessSwitched {
					if int(prog[k].C&0xF) == kop.PayDiscard && prog[k].B.N == kop.PayBReveal {
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

func kcostTokensDropMill(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			abs[j].Cost.Mill = 0
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}

func kcostTokensDropReveal(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			abs[j].Cost.RevealSelf = false
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
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
