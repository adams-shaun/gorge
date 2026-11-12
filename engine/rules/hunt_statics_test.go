package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// hunt_statics_test.go pins the three Commander statics hunt-cmdr-statics adds:
// CantPayLife (Angel of Jubilation), CanAttackIfHaste (Instill Energy) and
// AttackRequirement (War's Toll).

func huntTwoSeat(t *testing.T) *Engine {
	t.Helper()
	return New(Config{Seed: 5, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
}

// TestCantPayLifeBlocksCostLife: the real Angel of Jubilation forbids either
// seat a Spell/Activated life payment; a payment of any other kind (ward,
// upkeep, unless) still pays; a game with no Angel pays (the control).
func TestCantPayLifeBlocksCostLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	angel := mustCorpusCard(t, reg, "Angel of Jubilation")
	greed := mustCorpusCard(t, reg, "Greed")
	if !effects.Supported()["stat:CantPayLife"] {
		t.Fatal("precondition: stat:CantPayLife is not a registered primitive")
	}
	e := huntTwoSeat(t)
	onBoardCard(t, e, 0, angel)
	greedID := onBoardCard(t, e, 1, greed)
	if len(e.activeStatics("CantPayLife")) == 0 {
		t.Fatal("precondition: the CantPayLife static is not active")
	}
	ctl := huntTwoSeat(t)
	ctlGreed := onBoardCard(t, ctl, 1, greed)
	if len(ctl.activeStatics("CantPayLife")) != 0 {
		t.Fatal("control precondition: the control game has a CantPayLife static")
	}
	for _, kind := range []pay.SAKind{pay.SASpell, pay.SAActivated} {
		for _, p := range []state.PlayerID{0, 1} {
			if !playerLifePayForbidden(e, p, kind) {
				t.Fatalf("seat %d kind %d: life payment not forbidden under Angel", p, kind)
			}
		}
	}
	lifeCost := pay.Cost{Life: 2}
	pool := func(g *Engine) (state.Mana, [7]state.Mana) {
		return g.G.Players[1].Pool, g.G.Players[1].ManaUnits()
	}
	p1, t1 := pool(e)
	if pay.CostPayablePool(asPayer(e), 1, greedID, pay.SAActivated, lifeCost, p1, t1) {
		t.Fatal("CostPayablePool priced a Greed-style 2-life activation under Angel")
	}
	if pay.CostPayablePool(asPayer(e), 1, greedID, pay.SASpell, lifeCost, p1, t1) {
		t.Fatal("CostPayablePool priced a 2-life spell cost under Angel")
	}
	if pay.CostPayableClass(asPayer(e), 1, pay.DescriptorFor(greedID, true, lifeCost), pay.PipRider{}, lifeCost) {
		t.Fatal("CostPayableClass priced a Greed-style 2-life activation under Angel")
	}
	if !pay.CostPayablePool(asPayer(e), 1, greedID, pay.SAOther, lifeCost, p1, t1) {
		t.Fatal("CostPayablePool refused a non-Spell/Activated 2-life payment under Angel")
	}
	if !pay.CostPayablePool(asPayer(e), 1, greedID, pay.SAActivated, pay.Cost{}, p1, t1) {
		t.Fatal("a zero-life activation must stay payable (CR 119.4b)")
	}
	c1, ct1 := pool(ctl)
	if !pay.CostPayablePool(asPayer(ctl), 1, ctlGreed, pay.SASpell, lifeCost, c1, ct1) ||
		!pay.CostPayablePool(asPayer(ctl), 1, ctlGreed, pay.SAActivated, lifeCost, c1, ct1) {
		t.Fatal("control: the 2-life payment is refused with no Angel on the board")
	}
	if !pay.CostPayableClass(asPayer(ctl), 1, pay.DescriptorFor(ctlGreed, true, lifeCost), pay.PipRider{}, lifeCost) {
		t.Fatal("control: CostPayableClass refused the 2-life activation with no Angel")
	}
}

// TestCantPayLifeCauseScoping: the kernel read on the Angel/Yasharn shape.
func TestCantPayLifeCauseScoping(t *testing.T) {
	e := huntTwoSeat(t)
	onBoardCard(t, e, 0, card(t, "Name:Gate Cant Pay\nTypes:Enchantment\n"+
		"S:Mode$ CantPayLife | ValidPlayer$ Player | ValidCause$ Spell,Activated | ForCost$ True\nOracle:x\n"))
	if len(e.activeStatics("CantPayLife")) == 0 {
		t.Fatal("precondition: the fixture static is not active")
	}
	if !playerLifePayForbidden(e, 0, pay.SASpell) || !playerLifePayForbidden(e, 0, pay.SAActivated) {
		t.Fatal("Spell/Activated life payment not forbidden")
	}
	if playerLifePayForbidden(e, 0, pay.SAOther) {
		t.Fatal("a ward/upkeep/unless payment (SAOther) must stay payable")
	}
	// ForCost$ False is the effect path: inert on the cost path.
	e2 := huntTwoSeat(t)
	onBoardCard(t, e2, 0, card(t, "Name:Gate Effect Only\nTypes:Enchantment\n"+
		"S:Mode$ CantPayLife | ValidPlayer$ Player | ValidCause$ Spell,Activated | ForCost$ False\nOracle:x\n"))
	if len(e2.activeStatics("CantPayLife")) == 0 || playerLifePayForbidden(e2, 0, pay.SASpell) {
		t.Fatal("a ForCost$ False line must be inert on the cost path")
	}
}

const huntSickBear = "Name:Hunt Sick Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// huntSickBearOn puts a summoning-sick bear on seat p's battlefield and makes
// p the active player, so combat's attack reads see it.
func huntSickBearOn(t *testing.T, e *Engine, p state.PlayerID) state.ObjID {
	t.Helper()
	id := onBoardCard(t, e, p, card(t, huntSickBear))
	e.G.Active = p
	if o := e.G.Obj(id); !o.SummonSick || o.Controller != p {
		t.Fatalf("precondition: the bear is not a summoning-sick seat-%d permanent: %+v", p, o)
	}
	return id
}

// TestCanAttackIfHastePair: the per-pair permission over the real board
// adapter, with a control (the same board without the static) for each.
func TestCanAttackIfHastePair(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	auraSrc := "Name:Hunt Haste Aura\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ CanAttackIfHaste | ValidCard$ Creature.EnchantedBy\nOracle:x\n"
	t.Run("enchanted", func(t *testing.T) {
		aura := card(t, auraSrc)
		e := corpusEngine(t, reg, []*cards.Card{aura}, nil)
		bear := huntSickBearOn(t, e, 0)
		if combat.CanAttackPair(asBoard(e), bear, 1) {
			t.Fatal("control: a sick bear attacks with no static")
		}
		attachCorpusAura(t, e, 0, aura, bear)
		if len(e.activeStatics("CanAttackIfHaste")) == 0 {
			t.Fatal("precondition: the CanAttackIfHaste static is not active")
		}
		if !combat.CanAttackPair(asBoard(e), bear, 1) || !combat.CanAttack(asBoard(e), bear) {
			t.Fatal("the enchanted sick bear cannot attack")
		}
		other := onBoardCard(t, e, 0, card(t, huntSickBear))
		if combat.CanAttackPair(asBoard(e), other, 1) {
			t.Fatal("the unenchanted sick bear attacks")
		}
	})
	t.Run("opponent-scoped", func(t *testing.T) {
		e := huntTwoSeat(t)
		bear := huntSickBearOn(t, e, 0)
		if combat.CanAttackPair(asBoard(e), bear, 1) {
			t.Fatal("control: a sick bear attacks with no static")
		}
		onBoardCard(t, e, 0, card(t, "Name:Hunt Saddle\nTypes:Enchantment\n"+
			"S:Mode$ CanAttackIfHaste | ValidTarget$ Opponent\nOracle:x\n"))
		if !combat.CanAttackPair(asBoard(e), bear, 1) {
			t.Fatal("an opponent-scoped permission does not reach seat 1")
		}
	})
	t.Run("not-the-controllers-opponent", func(t *testing.T) {
		e := huntTwoSeat(t)
		onBoardCard(t, e, 0, card(t, "Name:Hunt Saddle\nTypes:Enchantment\n"+
			"S:Mode$ CanAttackIfHaste | ValidTarget$ Opponent\nOracle:x\n"))
		bear := huntSickBearOn(t, e, 1)
		if len(e.activeStatics("CanAttackIfHaste")) == 0 {
			t.Fatal("precondition: the CanAttackIfHaste static is not active")
		}
		if combat.CanAttackPair(asBoard(e), bear, 0) {
			t.Fatal("seat 0's Opponent-scoped permission let seat 1's bear attack seat 0")
		}
	})
}

// TestInstillEnergyHastesEnchantedCreature is the filing card: the real
// Instill Energy on a summoning-sick bear puts the bear's attack in the
// KAttackers offer list; without the Aura the bear is not offered.
func TestInstillEnergyHastesEnchantedCreature(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	instill := mustCorpusCard(t, reg, "Instill Energy")
	offered := func(e *Engine, bear state.ObjID) bool {
		e.G.Step = state.StepDeclareAttackers
		e.pending = nil
		e.askAttackers()
		d := e.Pending()
		if d == nil {
			return false
		}
		if d.Kind != decision.KAttackers {
			t.Fatalf("expected an attackers decision, got %v", d.Kind)
		}
		for _, o := range d.Options {
			if o.Obj == bear {
				return true
			}
		}
		return false
	}
	ctl := corpusEngine(t, reg, []*cards.Card{instill}, nil)
	ctlBear := huntSickBearOn(t, ctl, 0)
	if combat.CanAttack(asBoard(ctl), ctlBear) || offered(ctl, ctlBear) {
		t.Fatal("control: the sick bear is offered an attack with no Instill Energy")
	}
	e := corpusEngine(t, reg, []*cards.Card{instill}, nil)
	bear := huntSickBearOn(t, e, 0)
	attachCorpusAura(t, e, 0, instill, bear)
	if o := e.G.Obj(bear); !o.SummonSick {
		t.Fatal("precondition: the bear is no longer summoning sick")
	}
	if !combat.CanAttackPair(asBoard(e), bear, 1) || !combat.CanAttack(asBoard(e), bear) {
		t.Fatal("Instill Energy: the enchanted sick bear cannot attack")
	}
	if !offered(e, bear) {
		t.Fatal("Instill Energy: the enchanted sick bear is not in the KAttackers offer list")
	}
}

// warsTollBoard is seat 0's real War's Toll against seat 1's three creatures:
// two able ones and one tapped one (not "able", CR 508.1d, so exempt). Seat 1
// is active at declare attackers with the KAttackers decision pending.
func warsTollBoard(t *testing.T) (*Engine, []state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := huntTwoSeat(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "War's Toll"))
	var able []state.ObjID
	for range 2 {
		id := onBoardCard(t, e, 1, card(t, huntSickBear))
		e.G.Obj(id).SummonSick = false
		able = append(able, id)
	}
	tapped := onBoardCard(t, e, 1, card(t, huntSickBear))
	e.G.Obj(tapped).SummonSick = false
	e.G.Obj(tapped).Tapped = true
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	if len(e.activeStatics("AttackRequirement")) == 0 {
		t.Fatal("precondition: the AttackRequirement static is not active")
	}
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	if len(d.AttackRequirements) == 0 {
		t.Fatal("precondition: the decision publishes no AttackRequirement closure")
	}
	for _, o := range d.Options {
		if o.Obj == tapped {
			t.Fatalf("precondition: the tapped creature is offered: %+v", o)
		}
	}
	return e, able, tapped
}

// TestAttackRequirementWholeDeclaration: under War's Toll one able attacker
// alone is rejected, all able attackers are accepted and so is the empty
// declaration.
func TestAttackRequirementWholeDeclaration(t *testing.T) {
	e, able, _ := warsTollBoard(t)
	d := e.Pending()
	one := []int{indexOfObjOption(d, able[0])}
	both := []int{indexOfObjOption(d, able[0]), indexOfObjOption(d, able[1])}
	if one[0] < 0 || both[1] < 0 {
		t.Fatalf("precondition: able creatures not offered: %+v", d.Options)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: one}); err == nil {
		t.Fatal("Validate accepted one able attacker under War's Toll")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: one}); err == nil {
		t.Fatal("the engine accepted one able attacker under War's Toll")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: both}); err != nil {
		t.Fatalf("both able attackers rejected: %v", err)
	}
	for _, id := range able {
		if !e.G.Obj(id).IsAttacking {
			t.Fatalf("creature %d is not attacking", id)
		}
	}
	e2, _, _ := warsTollBoard(t)
	d2 := e2.Pending()
	if err := e2.Submit(decision.Intent{Seq: d2.Seq, Player: d2.Player, Choices: nil}); err != nil {
		t.Fatalf("the empty declaration was rejected: %v", err)
	}
}

// TestAttackRequirementBotAnswerPassesValidator: the one-home test -- the
// real bot's answers, run through the real validator, never livelock, and the
// board ends with zero or all able members attacking.
func TestAttackRequirementBotAnswerPassesValidator(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		e, able, _ := warsTollBoard(t)
		bot := newTestBot(seed)
		for i := 0; i < 16; i++ {
			d := e.Pending()
			if d == nil || e.G.Step != state.StepDeclareAttackers || d.Kind == decision.KPriority {
				break
			}
			in := bot.answer(e, d)
			if err := e.Submit(in); err != nil {
				t.Fatalf("seed %d: the bot's own %v answer %v was rejected: %v", seed, d.Kind, in.Choices, err)
			}
		}
		n := 0
		for _, id := range able {
			if e.G.Obj(id).IsAttacking {
				n++
			}
		}
		if n != 0 && n != len(able) {
			t.Fatalf("seed %d: %d of %d able creatures attack, want none or all", seed, n, len(able))
		}
	}
}

// TestAttackRequirementRepairCompletesClause: decision.FitRequired -- the repair
// the bot's Clamp calls -- completes a partial answer to the full declaration.
func TestAttackRequirementRepairCompletesClause(t *testing.T) {
	e, able, _ := warsTollBoard(t)
	d := e.Pending()
	got := d.FitRequired([]int{indexOfObjOption(d, able[0])})
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: got}); err != nil {
		t.Fatalf("FitRequired returned %v which Validate rejects: %v", got, err)
	}
	if len(got) != 2 {
		t.Fatalf("FitRequired = %v, want both able creatures", got)
	}
}

// TestHuntStaticsCorpusCardsCompile: the three real cards report no unsupported
// primitive.
func TestHuntStaticsCorpusCardsCompile(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Angel of Jubilation", "Instill Energy", "War's Toll"} {
		c := mustCorpusCard(t, reg, name)
		if u := reg.Unsupported(c, effects.Supported()); len(u) != 0 {
			t.Errorf("%s: Unsupported = %v", name, u)
		}
	}
}
