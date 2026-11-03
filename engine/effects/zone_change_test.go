package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 304.4 and 307.4: an instant or sorcery card a ChangeZone would put onto
// the battlefield stays where it is (Julian Bashir's script names no creature
// type for its target, so a graveyard instant is a legal target).
func TestChangeZoneLeavesAnInstantInTheGraveyard(t *testing.T) {
	h, _ := fixtureHost(t)
	inst := h.g.AddObject(mkCard(t, "Name:Fizzle Fixture\nTypes:Instant\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: inst.ID, From: inst.Zone, To: state.ZGraveyard})
	cre := h.g.AddObject(mkCard(t, "Name:Body Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: cre.ID, From: cre.Zone, To: state.ZGraveyard})
	sa := &cards.SA{Params: map[string]string{"Origin": "Graveyard", "Destination": "Battlefield"}}
	cz := ChangeZoneOf(sa)
	if !changeZoneMoveSkip(h, cz, h.g.Obj(inst.ID), state.ZBattlefield, false) {
		t.Fatalf("an instant card in the graveyard is not skipped when it would enter the battlefield")
	}
	if changeZoneMoveSkip(h, cz, h.g.Obj(cre.ID), state.ZBattlefield, false) {
		t.Fatalf("a creature card in the graveyard is skipped; it enters")
	}
	if changeZoneMoveSkip(h, cz, h.g.Obj(inst.ID), state.ZHand, false) {
		t.Fatalf("an instant card is skipped on its way to hand")
	}
}
