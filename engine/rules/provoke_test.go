package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestProvokeForcesBlock is CR 702.39a: Goblin Grappler's Provoke, "whenever
// this creature attacks, you may have target creature defending player
// controls untap and block this creature this combat if able". The provoked
// creature, tapped before combat, untaps and owes the block.
func TestProvokeForcesBlock(t *testing.T) {
	t.Parallel()
	e, cfg := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Goblin Grappler"}, nil, nil,
		[]string{"Name:Provoked Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	grappler := findBattlefield(t, e, 0, "Goblin Grappler", 0)
	bear := findBattlefield(t, e, 1, "Provoked Bear", 0)
	e.emit(events.Event{Kind: events.Tap, Obj: bear})
	if !e.G.Obj(bear).Tapped {
		t.Fatal("precondition: the bear is not tapped")
	}
	e.askAttackers()
	submitAttackers(t, e, grappler)
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 && i > 0 {
				break
			}
			passPriorityOnce(t, e)
			continue
		}
		pick := -1
		for _, o := range d.Options {
			if o.Obj == bear || (pick < 0 && strings.EqualFold(o.Label, "yes")) {
				pick = o.Index
			}
		}
		if pick < 0 && len(d.Options) > 0 {
			pick = d.Options[0].Index
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
			t.Fatalf("submit %s: %v", d.Kind, err)
		}
	}
	if e.G.Obj(bear).Tapped {
		t.Fatal("the provoked creature did not untap")
	}
	if !combat.MustBlockPairRequired(asBoard(e), bear, grappler) {
		t.Fatal("the provoked creature does not have to block Goblin Grappler")
	}
	replayCheck(t, e, cfg)
}
