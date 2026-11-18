package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHKeywordsRampagePumpsPerExtraBlocker plays Marhault Elsdragon's
// printed Rampage 1 (CR 702.23) through a real combat: one blocker is the
// baseline (no pump), each blocker beyond the first pumps +1/+1 until end of
// turn when the becomes-blocked trigger resolves.
func TestEDHKeywordsRampagePumpsPerExtraBlocker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	marhault, ok := reg.Lookup("Marhault Elsdragon")
	if !ok {
		t.Fatal("Marhault Elsdragon not in the corpus")
	}
	if got, ok := marhault.Faces[0].KeywordParam("Rampage"); !ok || got != "1" {
		t.Fatalf("Marhault Elsdragon keyword param = %q (%v), want Rampage 1", got, ok)
	}
	const bear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

	// Two blockers: 4/6 becomes 5/6 when the trigger resolves.
	e := combatEngine(t)
	attacker := onBoardCard(t, e, 0, marhault)
	e.G.Obj(attacker).SummonSick = false
	b1 := onBoard(t, e, 1, bear)
	b2 := onBoard(t, e, 1, bear)
	e.askAttackers()
	submitAttackersOnly(t, e, attacker)
	drainCombatPriority(t, e)
	if d := e.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e, b1, b2)
	if n := len(e.G.Stack); n != 1 {
		t.Fatalf("stack = %d, want one Rampage trigger", n)
	}
	if e.Power(attacker) != 4 || e.Toughness(attacker) != 6 {
		t.Fatalf("before resolution = %d/%d, want 4/6", e.Power(attacker), e.Toughness(attacker))
	}
	e.resolveTop()
	if e.Power(attacker) != 5 || e.Toughness(attacker) != 7 {
		t.Fatalf("after resolution = %d/%d, want 5/7 (one blocker beyond the first pumps +1/+1)", e.Power(attacker), e.Toughness(attacker))
	}

	// One blocker: no pump at all.
	e2 := combatEngine(t)
	alone := onBoardCard(t, e2, 0, marhault)
	e2.G.Obj(alone).SummonSick = false
	blk := onBoard(t, e2, 1, bear)
	e2.askAttackers()
	submitAttackersOnly(t, e2, alone)
	drainCombatPriority(t, e2)
	if d := e2.Pending(); d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e2, blk)
	if n := len(e2.G.Stack); n != 1 {
		t.Fatalf("single-blocker stack = %d, want one Rampage trigger", n)
	}
	e2.resolveTop()
	if e2.Power(alone) != 4 || e2.Toughness(alone) != 6 {
		t.Fatalf("single blocker after resolution = %d/%d, want 4/6", e2.Power(alone), e2.Toughness(alone))
	}
}
