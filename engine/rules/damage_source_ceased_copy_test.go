package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A token copy of Steampath Charger dies and its trigger deals damage as the Lizard it
// last was (CR 608.2h), so Valley Flamecaller adds 1 though the token has ceased.
func TestDamageReplacementReadsACeasedTokenCopySource(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	addToBattlefield(t, e, corpusCard(t, "Valley Flamecaller"), 0)
	id := addToBattlefield(t, e, corpusCard(t, "Steampath Charger"), 0)
	o := e.G.Obj(id)
	o.IsToken, o.IsCopy = true, true
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	d := passUntilTarget(t, e, 5)
	for i, opt := range d.Options {
		if opt.Player == 1 {
			submitChoices(t, e, i)
			break
		}
	}
	before := e.G.Players[1].Life
	passUntil(t, e, func() bool { return len(e.G.Stack) == 0 })
	if got := before - e.G.Players[1].Life; got != 2 {
		t.Fatalf("the trigger dealt %d, want 1 plus Valley Flamecaller's 1", got)
	}
}
