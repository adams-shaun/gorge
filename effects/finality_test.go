package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestFinalityCounterExilesDestroyedPermanent(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	o := g.Obj(ids["myBear"])
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: bear zone = %s, want battlefield", o.Zone)
	}
	h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "FINALITY", Amount: 1})
	if o.Counter("FINALITY") != 1 {
		t.Fatalf("precondition: finality counters = %d, want 1", o.Counter("FINALITY"))
	}

	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: o.ID}}}, sa(t, "SP$ Destroy | ValidTgts$ Creature"))
	if o.Zone != state.ZExile {
		t.Fatalf("finality permanent zone = %s, want exile", o.Zone)
	}
	foundMove := false
	for _, ev := range h.log {
		if ev.Kind == events.MoveZone && ev.Obj == o.ID {
			foundMove = true
			if ev.To != state.ZExile {
				t.Fatalf("finality move destination = %s, want exile", ev.To)
			}
		}
	}
	if !foundMove {
		t.Fatal("destroy did not emit a finality exile MoveZone event")
	}
}

func TestWithTotalCardTypesRejectsInsufficientHiddenPick(t *testing.T) {
	g := state.NewGame(names(2))
	creature := mkCard(t, "Name:A\nTypes:Creature\nPT:2/2\nOracle:x\n")
	otherCreature := mkCard(t, "Name:B\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	a := g.AddObject(creature, 0)
	b := g.AddObject(otherCreature, 0)
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{a.ID, b.ID})
	a.Zone, b.Zone = state.ZGraveyard, state.ZGraveyard
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Source: a.ID}
	ability := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile | ChangeNum$ 2 | ChangeType$ Card | WithTotalCardTypes$ 2")

	// Both candidates are creatures, so the chosen set has only one distinct
	// card type and must fail closed rather than exiling an invalid set.
	applyLibrarySearch(h, c, ability, 0, state.ZExile, []state.ObjID{a.ID, b.ID}, []state.Zone{state.ZGraveyard})
	if a.Zone != state.ZGraveyard || b.Zone != state.ZGraveyard {
		t.Fatalf("insufficient-type pick moved cards: zones = %s, %s", a.Zone, b.Zone)
	}
	foundNote := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && ev.Text == "hidden pick fails WithTotalCardTypes$ requirement" {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatalf("failed hidden pick did not record a Note witness: log=%+v params=%+v", h.log, ability.Params)
	}
}
