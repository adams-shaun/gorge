package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A copy of a permanent spell is a token as it enters (CR 608.3f), so Satoru's copy,
// made by Double Down, is no "nontoken creature" for its own trigger. The trigger
// matches a moving source against the snapshot taken on the stack.
func TestCopiedSpellDoesNotFireItsOwnNontokenTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Satoru, the Infiltrator"), lookup(t, reg, "Double Down")}, nil)
	moveCorpusCard(t, e, "Double Down", 0, state.ZBattlefield)
	moveCorpusCard(t, e, "Satoru, the Infiltrator", 0, state.ZHand)
	castVanilla(t, e, "Satoru, the Infiltrator", "UB")
	if n := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.StackCopy }); n != 1 {
		t.Fatalf("Double Down copied %d times, want 1", n)
	}
	draws := countEvents(e, func(ev events.Event) bool {
		o := e.G.Obj(ev.Obj)
		return ev.Kind == events.TriggerPush && o != nil && o.Face() != nil && o.Face().Name == "Satoru, the Infiltrator"
	})
	if draws != 0 {
		t.Fatalf("Satoru triggered %d times for a cast spell and a token copy, want 0", draws)
	}
	replayCheck(t, e, cfg)
}
