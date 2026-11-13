package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// The narrowing of the shared combatChargeAffordable read.
//
// A resolved zero is a free charge, not an unpriceable cost. On main a
// Count$/SVar combat price that resolved to 0 was skipped as a free pair; the
// fail-closed round initially classified it `unpriceable`, so the creature
// could not attack or block AT ALL -- a real regression on corpus cards whose
// counted resource is legitimately empty (Myr Prototype at 0 counters,
// Collective Restraint with 0 land types, Cowed by Wisdom with an empty hand).
//
// A multi-pip charge payable ONLY by mixing branches (one pip with colour, one
// with life) is admitted by the joint Cost{Generic, Phyrexian} reachability
// read and is now settleable, because the plan routes and asks per pip
// (CR 107.4f); the window collects the colour pip's source and charges two
// life for the life pip.
//
// These tests pin both directions at the charge resolver and end to end
// through the offer list, each asserting its own precondition.

// TestCombatPropResolvedZeroCountIsFree pins the resolver: a Count$ body that
// resolves to 0 with a valid SVar table is a priceable FREE charge, never
// `unpriceable`.
func TestCombatPropResolvedZeroCountIsFree(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	source := onBoardReady(t, e, 0, "Name:Count source\nTypes:Creature\nPT:1/1\nOracle:x\n")
	sv := staticView{
		Source:     source,
		Controller: 0,
		Params:     map[string]string{"Cost": "Count$CardCounters.P1P1"},
		SVars:      map[string]string{},
	}
	attack, ok := e.attackUnlessCharge(sv, 0)
	if !ok || !attack.zero() || attack.unpriceable {
		t.Fatalf("precondition/result: resolved zero-counter attack count = %+v, ok=%v; want a priceable free charge", attack, ok)
	}
	block, ok := e.blockUnlessCharge(sv)
	if !ok || !block.zero() || block.unpriceable {
		t.Fatalf("precondition/result: resolved zero-counter block count = %+v, ok=%v; want a priceable free charge", block, ok)
	}
	// PRECONDITION: the source the count reads really is on the battlefield
	// and really has zero counters, so the zero is a resolved zero and not a
	// missing object.
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield || len(o.Counters) != 0 {
		t.Fatal("precondition: zero-counter count source is not on the battlefield")
	}
}

// TestMyrPrototypeZeroCountersAttacksFree is the corpus-shaped half the
// review's finding asked for: Myr Prototype (`Cost$ Y`, `SVar:Y:
// Count$CardCounters.P1P1`) has no +1/+1 counters the turn it lands, so its
// self-charge resolves to a free price and the creature MUST still be offered
// as an attacker. Before the resolved-zero fix the pair was unpriceable and
// disappeared from the offer list entirely -- the card could never attack.
func TestMyrPrototypeZeroCountersAttacksFree(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	myr := onBoardReadyCard(t, e, 0, corpusAlternativeCard(t, "Myr Prototype"))
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers

	// PRECONDITION: Myr Prototype is seat 0's battlefield creature with zero
	// counters, the charge it prices really reads those counters, and the
	// defender is seat 1 (the absent Target$ scopes every defender).
	o := e.G.Obj(myr)
	if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 || len(o.Counters) != 0 {
		t.Fatalf("precondition: Myr Prototype = %+v, want a 0-counter creature controlled by seat 0", o)
	}
	ch := e.attackPairCharge(myr, 1, 0)
	if ch.unpriceable || !ch.zero() {
		t.Fatalf("precondition: Myr Prototype charge = %+v, want priceable/zero (the resolved-zero fix is not in force)", ch)
	}
	if !e.combatChargeAffordable(0, ch, map[state.ObjID]bool{myr: true}, map[state.ObjID]bool{myr: true}) {
		t.Fatal("precondition: the resolved-zero charge reads unaffordable")
	}

	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	opt := findAttackOption(d, myr, 1)
	if opt == nil {
		t.Fatalf("0-counter Myr Prototype cannot attack at all: %+v", d.Options)
	}
	if opt.Value != 0 {
		t.Fatalf("the resolved-zero charge was priced at Value %d, want 0", opt.Value)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit the resolved-zero attack: %v", err)
	}
	if o := e.G.Obj(myr); o == nil || !o.IsAttacking {
		t.Fatal("the resolved-zero attacker was never declared")
	}
	drainCombatPriority(t, e)
}

// combatMultiPipFixture charges two Phyrexian pips of different colours: the
// shape whose MIXED branch affordability (one pip with colour, one with life)
// the old all-or-nothing election could not settle and now can.
const combatMultiPipFixture = "Name:Multi Pip Tax\nTypes:Enchantment\n" +
	"S:Mode$ CantAttackUnless | ValidCard$ Creature | Target$ You | Cost$ W/P U/P | Description$x\n" +
	"Oracle:x\n"

// TestMixedMultiPipChargeIsOffered pins the per-pip CR 107.4f consequence:
// a two-pip charge payable ONLY by mixing branches (one colour, one life) is
// settleable per pip now, so it must be affordable and offered, and its
// payment window must collect the colour pip's source. The all-life control
// stays affordable too.
func TestMixedMultiPipChargeIsOffered(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	onBoardCard(t, e, 0, card(t, combatMultiPipFixture))
	bear := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	plains := onBoardCard(t, e, 1, card(t, "Name:Test Plains\nTypes:Basic Land Plains\nOracle:x\n"))
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	e.G.Players[1].Life = 3

	ch := e.attackPairCharge(bear, 0, 0)
	if len(ch.phyrexian) != 2 || ch.phyrexian[0] != 'W' || ch.phyrexian[1] != 'U' || ch.mana != 0 {
		t.Fatalf("precondition: charge = %+v, want two pips W then U", ch)
	}

	// MIXED-ONLY: one white source (cannot pay both pips with colour) and
	// three life (cannot pay both pips with life). The joint cost read admits
	// the mix, and the per-pip plan can settle it as W + 2 life.
	if !pay.UnlessManaReachable(asPayer(e), 1, chCost(ch), e.G.Players[1].Pool, e.G.Players[1].Snow,
		e.G.Players[1].ManaUnits(), e.G.Players[1].Life, asEval(e).Conv(1, 0, pay.SAOther),
		e.attackWindowUnits(1, nil)) {
		t.Fatal("precondition: the MIXED branch is payable, but the joint read says no (test is vacuous)")
	}
	committed := map[state.ObjID]bool{bear: true}
	if !e.combatChargeAffordable(1, ch, committed, committed) {
		t.Fatal("the mixed W + 2 life payment must be affordable")
	}
	// ALL-LIFE control: four life covers both pips.
	e.G.Players[1].Life = 4
	if !e.combatChargeAffordable(1, ch, committed, committed) {
		t.Fatal("precondition: an all-life multi-pip charge must read affordable with 4 life")
	}
	e.G.Players[1].Life = 3

	e.askAttackers()
	d := e.Pending()
	opt := findAttackOption(d, bear, 0)
	if opt == nil {
		t.Fatalf("mixed-payable multi-pip pair not offered: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
		t.Fatalf("submit the mixed-payable multi-pip attack: %v", err)
	}

	// The W pip is routed to colour and the U pip to life, so the window asks
	// for the one white mana the colour face needs.
	mana := e.Pending()
	if mana == nil || mana.Kind != decision.KChoose {
		t.Fatalf("expected the mana-source ask for the colour pip, got %+v", mana)
	}
	manaIdx := -1
	for _, o := range mana.Options {
		if o.Kind == "attack_mana" {
			manaIdx = o.Index
			break
		}
	}
	if manaIdx < 0 {
		t.Fatalf("mana-source ask has no attack_mana option: %+v", mana.Options)
	}
	if err := e.Submit(decision.Intent{Seq: mana.Seq, Player: mana.Player, Choices: []int{manaIdx}}); err != nil {
		t.Fatalf("tap the Plains for the colour pip: %v", err)
	}
	if !e.G.Obj(plains).Tapped {
		t.Fatal("the colour pip's Plains was never tapped")
	}
	if got := e.G.Players[1].Life; got != 1 {
		t.Fatalf("life after the mixed multi-pip payment = %d, want 1 (one pip with 2 life)", got)
	}
	if o := e.G.Obj(bear); o == nil || !o.IsAttacking {
		t.Fatal("the mixed-payable attacker was never declared")
	}
	drainCombatPriority(t, e)
}

// chCost is the joint mana requirement combatChargeAffordable's reachability
// call prices: the generic plus every pip (each colour OR two life).
func chCost(c blockCharge) Cost {
	out := Cost{Generic: c.mana, Phyrexian: append([]byte(nil), c.phyrexian...)}
	return out
}

// combatTapPipFixture charges `tapXType<1/Creature> W/P`: one creature tap
// obligation plus a Phyrexian pip. The tap obligation and the pip's colour
// branch compete for the same creature when it is the payer's only white
// source.
const combatTapPipFixture = "Name:Tap Pip Tax\nTypes:Enchantment\n" +
	"S:Mode$ CantAttackUnless | ValidCard$ Creature | Target$ You | Cost$ tapXType<1/Creature> W/P | Description$x\n" +
	"Oracle:x\n"

// TestPhyrexianColourBranchExcludesReservedTapSource: a
// plan-reserved tap permanent must be withheld from the Phyrexian colour
// branch probe too, not only from combatChargeAffordable's reachability. When
// the payer's only white source IS the creature the tap obligation must tap,
// the colour branch is NOT reachable through the plan; only the life branch
// is. Reading the colour branch as reachable would offer a CR 107.4f election
// whose colour half the window cannot settle, aborting an otherwise payable
// declaration.
func TestPhyrexianColourBranchExcludesReservedTapSource(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	onBoardCard(t, e, 0, card(t, combatTapPipFixture))
	bear := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	dork := onBoardReady(t, e, 1, "Name:White Dork\nManaCost:1 W\nTypes:Creature Elf\nPT:1/1\n"+
		"A:AB$ Mana | Cost$ T | Produced$ W | Oracle:x\n")
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	e.G.Players[1].Life = 2

	ch := e.attackPairCharge(bear, 0, 0)
	if len(ch.taps) != 1 || len(ch.phyrexian) != 1 || ch.phyrexian[0] != 'W' {
		t.Fatalf("precondition: charge = %+v, want one tap obligation + one W pip", ch)
	}
	// PRECONDITION: the plan really reserves the dork for the tap obligation,
	// and the dork really is a window mana source, so the exclusion matters.
	taps, _, _, ok := e.chargeObjPlan(1, ch, map[state.ObjID]bool{bear: true})
	if !ok || len(taps) != 1 || taps[0] != dork {
		t.Fatalf("precondition: plan taps = %v (ok=%v), want exactly the white dork %d reserved", taps, ok, dork)
	}
	found := false
	for _, s := range e.attackWindowUnits(1, nil) {
		if s.ID == dork {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: the dork is not a window mana source, so the exclusion test is vacuous")
	}

	// WITHOUT the caller's exclusion the colour branch reads reachable (the
	// dork pays the pip); WITH it the colour branch cannot be reached and only
	// the life branch remains, so the plan must settle deterministically on
	// life (all pips routed, none still owed) rather than pose a CR 107.4f
	// election whose colour half the window cannot settle.
	committed := map[state.ObjID]bool{bear: true}
	unexcluded, ok := e.openCombatPayPlan(1, ch, nil, nil)
	if !ok || unexcluded.phyDone != 0 {
		t.Fatalf("precondition: unexcluded plan must still owe the pip (ok=%v)", ok)
	}
	if colour, life := e.combatPhyPipBranches(unexcluded, nil); !colour || !life {
		t.Fatalf("precondition: unexcluded colour=%v life=%v, want both reachable", colour, life)
	}
	if !e.combatChargeAffordable(1, ch, committed, committed) {
		t.Fatal("precondition: the charge must still be affordable through the life branch")
	}
	plan, ok := e.openCombatPayPlan(1, ch, committed, committed)
	if !ok {
		t.Fatal("precondition: the plan must open for the life branch")
	}
	if plan.phyDone != int32(len(ch.phyrexian)) {
		t.Fatal("the plan still owes a Phyrexian election whose colour half is unreachable through the reserved tap source")
	}
	if plan.phyLifePips() != int32(len(ch.phyrexian)) {
		t.Fatalf("plan routed %d pips to life, want all %d (the colour branch is unreachable)", plan.phyLifePips(), len(ch.phyrexian))
	}
}
