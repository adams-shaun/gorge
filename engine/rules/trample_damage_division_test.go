package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTrampleSingleBlockerDamageAssignmentIsAChoice pins CR 702.19b/510.1c: a
// trample attacker's controller divides the damage between the blockers (at
// least lethal to each) and the defending player; the excess is not forced
// through. A 6/1 trampler blocked by a 2/2 must pose one division whose
// legal splits run from "2 to the blocker, 4 tramples" through "6 to the
// blocker, nothing tramples".
func TestTrampleSingleBlockerDamageAssignmentIsAChoice(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	atk := onBoardReady(t, e, 0, "Name:Charger\nManaCost:3 R\nTypes:Creature Beast\nPT:6/1\nK:Trample\nOracle:x\n")
	blk := onBoard(t, e, 1, "Name:Guard\nManaCost:1 W\nTypes:Creature Soldier\nPT:2/2\nOracle:x\n")

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockers(t, e, blk)

	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Source != atk || d.Player != 0 {
		t.Fatalf("pending = %+v, want the trample attacker's controller division ask", d)
	}
	if len(e.combatRound.askOptions) != 5 {
		t.Fatalf("division options = %d, want 5 splits of 6 across the 2/2 and the player: %v",
			len(e.combatRound.askOptions), e.combatRound.askOptions)
	}
	if first := e.combatRound.askOptions[0]; first[0] != 2 || first[1] != 4 {
		t.Fatalf("first division = %v, want [2 4] (lethal to the 2/2, rest tramples)", first)
	}
	if last := e.combatRound.askOptions[len(e.combatRound.askOptions)-1]; last[0] != 6 || last[1] != 0 {
		t.Fatalf("last division = %v, want [6 0] (all to the blocker, nothing tramples)", last)
	}

	// Answer the maximal-blocker split: nothing tramples, so the defending
	// player stays at 20 and the 2/2 dies to all 6.
	submitDivision(t, e, []int32{6, 0})
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("defender life = %d, want 20 (all 6 assigned to the blocker, none tramples)", got)
	}
	if o := e.G.Obj(blk); o.Zone != state.ZGraveyard {
		t.Fatalf("blocker zone = %v, want graveyard (assigned all 6)", o.Zone)
	}
}
