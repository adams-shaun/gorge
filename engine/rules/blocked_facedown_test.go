package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestFaceDownCreatureSeesNoBlockedTrigger pins CR 708.2a on the become-blocked
// scan: a manifested Norin, Swift Survivalist is a 2/2 with no abilities, so
// when it attacks and is blocked nothing is queued. The face-up control arm
// queues its "may exile it" trigger, so the scan is live.
func TestFaceDownCreatureSeesNoBlockedTrigger(t *testing.T) {
	t.Parallel()
	norin := corpusAlternativeCard(t, "Norin, Swift Survivalist")
	for _, down := range []bool{false, true} {
		e := blocksCombatEngine(t, 0)
		n := onBoardCard(t, e, 0, norin)
		e.G.Obj(n).SummonSick = false
		e.G.Obj(n).FaceDown = down
		b := onBoard(t, e, 1, "Name:Memnite\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n")
		e.askAttackers()
		submitAttackersOnly(t, e, n)
		drainCombatPriority(t, e)
		if d := e.Pending(); d == nil || d.Kind != decision.KBlockers {
			t.Fatalf("down=%v: expected a blockers decision, got %+v", down, d)
		}
		submitBlockersOnly(t, e, b)
		want := 1
		if down {
			want = 0
		}
		if got := len(e.G.Stack); got != want {
			t.Fatalf("face down=%v: %d stack entries after the block, want %d", down, got, want)
		}
	}
}
