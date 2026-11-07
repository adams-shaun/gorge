package rules

// graveyard_entry_watcher_test.go: an Origin$ Graveyard | Destination$
// Battlefield watcher is an entry trigger (Flayer of the Hatebound) and judges
// the permanent as it entered, so an animated-artifact return must not fire it.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const gyEntryWatcher = "Name:Entry Watcher\nManaCost:2 B\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Graveyard | Destination$ Battlefield | TriggerZones$ Battlefield | ValidCard$ Creature | Execute$ TrigGain\n" +
	"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n"

func TestGraveyardEntryWatcherIgnoresAnimatedArtifactReturn(t *testing.T) {
	t.Parallel()
	e := newSeats(t, 2)
	e.pending = nil
	watcher := onBoard(t, e, 0, gyEntryWatcher)
	tom := onBoard(t, e, 0, tomReturnFixture)
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: tom, From: state.ZBattlefield,
		To: state.ZGraveyard, Text: "lethal damage"})
	if len(e.pendingTriggers) != 1 || e.putTriggersOnStack() || len(e.G.Stack) != 1 {
		t.Fatalf("precondition: return trigger not on the stack: %+v", e.G.Stack)
	}
	e.resolveTop()
	if hasTypeWord(e.Derived(tom).Types, "Creature") {
		t.Fatalf("precondition: returned Tom is still a creature")
	}
	if o := e.G.Obj(watcher); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: watcher left the battlefield")
	}
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("queued %d trigger(s), want none: the permanent entered as an artifact", len(e.pendingTriggers))
	}
}
