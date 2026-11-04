package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSharedAnimosityCountsAttackersSharingTheAttackersType: Shared
// Animosity's "it gets +1/+0 for each other attacking creature that shares a
// creature type with it" reads the creature the Attacks trigger fired for
// (sharesCreatureTypeWith TriggeredAttacker, the instance's own attacker,
// CR 603.2c). Two Bears and a Giant attack: each Bear counts the other Bear
// and gets +1/+0; the Giant shares no type with either Bear and gets nothing.
func TestSharedAnimosityCountsAttackersSharingTheAttackersType(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bears, giant, anim := lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Hill Giant"), lookup(t, reg, "Shared Animosity")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{bears, bears, giant, anim}, []*cards.Card{giant})
	moveByName(t, e, 0, "Shared Animosity", state.ZBattlefield)
	var ids []state.ObjID
	for _, n := range []string{"Grizzly Bears", "Grizzly Bears", "Hill Giant"} {
		id := moveByName(t, e, 0, n, state.ZBattlefield)
		e.G.Obj(id).SummonSick = false
		ids = append(ids, id)
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: ids})
	// Three triggers of one controller: the order choice comes first.
	for i := 0; i < 10 && len(e.pendingTriggers) > 0; i++ {
		if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 1, 2}}); err != nil {
				t.Fatalf("order triggers: %v", err)
			}
			continue
		}
		e.putTriggersOnStack()
	}
	drainEtb(t, e)
	for i, want := range []int32{3, 3, 3} {
		if got := e.Power(ids[i]); got != want {
			t.Errorf("attacker %d (%s): power %d, want %d", i, e.G.Obj(ids[i]).Face().Name, got, want)
		}
	}
}
