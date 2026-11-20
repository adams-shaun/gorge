package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// materiaBoard seats p0 with a Forest, a Smokebraider wearing Summoning Materia
// (its granted {T}: Add {G} beside its own Elemental-only mana), `extra` more
// bare Smokebraiders, and Nissa's Chosen ({G}{G}) both on top of the library
// and in hand, then poses the seat-0 priority decision. It returns the plans
// of the library cast (mode "mayplay") and the hand cast.
func materiaBoard(t *testing.T, extra int) (library, hand PotentialPlan) {
	t.Helper()
	reg := searchTestRegistry(t)
	e := handEngine(t)
	put := func(c *cards.Card) state.ObjID {
		o := e.G.AddObject(c, 0)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, 0, append(append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...), o.ID))
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield})
		return o.ID
	}
	host := put(lookup(t, reg, "Smokebraider"))
	e.emit(events.Event{Kind: events.Attach, Obj: put(lookup(t, reg, "Summoning Materia")), IDs: []state.ObjID{host}})
	for i := 0; i < extra; i++ {
		put(lookup(t, reg, "Smokebraider"))
	}
	put(lookup(t, reg, "Forest"))
	top := e.G.AddObject(lookup(t, reg, "Nissa's Chosen"), 0)
	top.Zone = state.ZLibrary
	e.G.SetZone(state.ZLibrary, 0, append([]state.ObjID{top.ID}, e.G.Zone(state.ZLibrary, 0)...))
	inHand := handCard(e, lookup(t, reg, "Nissa's Chosen"), 0)
	e.G.Turn = 5
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		e.G.Obj(id).SummonSick = false
	}
	ppPriority(t, e)
	for _, pp := range e.PotentialPaymentPlans(0) {
		switch {
		case pp.Action.Kind == "cast" && pp.Action.Obj == top.ID && pp.Action.Mode == "mayplay":
			library = pp
		case pp.Action.Kind == "cast" && pp.Action.Obj == inHand:
			hand = pp
		}
	}
	if library.Action.Obj == 0 || hand.Action.Obj == 0 {
		t.Fatalf("both Nissa's Chosen casts must be potential plays: library %+v hand %+v", library.Action, hand.Action)
	}
	return library, hand
}

// CR 106.6: a bare Smokebraider's mana is spendable on Elemental spells only,
// so beside the Materia's granted {G} it is a dead gap for Nissa's Chosen. The
// prefix search drops such a gap for a hand cast, and must for the same
// printed-cost cast from the top of the library: with it kept, the search
// spent its node budget on that source's colour branches and left the legal
// cast census_incomplete.
func TestMayPlayLibraryCastDropsUnspendableGaps(t *testing.T) {
	t.Parallel()
	for extra := 0; extra <= 2; extra++ {
		library, hand := materiaBoard(t, extra)
		if hand.Reason != "" || len(hand.Script) == 0 {
			t.Fatalf("extra=%d: precondition: the hand cast must be priced by a prefix script: %+v", extra, hand)
		}
		if library.Reason != "" || len(library.Script) == 0 {
			t.Errorf("extra=%d: the library cast is %q/%q with %d script steps, want a prefix script like the hand cast",
				extra, library.Reason, library.Detail, len(library.Script))
		}
	}
}
