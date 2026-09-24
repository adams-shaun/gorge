package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestLegacyBlockAnswerPassesTheBlockValidator(t *testing.T) {
	const menace = "Name:Goblin Glory Chaser\nManaCost:R\nTypes:Creature Goblin Warrior\nPT:1/1\nK:Menace\nOracle:x\n"
	const blocker = "Name:Memnite\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n"
	for seed := uint64(0); seed < 100; seed++ {
		e := combatEngine(t)
		attacker := onBoardReady(t, e, 0, menace)
		onBoard(t, e, 1, blocker)
		onBoard(t, e, 1, blocker)
		if e.G.Obj(attacker).Zone != state.ZBattlefield || !e.HasKeyword(attacker, "Menace") {
			t.Fatal("Menace attacker precondition missing")
		}
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
		e.G.Step = state.StepDeclareBlockers
		e.askBlockers()
		d := e.Pending()
		if d == nil || d.Kind != decision.KBlockers || len(d.Options) != 2 {
			t.Fatalf("pending blockers decision: %+v", d)
		}
		board := botpolicy.BoardFromGame(e.G, e, 1)
		in := botpolicy.LegacyDecide(board, d, rand.New(rand.NewPCG(seed, seed+1)))
		if err := e.Submit(in); err != nil {
			t.Fatalf("seed %d submit %v: %v", seed, in.Choices, err)
		}
	}
}
