package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEntersLineWithoutZonesDoesNotLookBack pins CR 603.6a against CR 603.10a
// for the corpus's two ChangesZone lines that name neither Origin$ nor
// Destination$ (Ghostly Dancers, Bullseye, Death Dealer, both printed "When
// CARDNAME enters"): the trigger fires when the card enters the battlefield
// and NOT when it leaves it.
func TestEntersLineWithoutZonesDoesNotLookBack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Ghostly Dancers", "Bullseye, Death Dealer"} {
		t.Run(name, func(t *testing.T) {
			e := layerEngine(t)
			id := onBoardCard(t, e, 0, mustCorpusCard(t, reg, name))
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield,
				To: state.ZGraveyard, Text: "destroyed"})
			if n := observedTriggerCount(e, id); n != 0 {
				t.Fatalf("%s leaving the battlefield queued %d triggers, want 0 (CR 603.6a: an enters trigger)", name, n)
			}
			e2 := layerEngine(t)
			id2 := onBoardCard(t, e2, 0, mustCorpusCard(t, reg, name))
			e2.emit(events.Event{Kind: events.MoveZone, Obj: id2, From: state.ZStack,
				To: state.ZBattlefield})
			if n := observedTriggerCount(e2, id2); n != 1 {
				t.Fatalf("%s entering the battlefield queued %d triggers, want its enters trigger", name, n)
			}
		})
	}
}
