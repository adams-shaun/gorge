package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func seekFixture(t *testing.T) (*fakeHost, state.ObjID, []state.ObjID) {
	h := newHost(t, 2)
	source := h.g.AddObject(mkCard(t, "Name:Seeker\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{source.ID})
	creature := mkCard(t, "Name:Creature\nTypes:Creature\nPT:1/1\nOracle:x\n")
	land := mkCard(t, "Name:Land\nTypes:Land\nOracle:x\n")
	ids := fillLibrary(h.g, 0, land, 1)
	for i := 0; i < 2; i++ {
		o := h.g.AddObject(creature, 0)
		ids = append(ids, o.ID)
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	return h, source.ID, ids
}

func TestSeekRandomlyMovesMatchingCardsWithoutShuffleOrReveal(t *testing.T) {
	h, source, ids := seekFixture(t)
	Resolve(h, &Ctx{Controller: 0, Source: source}, sa(t, "SP$ Seek | Type$ Creature | Num$ 2"))
	if h.n != 2 {
		t.Fatalf("Rand calls = %d, want 2", h.n)
	}
	if len(h.g.Zone(state.ZHand, 0)) != 2 || len(h.g.Zone(state.ZLibrary, 0)) != 1 {
		t.Fatalf("hand/library = %d/%d, want 2/1", len(h.g.Zone(state.ZHand, 0)), len(h.g.Zone(state.ZLibrary, 0)))
	}
	for _, id := range ids {
		o := h.g.Obj(id)
		if o.Zone == state.ZHand && o.Card.Faces[0].Types[0] != "Creature" {
			t.Fatal("seek moved an ineligible card")
		}
	}
	for _, e := range h.log {
		if e.Kind == events.Shuffle || (e.Kind == events.Note && len(e.IDs) != 0) {
			t.Fatalf("seek emitted forbidden event: %+v", e)
		}
	}
}

func TestSeekEmptyAndRememberedImprintedRiders(t *testing.T) {
	h, source, _ := seekFixture(t)
	Resolve(h, &Ctx{Controller: 0, Source: source}, sa(t, "SP$ Seek | Type$ Artifact"))
	for _, e := range h.log {
		if e.Kind == events.Seek {
			t.Fatal("empty seek emitted marker")
		}
	}
	Resolve(h, &Ctx{Controller: 0, Source: source}, sa(t, "SP$ Seek | Type$ Creature | RememberFound$ True | ImprintFound$ True"))
	if len(h.g.Obj(source).Remembered) != 1 || len(h.g.Obj(source).Imprinted) != 1 {
		t.Fatalf("riders not recorded: remembered=%v imprinted=%v", h.g.Obj(source).Remembered, h.g.Obj(source).Imprinted)
	}
}

func TestSeekTypesAndTopTenPool(t *testing.T) {
	h := newHost(t, 1)
	land := mkCard(t, "Name:Land\nTypes:Land\nOracle:x\n")
	creature := mkCard(t, "Name:Creature\nTypes:Creature\nPT:1/1\nOracle:x\n")
	landID := h.g.AddObject(land, 0).ID
	creatureID := h.g.AddObject(creature, 0).ID
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{landID, creatureID})
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Seek | Types$ Land,Creature"))
	if len(h.g.Zone(state.ZHand, 0)) != 2 || h.n != 2 {
		t.Fatalf("Types seek hand/Rand = %d/%d, want 2/2", len(h.g.Zone(state.ZHand, 0)), h.n)
	}
	h = newHost(t, 1)
	ids := fillLibrary(h.g, 0, land, 10)
	o := h.g.AddObject(creature, 0)
	ids = append(ids, o.ID)
	h.g.SetZone(state.ZLibrary, 0, ids)
	below := o.ID
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Seek | Type$ Creature | DefinedCards$ Top_10_OfLibrary"))
	if h.g.Obj(below).Zone != state.ZLibrary {
		t.Fatal("Top_10_OfLibrary selected below-prefix card")
	}
}

func TestSeekDefinedEachPlayer(t *testing.T) {
	h := newHost(t, 2)
	creature := mkCard(t, "Name:Creature\nTypes:Creature\nPT:1/1\nOracle:x\n")
	fillLibrary(h.g, 0, creature, 1)
	fillLibrary(h.g, 1, creature, 1)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Seek | Defined$ Player | Type$ Creature"))
	if len(h.g.Zone(state.ZHand, 0)) != 1 || len(h.g.Zone(state.ZHand, 1)) != 1 {
		t.Fatalf("player seeks hand sizes = %d/%d", len(h.g.Zone(state.ZHand, 0)), len(h.g.Zone(state.ZHand, 1)))
	}
}
