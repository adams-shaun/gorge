package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRampageCountsBlockersBeyondTheFirst pins CR 702.23a on Craw Giant
// (Rampage 2): blocked by three creatures it gets +4/+4 when the trigger
// resolves, blocked by one it gets nothing.
func TestRampageCountsBlockersBeyondTheFirst(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	giantCard, ok := reg.Lookup("Craw Giant")
	if !ok {
		t.Fatal("Craw Giant not in the corpus")
	}
	if got, ok := giantCard.Faces[0].KeywordParam("Rampage"); !ok || got != "2" {
		t.Fatalf("Craw Giant keyword param = %q (%v), want Rampage 2", got, ok)
	}
	const bear = "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	for _, w := range []struct {
		blockers int
		bonus    int32
	}{{3, 4}, {2, 2}, {1, 0}} {
		e := combatEngine(t)
		giant := onBoardCard(t, e, 0, giantCard)
		e.G.Obj(giant).SummonSick = false
		var blk []state.ObjID
		for i := 0; i < w.blockers; i++ {
			blk = append(blk, onBoard(t, e, 1, bear))
		}
		e.askAttackers()
		submitAttackersOnly(t, e, giant)
		drainCombatPriority(t, e)
		if d := e.Pending(); d == nil || d.Kind != decision.KBlockers {
			t.Fatalf("expected a blockers decision, got %+v", d)
		}
		submitBlockersOnly(t, e, blk...)
		if len(e.G.Stack) != 1 {
			t.Fatalf("%d blockers: stack = %d, want one Rampage trigger", w.blockers, len(e.G.Stack))
		}
		if e.Power(giant) != 6 || e.Toughness(giant) != 4 {
			t.Fatalf("%d blockers before resolution = %d/%d, want 6/4", w.blockers, e.Power(giant), e.Toughness(giant))
		}
		e.resolveTop()
		if e.Power(giant) != 6+w.bonus || e.Toughness(giant) != 4+w.bonus {
			t.Fatalf("%d blockers after resolution = %d/%d, want %d/%d", w.blockers,
				e.Power(giant), e.Toughness(giant), 6+w.bonus, 4+w.bonus)
		}
	}
}
