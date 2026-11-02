package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "Permanents enter tapped" reads the card as it would exist on the battlefield,
// from any zone (CR 110.1, 614.12).
func TestOrbOfDreamsTapsAnEntryFromAnyZone(t *testing.T) {
	for _, from := range []state.Zone{state.ZLibrary, state.ZHand, state.ZGraveyard, state.ZExile} {
		t.Run(from.String(), func(t *testing.T) {
			reg := testutil.CorpusRegistry(t)
			e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Orb of Dreams"), lookup(t, reg, "Grizzly Bears")}, nil)
			moveByName(t, e, 0, "Orb of Dreams", state.ZBattlefield)
			bears := moveByName(t, e, 0, "Grizzly Bears", from)
			e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: from, To: state.ZBattlefield})
			if !e.G.Obj(bears).Tapped {
				t.Fatalf("Bears entered from %v untapped", from)
			}
		})
	}
}

func TestOrbOfDreamsTapsALand(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _ := animateTriggersEngine(t, reg, lookup(t, reg, "Orb of Dreams"))
	moveByName(t, e, 0, "Orb of Dreams", state.ZBattlefield)
	mountain := moveByName(t, e, 0, "Mountain", state.ZLibrary)
	e.emit(events.Event{Kind: events.MoveZone, Obj: mountain, From: state.ZLibrary, To: state.ZBattlefield})
	if !e.G.Obj(mountain).Tapped {
		t.Fatal("a land from the library entered untapped")
	}
}

// A face-down card is a permanent whatever its printed types say (CR 708.2a).
func TestOrbOfDreamsTapsAFaceDownInstant(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Orb of Dreams"), lookup(t, reg, "Lightning Bolt")}, nil)
	moveByName(t, e, 0, "Orb of Dreams", state.ZBattlefield)
	bolt := moveByName(t, e, 0, "Lightning Bolt", state.ZLibrary)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bolt, From: state.ZLibrary, To: state.ZBattlefield, Counter: events.FaceDownEntryCounter})
	if !e.G.Obj(bolt).Tapped {
		t.Fatal("a manifested instant entered untapped")
	}
}
