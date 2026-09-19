package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The zoneGate self-move admission's wider measured population, pinned on the
// elder carrier: the "put into a graveyard FROM ANYWHERE" ChangesZone
// self-triggers (Origin$ Any | Destination$ Graveyard | ValidCard$ Card.Self,
// no TriggerZones$) — Emrakul, Kozilek, Ulamog, Worldspine Wurm, Dread,
// Guile, Vigor, Purity, Hostility, Serra Avatar — must fire when the card is
// discarded FROM ITS HAND, the one zone the battlefield-default zone check
// cannot observe. The admission extends the Discarded/Cycled courtesy to
// every no-TriggerZones$ self move: measured population 14 blocks / 14 files
// (11 elders, bone_rattler, gixian_recycler, fear_of_change, Lupine
// Harbingers), every fire oracle-correct for the "from anywhere" carriers.
//
// Emrakul, the Aeons Torn is the pin: discarded from hand, its owner shuffles
// their WHOLE graveyard into their library (the ChangeZoneAll body).

func TestEmrakulDiscardedFromHandShufflesTheGraveyardBack(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Emrakul, the Aeons Torn"))
	emrakul := e.G.Zone(state.ZHand, 0)[0]
	// Graveyard stock the shuffle must return with it.
	fillers := []state.ObjID{
		graveCreature(t, e, 0, "Name:FillerA\nManaCost:no cost\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
		graveCreature(t, e, 0, "Name:FillerB\nManaCost:no cost\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: emrakul, From: state.ZHand, To: state.ZGraveyard,
		Text: "discarded"})
	// The admission queued the elder's own trigger (the zone the battlefield
	// default cannot see: the card sat in hand when it moved).
	if n := observedTriggerCount(e, emrakul); n != 1 {
		t.Fatalf("hand-discarded Emrakul queued %d triggers, want the from-anywhere shuffle's 1", n)
	}
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			break
		}
		e.resolveTop()
	}
	if o := e.G.Obj(emrakul); o.Zone != state.ZLibrary {
		t.Fatalf("discarded-from-hand Emrakul sits in %s, want library (shuffled back from anywhere)", o.Zone)
	}
	for _, fid := range fillers {
		if o := e.G.Obj(fid); o.Zone != state.ZLibrary {
			t.Fatalf("graveyard filler %s stayed in %s, want library (the whole graveyard shuffled)", o.Face().Name, o.Zone)
		}
	}
}
