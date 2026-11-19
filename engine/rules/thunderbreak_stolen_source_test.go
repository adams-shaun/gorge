package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestThunderbreakStolenSource pins CR 608.2h / 113.7a for Thunderbreak
// Regent's "that player": the opponent who controlled the targeting spell or
// ability WHEN it targeted. Seat 1 controls a Mogg Fanatic seat 0 owns,
// sacrifices it to target the Regent, and the Fanatic is in seat 0's
// graveyard (its owner's) by the time the Regent's trigger resolves -- the
// 3 damage still goes to seat 1.
func TestThunderbreakStolenSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, ids := targetingBoard(t, reg, []string{"Thunderbreak Regent"}, nil)
	regent := ids["Thunderbreak Regent"]
	// Seat 1 controls the Fanatic seat 0 owns: it entered under seat 1's
	// control, then its owner is rewritten (a Control Magic'd Fanatic).
	fo := e.G.AddObject(mustCorpusCard(t, reg, "Mogg Fanatic"), 1)
	fo.Zone = state.ZHand
	fanatic := fo.ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: fanatic, From: state.ZHand, To: state.ZBattlefield})
	e.G.Obj(fanatic).Owner = 0
	if o := e.G.Obj(fanatic); o.Controller != 1 || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Fanatic controller/zone = %d/%v, want 1/battlefield", o.Controller, o.Zone)
	}
	e.staticEpoch, e.activeEpoch = -1, -1
	e.G.Active, e.G.Priority = 1, 1
	e.askPriority(1)
	submitChoices(t, e, abilityOption(t, e, fanatic, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the Fanatic's target decision, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == regent {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Regent not offered as a target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	if z := e.G.Obj(fanatic).Zone; z == state.ZBattlefield {
		t.Fatalf("precondition: the Fanatic was sacrificed as a cost, still on the battlefield")
	}
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("seat 1 (the Fanatic's controller when it targeted) life = %d, want 17", got)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("seat 0 (the Fanatic's owner) life = %d, want 20 (the Fanatic's own 1 damage hit the Regent)", got)
	}
}
