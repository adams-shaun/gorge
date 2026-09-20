package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestFlankingKnightOfTheHolyNimbusDebuffsBlockers pins kw:Flanking (CR
// 702.25a) on the real corpus card Knight of the Holy Nimbus: a flanking
// attacker blocked by a creature WITHOUT flanking gives that blocker -1/-1
// until end of turn (a 1/1 blocker dies; a 2/2 blocker survives at 1/1), a
// blocker that itself has flanking gets nothing, and an unblocked flanking
// attacker does nothing. The expansion is a Mode$ AttackerBlockedByCreature
// trigger, one instance per (attacker, non-flanking blocker) pair, whose body
// pumps Defined$ TriggeredBlockerLKICopy -1/-1 -- the ordinary pump/continuous
// path, never a direct state write.
func TestFlankingKnightOfTheHolyNimbusDebuffsBlockers(t *testing.T) {
	nimbus := mshCorpusCardPath(t, "Knight of the Holy Nimbus", "k/knight_of_the_holy_nimbus.txt")
	e := combatEngine(t)
	knight := onBoardCard(t, e, 0, nimbus)
	e.G.Obj(knight).SummonSick = false
	bear := onBoard(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	askari := onBoard(t, e, 1, "Name:Blazing Blade Askari\nManaCost:1 R\nTypes:Creature Human Knight\nPT:2/2\nK:Flanking\nOracle:x\n")

	e.askAttackers()
	submitAttackersOnly(t, e, knight)
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	// Bear (no flanking) blocks first, Askari (flanking) second: both pairs
	// carry the knight as attacker.
	submitBlockersOnly(t, e, bear, askari)
	e.resolveTop()

	if got := e.Power(bear); got != 1 || e.Toughness(bear) != 1 {
		t.Fatalf("non-flanking blocker = %d/%d, want 1/1 until EOT", e.Power(bear), e.Toughness(bear))
	}
	if got := e.Power(askari); got != 2 || e.Toughness(askari) != 2 {
		t.Fatalf("flanking blocker = %d/%d, want untouched 2/2", e.Power(askari), e.Toughness(askari))
	}
	if e.Power(knight) != 2 || e.Toughness(knight) != 2 {
		t.Fatalf("attacker = %d/%d, want untouched 2/2", e.Power(knight), e.Toughness(knight))
	}

	// Lethal for a 1/1: a second run with a Memnite blocker sees it die to
	// the SBA once the -1/-1 lands.
	e2 := combatEngine(t)
	knight2 := onBoardCard(t, e2, 0, nimbus)
	e2.G.Obj(knight2).SummonSick = false
	memnite := onBoard(t, e2, 1, "Name:Memnite\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n")
	e2.askAttackers()
	submitAttackersOnly(t, e2, knight2)
	drainCombatPriority(t, e2)
	if d2 := e2.Pending(); d2 == nil || d2.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d2)
	} else {
		submitBlockersOnly(t, e2, memnite)
		e2.resolveTop()
		// The SBA check a real game runs before the next priority round.
		e2.checkStateBased()
	}
	if e2.G.Obj(memnite).Zone != state.ZGraveyard {
		t.Fatalf("1/1 blocker = %v, want graveyard (0 toughness after -1/-1)", e2.G.Obj(memnite).Zone)
	}

	// An unblocked flanking attacker debuffs nothing.
	e3 := combatEngine(t)
	knight3 := onBoardCard(t, e3, 0, nimbus)
	e3.G.Obj(knight3).SummonSick = false
	b3 := onBoard(t, e3, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e3.askAttackers()
	submitAttackersOnly(t, e3, knight3)
	drainCombatPriority(t, e3)
	if d3 := e3.Pending(); d3 == nil || d3.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d3)
	} else {
		// No blocks: submit the empty declaration; nothing may queue.
		submitBlockersOnly(t, e3)
		if len(e3.pendingTriggers) != 0 || len(e3.G.Stack) != 0 {
			t.Fatalf("unblocked flanking attacker queued %d triggers / %d stack objects", len(e3.pendingTriggers), len(e3.G.Stack))
		}
	}
	if got := e3.Power(b3); got != 2 || e3.Toughness(b3) != 2 {
		t.Fatalf("unblocked flanking attacker debuffed the bear = %d/%d, want 2/2", e3.Power(b3), e3.Toughness(b3))
	}
}
