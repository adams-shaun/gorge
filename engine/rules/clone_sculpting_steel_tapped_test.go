package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 614.12, 707.5: a permanent entering as a copy of an "enters tapped"
// permanent has the template's self entry replacement, so it enters tapped.
// Sculpting Steel copies Goldmire Bridge (Moved ... ReplaceWith$ ETBTapped).
func TestCloneSculptingSteelCopiesEntersTappedLand(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Sculpting Steel"))
	bridge := e.G.AddObject(corpusAlternativeCard(t, "Goldmire Bridge"), 1)
	bridge.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{bridge.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	if e.G.Obj(id).Face().Name == bridge.Face().Name || bridge.Zone != state.ZBattlefield || e.G.Obj(id).Tapped || bridge.Tapped {
		t.Fatal("precondition: distinct untapped Sculpting Steel and battlefield Bridge")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("Sculpting Steel copy election missing: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == bridge.ID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Bridge absent from election: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != bridge.Face().Name {
		t.Fatalf("Sculpting Steel entry = %+v, want Goldmire Bridge copy", o)
	}
	if !o.Tapped {
		t.Fatal("copy of an enters-tapped permanent entered untapped")
	}
}
