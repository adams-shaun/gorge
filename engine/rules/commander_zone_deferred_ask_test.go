package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// bounceThenSearchFixtureSrc is a bounce-then-fetch spell, authored freely:
// return a target creature to its owner's hand (a CR 903.9b commander
// replacement parks there), then a SECOND step of the same resolution poses
// a decision -- the bounced creature's controller may search for a basic
// land.
const bounceThenSearchFixtureSrc = `Name:Fixture Bounce Then Search
ManaCost:0
Types:Instant
A:SP$ ChangeZone | Origin$ Battlefield | Destination$ Hand | ValidTgts$ Creature | SubAbility$ DBFetch | SpellDescription$ Bounce a creature; its controller may fetch a basic land.
SVar:DBFetch:DB$ ChangeZone | Optional$ True | Origin$ Library | Destination$ Battlefield | Tapped$ True | ChangeType$ Land.Basic | DefinedPlayer$ TargetedController | ShuffleNonMandatory$ True
Oracle:Bounce a creature; its controller may fetch a basic land.
`

// moveToHandByName moves the named seeded deck card from the library to
// seat p's hand through a LOGGED MoveZone, so log-only replay reconstructs it.
func moveToHandByName(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
			return id
		}
	}
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("card %q not found in seat %d's library or hand", name, p)
	return 0
}
