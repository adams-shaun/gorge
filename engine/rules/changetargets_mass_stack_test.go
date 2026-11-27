package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestChangeTargetsMassStack: Boltbender's "any number of other spells and/or
// abilities" is ChangeTargets over Defined$ ValidStack SpellAbility | Optional$
// True. Every stack object under the resolving redirect is offered its own
// "choose new target" ask, bottom first, and each answer lands on its object;
// the redirect itself is never offered.
func TestChangeTargetsMassStack(t *testing.T) {
	t.Parallel()
	e := stealEngine(t, 747)
	a := onBoard(t, e, 0, "Name:A\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoard(t, e, 0, "Name:B\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	const shock = "Name:Shock\nTypes:Instant\nManaCost:R\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2\nOracle:x\n"
	s1 := e.G.AddObject(card(t, shock), 1)
	s2 := e.G.AddObject(card(t, shock), 1)
	mass := e.G.AddObject(card(t, "Name:Bender\nTypes:Instant\nManaCost:R\nA:SP$ ChangeTargets | Defined$ ValidStack SpellAbility | Optional$ True\nOracle:x\n"), 0)
	for _, id := range []state.ObjID{s1.ID, s2.ID, mass.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZStack})
	}
	for _, id := range []state.ObjID{s1.ID, s2.ID} {
		e.emit(events.Event{Kind: events.TargetsChosen, Obj: id, IDs: []state.ObjID{a}})
		if got := e.G.Obj(id).Targets; len(got) != 1 || got[0].Obj != a {
			t.Fatalf("precondition: spell %d targets = %+v, want creature %d", id, got, a)
		}
	}
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: mass.ID, ResolvingObj: mass.ID, Controller: 0, SVars: mass.Face().SVars}
	}, mass.Face().SpellAbility())
	for i, id := range []state.ObjID{s1.ID, s2.ID} {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			t.Fatalf("ask %d: want a choose, got %+v (targets so far: %+v / %+v)", i, d,
				e.G.Obj(s1.ID).Targets, e.G.Obj(s2.ID).Targets)
		}
		if offersObj(d, mass.ID) {
			t.Fatalf("ask %d offers the redirect itself: %+v", i, d.Options)
		}
		submitChoices(t, e, optionIdxForObj(t, d, b))
		if got := e.G.Obj(id).Targets; len(got) != 1 || got[0].Obj != b {
			t.Fatalf("spell %d targets after ask %d = %+v, want creature %d", id, i, got, b)
		}
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("a third redirect ask was posed: %+v", d)
	}
}
