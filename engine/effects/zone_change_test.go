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

// A double-faced sorcery put onto the battlefield transformed enters as its
// back face (CR 711.10a), so the instant-or-sorcery guard leaves it alone;
// without Transformed$ it stays put.
func TestChangeZoneLetsATransformedSorceryEnter(t *testing.T) {
	h, _ := fixtureHost(t)
	dfc := h.g.AddObject(mkCard(t, "Name:Front Fixture\nTypes:Sorcery\nOracle:x\nAlternateMode:DoubleFaced\n\nALTERNATE\n\nName:Back Fixture\nTypes:Enchantment Creature Saga\nPT:4/4\nOracle:y\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: dfc.ID, From: dfc.Zone, To: state.ZExile})
	plain := ChangeZoneOf(&cards.SA{Params: map[string]string{"Origin": "Exile", "Destination": "Battlefield"}})
	flipped := ChangeZoneOf(&cards.SA{Params: map[string]string{"Origin": "Exile", "Destination": "Battlefield", "Transformed": "True"}})
	if !changeZoneMoveSkip(h, plain, h.g.Obj(dfc.ID), state.ZBattlefield, false) {
		t.Fatalf("a sorcery front face entering untransformed is not skipped")
	}
	if changeZoneMoveSkip(h, flipped, h.g.Obj(dfc.ID), state.ZBattlefield, false) {
		t.Fatalf("a double-faced sorcery entering transformed is skipped; it enters as its back face")
	}
}
