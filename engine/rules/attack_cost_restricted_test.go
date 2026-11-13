// Combat-tax affordability prices the restriction-aware floating pool
// (CR 106.6): restricted mana a PurposeOther cost cannot spend is neither
// offered as payment nor settled against. Four sites share
// combatManaAvailable: the offer gate, the inline-settle read and the two
// Phyrexian branch probes.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const combatTax2 = "Name:ZZ Attack Tax\nTypes:Enchantment\n" +
	"S:Mode$ CantAttackUnless | ValidCard$ Creature | Target$ You | Cost$ 2 | Description$x\nOracle:x\n"
const blockTax2 = "Name:ZZ Block Tax\nTypes:Enchantment\n" +
	"S:Mode$ CantBlockUnless | ValidCard$ Creature | Cost$ 2 | Description$x\nOracle:x\n"
const phyTax = "Name:ZZ Phy Tax\nTypes:Enchantment\n" +
	"S:Mode$ CantAttackUnless | ValidCard$ Creature | Target$ You | Cost$ W/P | Description$x\nOracle:x\n"

const restrBear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n"

// seedRestricted floats n restricted mana of colour idx/letter for p.
func seedRestricted(e *Engine, p state.PlayerID, idx int, letter string, n int, valid string) {
	e.G.Players[p].Pool[idx] = int32(n)
	e.G.Players[p].RestrictedMana = []state.ManaRestriction{{Color: letter, Amount: int32(n), Valid: valid}}
}

func availTotal(e *Engine, p state.PlayerID, c Cost) int {
	return int(pay.AvailableFor(asPayer(e), p, pay.Descriptor{Class: pay.PurposeOther, Cost: &c}).Pool.Total())
}

func attackBoard(t *testing.T, valid string) (*Engine, state.ObjID) {
	t.Helper()
	e := threeSeatEngine(t)
	onBoardCard(t, e, 0, card(t, combatTax2))
	bear := onBoardReady(t, e, 1, restrBear)
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	seedRestricted(e, 1, state.MR, "R", 2, valid)
	return e, bear
}

func TestCombatTaxHidesUnadmittedRestrictedMana(t *testing.T) {
	t.Parallel()
	e, bear := attackBoard(t, "Static.Unlock,Spell.Room")
	ch := e.attackPairCharge(bear, 0, 0)
	if ch.mana != 2 || e.G.Players[1].Pool.Total() != 2 || availTotal(e, 1, Cost{Generic: 2}) != 0 {
		t.Fatalf("precondition: charge %+v raw %d avail %d", ch, e.G.Players[1].Pool.Total(), availTotal(e, 1, Cost{Generic: 2}))
	}
	m := map[state.ObjID]bool{bear: true}
	if e.combatChargeAffordable(1, ch, m, m) {
		t.Fatal("charge affordable from unadmitted restricted mana")
	}
	e.askAttackers()
	if d := e.Pending(); d != nil && findAttackOption(d, bear, 0) != nil {
		t.Fatalf("pair offered: %+v", d.Options)
	}

	// Positive control on a fresh engine.
	e, bear = attackBoard(t, "nonSpell")
	if availTotal(e, 1, Cost{Generic: 2}) != 2 {
		t.Fatal("control precondition: nonSpell batch must be available")
	}
	e.askAttackers()
	d := e.Pending()
	if d == nil || findAttackOption(d, bear, 0) == nil {
		t.Fatalf("control: admitted restricted mana did not offer the pair")
	}
}

func blockBoard(t *testing.T, valid string, lands int) (*Engine, state.ObjID, state.ObjID, []state.ObjID) {
	t.Helper()
	e := threeSeatEngine(t)
	onBoardCard(t, e, 0, card(t, blockTax2))
	var ms []state.ObjID
	for i := 0; i < lands; i++ {
		ms = append(ms, onBoardCard(t, e, 0, card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")))
	}
	blocker := onBoardCard(t, e, 0, card(t, memniteSrc))
	att := onBoardReady(t, e, 1, restrBear)
	attackSeat0(t, e, att)
	seedRestricted(e, 0, state.MR, "R", 2, valid)
	return e, blocker, att, ms
}

func TestBlockTaxHidesUnadmittedRestrictedMana(t *testing.T) {
	t.Parallel()
	e, blocker, att, _ := blockBoard(t, "Static.Unlock,Spell.Room", 0)
	ch := e.blockPairCharge(blocker, att)
	if ch.mana != 2 || e.G.Players[0].Pool.Total() != 2 || availTotal(e, 0, Cost{Generic: 2}) != 0 {
		t.Fatalf("precondition: charge %+v", ch)
	}
	if e.blockChargeAffordable(0, ch, nil, nil) {
		t.Fatal("block charge affordable from unadmitted restricted mana")
	}
	if d := askBlockersFresh(t, e); d != nil && findBlockOption(d, blocker, att) != nil {
		t.Fatalf("pair offered: %+v", d.Options)
	}
	if len(e.G.Obj(att).BlockedBy) != 0 {
		t.Fatalf("attacker blocked: %v", e.G.Obj(att).BlockedBy)
	}

	e, blocker, att, _ = blockBoard(t, "nonSpell", 0)
	if availTotal(e, 0, Cost{Generic: 2}) != 2 {
		t.Fatal("control precondition")
	}
	d := askBlockersFresh(t, e)
	if d == nil || findBlockOption(d, blocker, att) == nil {
		t.Fatal("control: admitted restricted mana did not offer the block")
	}
}

func TestBlockTaxSettlesOnlyAdmittedMana(t *testing.T) {
	t.Parallel()
	e, blocker, att, ms := blockBoard(t, "Static.Unlock,Spell.Room", 2)
	ch := e.blockPairCharge(blocker, att)
	if !e.blockChargeAffordable(0, ch, nil, nil) {
		t.Fatal("precondition: Mountains must afford the charge")
	}
	plan, ok := e.openCombatPayPlan(0, ch, map[state.ObjID]bool{blocker: true}, nil)
	if !ok || e.manaSatisfied(plan) {
		t.Fatalf("precondition: plan ok=%v satisfied=%v", ok, ok && e.manaSatisfied(plan))
	}
	d := askBlockersFresh(t, e)
	if d == nil {
		t.Fatal("no blockers decision")
	}
	opt := findBlockOption(d, blocker, att)
	if opt == nil {
		t.Fatalf("pair not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatal(err)
	}
	if len(e.G.Obj(att).BlockedBy) != 0 {
		t.Fatal("block committed before the tax was paid")
	}
	for round := 0; round < 2; round++ {
		w := e.Pending()
		if w == nil || w.Kind != decision.KChoose || len(w.Options) == 0 || w.Options[0].Kind != "block_mana" {
			t.Fatalf("round %d: expected block_mana window, got %+v", round, w)
		}
		if err := e.Submit(decision.Intent{Seq: w.Seq, Player: w.Player, Choices: []int{0}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.G.Obj(att).BlockedBy) != 1 {
		t.Fatalf("block not committed: %v", e.G.Obj(att).BlockedBy)
	}
	for _, m := range ms {
		if !e.G.Obj(m).Tapped {
			t.Fatalf("Mountain %d untapped", m)
		}
	}
	if e.G.Players[0].Pool[state.MR] != 2 || len(e.G.Players[0].RestrictedMana) != 1 {
		t.Fatalf("restricted mana spent: pool %v batches %v", e.G.Players[0].Pool, e.G.Players[0].RestrictedMana)
	}
}

func TestCombatPhyrexianColourBranchHidesUnadmittedRestrictedMana(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	onBoardCard(t, e, 0, card(t, phyTax))
	bear := onBoardReady(t, e, 1, restrBear)
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	e.G.Players[1].Life = 4
	seedRestricted(e, 1, state.MW, "W", 1, "Spell.Room")
	ch := e.attackPairCharge(bear, 0, 0)
	if len(ch.phyrexian) != 1 || availTotal(e, 1, Cost{Phyrexian: []byte{'W'}}) != 0 {
		t.Fatalf("precondition: charge %+v", ch)
	}
	m := map[state.ObjID]bool{bear: true}
	plan, ok := e.openCombatPayPlan(1, ch, m, m)
	if !ok {
		t.Fatal("precondition: the plan must open for the life branch")
	}
	if plan.phyDone != 1 || plan.phyLifePips() != 1 {
		t.Fatalf("plan done=%d life=%d, want the one-pip life route", plan.phyDone, plan.phyLifePips())
	}
	e.askAttackers()
	d := e.Pending()
	opt := findAttackOption(d, bear, 0)
	if opt == nil {
		t.Fatalf("pair not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatal(err)
	}
	if got := e.G.Players[1].Life; got != 2 {
		t.Fatalf("life = %d, want 2", got)
	}
	if e.G.Players[1].Pool[state.MW] != 1 {
		t.Fatal("restricted W was spent")
	}
	if o := e.G.Obj(bear); o == nil || !o.IsAttacking {
		t.Fatal("attacker not declared")
	}
}
