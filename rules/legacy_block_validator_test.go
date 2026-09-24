package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
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
			t.Fatal("precondition: Menace attacker is not on the battlefield")
		}
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
		e.G.Step = state.StepDeclareBlockers
		e.askBlockers()
		d := e.Pending()
		if d == nil || len(d.Options) != 2 || d.Options[0].MinBlockers < 2 {
			t.Fatalf("precondition: expected two offered blocks with team bound: %+v", d)
		}
		board := botpolicy.BoardFromGame(e.G, e, 1)
		hasMenace := false
		for _, keyword := range board.Creatures[attacker].Keywords {
			if keyword == "Menace" {
				hasMenace = true
			}
		}
		if !hasMenace {
			t.Fatal("board omitted Menace")
		}
		in := botpolicy.LegacyDecide(board, d, rand.New(rand.NewPCG(seed, seed+1)))
		if err := e.Submit(in); err != nil {
			t.Fatalf("seed %d: legacy answer rejected: %v (choices %v)", seed, err, in.Choices)
		}
	}
}
