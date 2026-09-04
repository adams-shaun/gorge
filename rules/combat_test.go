package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// combatEngine builds a two-player engine (seats 0 and 1) with active/step
// left for the test to set, ready for onBoard placements. Reuses layerEngine's
// own construction (mountain decks, no creatures) since combat tests care
// only about the creatures they place themselves.
func combatEngine(t *testing.T) *Engine {
	t.Helper()
	e := layerEngine(t)
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e
}

// submitAttackers finds the KAttackers options naming each of ids and submits
// exactly those. Order does not matter for attackers (every one of them ends
// up sharing the same M1-fixed defender), unlike submitBlockers below.
func submitAttackers(t *testing.T, e *Engine, ids ...state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	want := make(map[state.ObjID]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var choices []int
	for _, o := range d.Options {
		if want[o.Obj] {
			choices = append(choices, o.Index)
		}
	}
	if len(choices) != len(ids) {
		t.Fatalf("only %d of %d requested attackers were offered: %+v", len(choices), len(ids), d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit attackers: %v", err)
	}
}

// submitBlockers finds, for each blockerID in the caller's own order, the
// KBlockers option that names it, and submits those indices in that same
// order. Order is significant here (unlike submitAttackers): it becomes the
// Pairs order in the DeclareBlockers event, which BlockedBy preserves, which
// is what dealCombatDamage's damage-assignment loop reads as blocker order.
func submitBlockers(t *testing.T, e *Engine, blockerIDs ...state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	var choices []int
	for _, bid := range blockerIDs {
		idx := -1
		for _, o := range d.Options {
			if o.Obj == bid {
				idx = o.Index
				break
			}
		}
		if idx < 0 {
			t.Fatalf("no block option for %d: %+v", bid, d.Options)
		}
		choices = append(choices, idx)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit blockers: %v", err)
	}
}

func TestUnblockedAttackerDamagesTheDefendingPlayer(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)

	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("defender life = %d, want 18", got)
	}
}

func TestSummoningSickCreatureCannotAttackWithoutHaste(t *testing.T) {
	e := combatEngine(t)
	sick := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Obj(sick).SummonSick = true

	e.askAttackers()
	if d := e.Pending(); d != nil {
		t.Fatalf("expected no attackers decision (no legal attacker), got %+v", d)
	}
	if e.canAttack(sick) {
		t.Fatal("a summoning-sick creature without Haste should not be able to attack")
	}

	// Reset combat and try again with a hasty creature: now there is exactly
	// one attacker option.
	e2 := combatEngine(t)
	hasty := onBoard(t, e2, 0, "Name:Raider\nManaCost:1 R\nTypes:Creature Goblin\nPT:2/2\nK:Haste\nOracle:x\n")
	e2.G.Obj(hasty).SummonSick = true

	e2.askAttackers()
	d := e2.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	if len(d.Options) != 1 {
		t.Fatalf("attacker options = %d, want 1 (Haste overrides summoning sickness)", len(d.Options))
	}
	if !e2.canAttack(hasty) {
		t.Fatal("a summoning-sick creature with Haste should be able to attack")
	}
}

func TestVigilanceAttackerStaysUntapped(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Sentinel\nManaCost:2 W\nTypes:Creature Soldier\nPT:2/2\nK:Vigilance\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)

	if e.G.Obj(atk).Tapped {
		t.Fatal("a Vigilance attacker should not tap when declared")
	}
}

func TestBlockedAttackerAndBlockerTradeDamage(t *testing.T) {
	// Toughness 4 on both sides, well above either creature's power, so the
	// exchange is observable afterward: events.Move clears Object.Damage the
	// instant something actually dies (see TestDeathtouchKillsRegardlessOf-
	// Toughness and friends for that case instead), so a trade that is meant
	// to be inspected for exact damage amounts must be one neither side dies
	// from.
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Ox\nManaCost:2 G\nTypes:Creature Ox\nPT:2/4\nOracle:x\n")
	blk := onBoard(t, e, 1, "Name:Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:1/4\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	if got := e.G.Obj(atk).Damage; got != 1 {
		t.Fatalf("attacker damage = %d, want 1 (the blocker's power)", got)
	}
	if got := e.G.Obj(blk).Damage; got != 2 {
		t.Fatalf("blocker damage = %d, want 2 (the attacker's power)", got)
	}
	if o := e.G.Obj(atk); o.Zone != state.ZBattlefield {
		t.Fatalf("attacker zone = %v, want battlefield (1 damage on a 2/4 is not lethal)", o.Zone)
	}
	if o := e.G.Obj(blk); o.Zone != state.ZBattlefield {
		t.Fatalf("blocker zone = %v, want battlefield (2 damage on a 1/4 is not lethal)", o.Zone)
	}
}

func TestFlyingCannotBeBlockedByGroundCreature(t *testing.T) {
	e := combatEngine(t)
	flier := onBoard(t, e, 0, "Name:Griffin\nManaCost:2 W\nTypes:Creature Griffin\nPT:2/2\nK:Flying\nOracle:x\n")
	ground := onBoard(t, e, 1, "Name:Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/2\nOracle:x\n")

	// Declare the attacker directly (state only, not through the decision
	// flow) so askBlockers can be exercised in isolation, without its own
	// result cascading straight through combat damage via Submit's Advance.
	fo := e.G.Obj(flier)
	fo.IsAttacking = true
	fo.Attacking = 1

	e.askBlockers()
	if d := e.Pending(); d != nil {
		t.Fatalf("a ground creature should not be offered as a blocker for a flier, got %+v", d)
	}
	if e.canBlock(ground, flier) {
		t.Fatal("canBlock should refuse a ground creature blocking a flier")
	}

	e2 := combatEngine(t)
	flier2 := onBoard(t, e2, 0, "Name:Griffin\nManaCost:2 W\nTypes:Creature Griffin\nPT:2/2\nK:Flying\nOracle:x\n")
	reach := onBoard(t, e2, 1, "Name:Archer\nManaCost:1 G\nTypes:Creature Elf\nPT:1/2\nK:Reach\nOracle:x\n")
	fo2 := e2.G.Obj(flier2)
	fo2.IsAttacking = true
	fo2.Attacking = 1

	e2.askBlockers()
	d := e2.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	if len(d.Options) != 1 {
		t.Fatalf("block options = %d, want 1 (the Reach creature)", len(d.Options))
	}
	if !e2.canBlock(reach, flier2) {
		t.Fatal("canBlock should allow a Reach creature blocking a flier")
	}
}

func TestDeathtouchKillsRegardlessOfToughness(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Ogre\nManaCost:4 R\nTypes:Creature Ogre\nPT:5/5\nOracle:x\n")
	blk := onBoard(t, e, 1, "Name:Adder\nManaCost:G\nTypes:Creature Snake\nPT:1/1\nK:Deathtouch\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	if o := e.G.Obj(atk); o.Zone != state.ZGraveyard {
		t.Fatalf("attacker zone = %v, want graveyard (a single deathtouch point should be lethal to a 5/5)", o.Zone)
	}
}

func TestTrampleAssignsExcessToThePlayer(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Rhino\nManaCost:3 G G\nTypes:Creature Rhino\nPT:5/5\nK:Trample\nOracle:x\n")
	blk := onBoard(t, e, 1, "Name:Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/2\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("defender life = %d, want 17 (20 - 3 trample excess)", got)
	}
	// The blocker took exactly its own toughness in damage (2, the minimum
	// Trample must assign before spilling over) and so died to state-based
	// actions -- which is also why its Damage cannot be asserted here
	// directly: events.Move clears it the instant the creature actually
	// leaves the battlefield.
	if o := e.G.Obj(blk); o.Zone != state.ZGraveyard {
		t.Fatalf("blocker zone = %v, want graveyard (assigned exactly lethal damage)", o.Zone)
	}
}

func TestLifelinkGainsLifeOnCombatDamage(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Cleric\nManaCost:2 W\nTypes:Creature Cleric\nPT:3/3\nK:Lifelink\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)

	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("attacker controller life = %d, want 23 (20 + 3 lifelink)", got)
	}
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("defender life = %d, want 17", got)
	}
}

func TestFirstStrikeKillsBeforeTheNormalDamageStep(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Duelist\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/1\nK:First Strike\nOracle:x\n")
	blk := onBoard(t, e, 1, "Name:Peasant\nManaCost:B\nTypes:Creature Human\nPT:1/1\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	if o := e.G.Obj(blk); o.Zone != state.ZGraveyard {
		t.Fatalf("blocker zone = %v, want graveyard (killed in the first-strike step)", o.Zone)
	}
	if got := e.G.Obj(atk).Damage; got != 0 {
		t.Fatalf("first striker damage = %d, want 0 (the blocker died before dealing any)", got)
	}
	if o := e.G.Obj(atk); o.Zone != state.ZBattlefield {
		t.Fatalf("first striker zone = %v, want battlefield (it took no damage back)", o.Zone)
	}
}

func TestMultipleBlockersEachTakeDamage(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Rhino\nManaCost:3 G G\nTypes:Creature Rhino\nPT:3/5\nK:Trample\nOracle:x\n")
	first := onBoard(t, e, 1, "Name:First Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/2\nOracle:x\n")
	second := onBoard(t, e, 1, "Name:Second Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/2\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)
	// Declared in this exact order: the attacker's 3 power assigns 2 (lethal)
	// to the first-declared blocker before the second-declared blocker sees
	// any of the remainder.
	submitBlockers(t, e, first, second)

	// The first-declared blocker was assigned its full lethal need (2) before
	// the second saw any of the remainder, and died to state-based actions --
	// which is also why only the survivor's exact Damage is asserted directly:
	// events.Move clears the field the instant something actually dies.
	if got := e.G.Obj(second).Damage; got != 1 {
		t.Fatalf("second-declared blocker damage = %d, want 1 (only the leftover)", got)
	}
	if o := e.G.Obj(first); o.Zone != state.ZGraveyard {
		t.Fatalf("first-declared blocker zone = %v, want graveyard (assigned exactly lethal damage)", o.Zone)
	}
	if o := e.G.Obj(second); o.Zone != state.ZBattlefield {
		t.Fatalf("second-declared blocker zone = %v, want battlefield (only 1 damage, not lethal)", o.Zone)
	}
}

func TestCombatDamageUsesDerivedPowerNotPrinted(t *testing.T) {
	e := combatEngine(t)
	atk := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: atk, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", Controller: 0, AddPower: 3, AddToughness: 3})

	e.askAttackers()
	submitAttackers(t, e, atk)

	if got := e.G.Players[1].Life; got != 15 {
		t.Fatalf("defender life = %d, want 15 (20 - 5 derived power, not the printed 2)", got)
	}
}

// TestCleanupClearsCombatDamageDeathtouchAndUntilEOTEffects is the regression
// test for a gap outside this task's own 11 named tests but explicitly in its
// scope: EndOfTurnCleanup (layers.go, built and tested since Task 19c) was
// never called from anywhere in the engine, so a resolved pump effect used to
// survive forever instead of expiring at the end of its own turn -- and
// nothing cleared marked damage or the Deathtouched counter this task
// introduces either. This drives the real cleanup path (priorityRound, not a
// direct EndOfTurnCleanup call) so the wiring itself is under test, not just
// the primitive layers_test.go already covers in isolation.
func TestCleanupClearsCombatDamageDeathtouchAndUntilEOTEffects(t *testing.T) {
	e := combatEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	o := e.G.Obj(id)
	o.Damage = 1
	o.AddCounter("Deathtouched", 1)
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", Controller: 0, AddPower: 3, AddToughness: 3, UntilEOT: true})

	if got := e.Power(id); got != 5 {
		t.Fatalf("power before cleanup = %d, want 5 (2 printed + 3 until-end-of-turn)", got)
	}

	e.G.Step = state.StepCleanup
	e.priorityRound()

	if o.Damage != 0 {
		t.Fatalf("damage after cleanup = %d, want 0", o.Damage)
	}
	if o.Counter("Deathtouched") != 0 {
		t.Fatalf("Deathtouched after cleanup = %d, want 0", o.Counter("Deathtouched"))
	}
	if got := e.Power(id); got != 2 {
		t.Fatalf("power after cleanup = %d, want 2 (the until-end-of-turn effect should have expired)", got)
	}
}
