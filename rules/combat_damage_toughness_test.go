package rules

// toughtdmg1: stat:CombatDamageToughness (CR 510.1, "assigns combat damage
// equal to its toughness rather than its power"). The static is collected
// through activeStatics (rules/statics.go combatDamageToughnessMatches) and
// the assignment amount is read through the ONE helper
// rules/statics.go combatDamageAmount, so the attacker's own assignment, each
// blocker's hit-back, the division option enumeration and the as-unblocked
// election gate cannot disagree.
//
// Every leaf pins a REAL corpus card (the same convention
// rules/combat_as_unblocked_test.go keeps) and drives the ordinary
// declare/submit paths, so the read lands in real combat rather than a
// direct-call fixture.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// toughnessBeast is a creature whose power and toughness differ sharply, so a
// misread of the assignment source is visible: 5 power, 2 toughness. With a
// CombatDamageToughness static it must assign 2, not 5.
const toughnessBeast = "Name:Combat Beast\nManaCost:3 G\nTypes:Creature Beast\nPT:5/2\nOracle:x\n"

func TestAssaultFormationAttackerDealsToughnessInsteadOfPower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := combatEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Assault Formation"))
	atk := onBoardReady(t, e, 0, toughnessBeast)
	// A 0/8 blocker survives the 2 damage and hits back for nothing, so both
	// creatures are still on the battlefield when the assertions read them.
	blk := onBoard(t, e, 1, "Name:Wall\nManaCost:1 W\nTypes:Creature Wall\nPT:0/8\nOracle:x\n")

	// Precondition: the two characteristics really differ, so this test can
	// distinguish the reads.
	if p, to := e.Power(atk), e.Toughness(atk); p == to || p != 5 || to != 2 {
		t.Fatalf("precondition: attacker P/T = %d/%d, want 5/2 (distinct)", p, to)
	}

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	if e.G.Obj(atk).Zone != state.ZBattlefield {
		t.Fatalf("precondition: the attacker must survive to be read (zone %v)", e.G.Obj(atk).Zone)
	}
	if got := e.G.Obj(blk).Damage; got != 2 {
		t.Fatalf("blocker damage = %d, want 2 (the attacker's TOUGHNESS, not its power 5)", got)
	}
	if got := e.G.Obj(atk).Damage; got != 0 {
		t.Fatalf("attacker damage = %d, want 0 (the 0-power blocker hit back for nothing)", got)
	}
}

func TestAssaultFormationBlockerHitsBackWithToughness(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := combatEngine(t)
	// The static sits on seat 1, so it is seat 1's BLOCKER whose hit-back is
	// toughness-based.
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Assault Formation"))
	atk := onBoardReady(t, e, 0, "Name:Big\nManaCost:4 R\nTypes:Creature Giant\nPT:6/6\nOracle:x\n")
	blk := onBoard(t, e, 1, toughnessBeast)

	if p, to := e.Power(blk), e.Toughness(blk); p == to || p != 5 || to != 2 {
		t.Fatalf("precondition: blocker P/T = %d/%d, want 5/2 (distinct)", p, to)
	}

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	// The 6/6 attacker survives either hit-back amount, so its marked damage
	// distinguishes the blocker's TOUGHNESS 2 from its power 5.
	if got := e.G.Obj(atk).Damage; got != 2 {
		t.Fatalf("attacker damage = %d, want 2 (the blocker's TOUGHNESS, not its power 5)", got)
	}
	// The attacker has no static of its own: the 5/2 blocker takes its power
	// 6 and dies (so its own marked damage is not readable).
	if e.G.Obj(blk).Zone == state.ZBattlefield {
		t.Fatalf("blocker should be dead to the 6-power attacker, still on battlefield")
	}
}

func TestAssaultFormationUnblockedAttackerDealsToughnessToPlayer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := combatEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Assault Formation"))
	atk := onBoardReady(t, e, 0, toughnessBeast)

	if p, to := e.Power(atk), e.Toughness(atk); p == to || p != 5 || to != 2 {
		t.Fatalf("precondition: attacker P/T = %d/%d, want 5/2 (distinct)", p, to)
	}

	e.askAttackers()
	submitAttackers(t, e, atk)
	drainCombatDamagePriority(t, e)

	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("defending player life = %d, want 18 (20 - the attacker's TOUGHNESS 2, not its power 5)", got)
	}
}

func TestAssaultFormationReadsBoostedToughnessThroughTheLayerWalk(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := combatEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Assault Formation"))
	// Spidersilk Armor is a +0/+1 static, so the creature's EFFECTIVE
	// toughness is 3 while its power stays 5: the layer-derived read must
	// assign 3, which neither the printed toughness (2) nor the power (5)
	// equals.
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Spidersilk Armor"))
	atk := onBoardReady(t, e, 0, toughnessBeast)
	blk := onBoard(t, e, 1, "Name:Wall\nManaCost:1 W\nTypes:Creature Wall\nPT:2/8\nOracle:x\n")

	if p, to := e.Power(atk), e.Toughness(atk); p != 5 || to != 3 {
		t.Fatalf("precondition: attacker derived P/T = %d/%d, want 5/3 (2 base +1 from Spidersilk Armor)", p, to)
	}

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	if got := e.G.Obj(blk).Damage; got != 3 {
		t.Fatalf("blocker damage = %d, want 3 (the layer-derived toughness, not printed 2 nor power 5)", got)
	}
}

func TestBedrockTortoisePowerLTtoughnessFilterScopesTheStatic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := combatEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Bedrock Tortoise"))
	// 3/6: toughness > power, so the powerLTtoughness static applies -> 6.
	big := onBoardReady(t, e, 0, "Name:Bark\nManaCost:3 G\nTypes:Creature Treefolk\nPT:3/6\nOracle:x\n")
	// 6/3: power > toughness, so the filter EXCLUDES it -> still power 6,
	// not toughness 3 (a wrong blanket read would deal 3).
	thin := onBoardReady(t, e, 0, "Name:Sapling\nManaCost:3 G\nTypes:Creature Treefolk\nPT:6/3\nOracle:x\n")
	w1 := onBoard(t, e, 1, "Name:Wall1\nManaCost:1 W\nTypes:Creature Wall\nPT:2/9\nOracle:x\n")
	w2 := onBoard(t, e, 1, "Name:Wall2\nManaCost:1 W\nTypes:Creature Wall\nPT:2/9\nOracle:x\n")

	if p, to := e.Power(big), e.Toughness(big); p >= to {
		t.Fatalf("precondition: Bark P/T = %d/%d, want toughness greater", p, to)
	}
	if p, to := e.Power(thin), e.Toughness(thin); p <= to {
		t.Fatalf("precondition: Sapling P/T = %d/%d, want power greater", p, to)
	}

	e.askAttackers()
	submitAttackers(t, e, big, thin)
	submitBlockerPairs(t, e, [2]state.ObjID{big, w1}, [2]state.ObjID{thin, w2})

	if got := e.G.Obj(w1).Damage; got != 6 {
		t.Fatalf("Wall1 damage = %d, want 6 (Bark's toughness, the filter applies)", got)
	}
	if got := e.G.Obj(w2).Damage; got != 6 {
		t.Fatalf("Wall2 damage = %d, want 6 (Sapling's POWER, the powerLTtoughness filter excludes it, not its toughness 3)", got)
	}
}

func TestCombatDamageToughnessFeedsTheDivisionDecision(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := combatEngine(t)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Assault Formation"))
	// 1/4: power 1, toughness 4. With the static the attacker has 4 damage
	// to divide among two blockers -- a real division ask over 5 options --
	// where a power read would offer nothing but the single "1 to first"
	// composition.
	atk := onBoardReady(t, e, 0, "Name:Turtle\nManaCost:1 G\nTypes:Creature Turtle\nPT:1/4\nOracle:x\n")
	b1 := onBoard(t, e, 1, "Name:Wall1\nManaCost:1 W\nTypes:Creature Wall\nPT:0/4\nOracle:x\n")
	b2 := onBoard(t, e, 1, "Name:Wall2\nManaCost:1 W\nTypes:Creature Wall\nPT:0/4\nOracle:x\n")

	if p, to := e.Power(atk), e.Toughness(atk); p != 1 || to != 4 {
		t.Fatalf("precondition: attacker P/T = %d/%d, want 1/4", p, to)
	}

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockerPairsOnly(t, e, [2]state.ObjID{atk, b1}, [2]state.ObjID{atk, b2})
	drainCombatPriority(t, e)

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the damage-division KChoose over the TOUGHNESS amount, got %+v", d)
	}
	// The option table confirms the amount divided is 4, not 1.
	found := false
	for _, sp := range e.combatRound.askOptions {
		if len(sp) == 2 && sp[0]+sp[1] == 4 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no division option summing to the toughness 4; table=%v", e.combatRound.askOptions)
	}
	submitDivision(t, e, []int32{2, 2})

	if e.G.Obj(b1).Zone != state.ZBattlefield || e.G.Obj(b2).Zone != state.ZBattlefield {
		t.Fatal("precondition: both walls must survive the 2 damage each to be read")
	}
	if got := e.G.Obj(b1).Damage; got != 2 {
		t.Fatalf("Wall1 damage = %d, want 2", got)
	}
	if got := e.G.Obj(b2).Damage; got != 2 {
		t.Fatalf("Wall2 damage = %d, want 2", got)
	}
}

// submitBlockerPairsOnly is submitBlockerPairs without the trailing priority
// drain, for fixtures that must inspect the decision the damage step poses
// next.
func submitBlockerPairsOnly(t *testing.T, e *Engine, pairs ...[2]state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	var choices []int
	for _, pr := range pairs {
		idx := -1
		for _, o := range d.Options {
			if o.Obj == pr[1] && o.Attacker == pr[0] {
				idx = o.Index
				break
			}
		}
		if idx < 0 {
			t.Fatalf("no block option pairing attacker %d with blocker %d: %+v", pr[0], pr[1], d.Options)
		}
		choices = append(choices, idx)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit blocker pairs: %v", err)
	}
}
