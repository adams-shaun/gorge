package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCloneKillerCosplayCopiesANamedBackFace is CR 712.19 on the chosen-name
// copy route: Killer Cosplay's NameCard may name a DFC's back face
// (Insectile Aberration), and the CopyFromChosenName$ copy then copies THAT
// face, not the card's front.
func TestCloneKillerCosplayCopiesANamedBackFace(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	h := &fakeHost{g: state.NewGame(names(2))}
	cosplay, _ := reg.Lookup("Killer Cosplay")
	forest, _ := reg.Lookup("Forest")
	if cosplay == nil || forest == nil {
		t.Fatal("missing corpus cards")
	}
	h.g.NameUniverse = reg.Universe()
	a, b := h.g.AddObject(cosplay, 0).ID, h.g.AddObject(forest, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{a, b})
	h.g.Obj(a).Zone, h.g.Obj(b).Zone = state.ZBattlefield, state.ZBattlefield
	original := h.g.Obj(a).Face()
	body := cards.ResolveSVar(original.SVars, "DBCopy")
	h.Emit(events.Event{Kind: events.Choose, Obj: a, Counter: "name", Text: "Insectile Aberration"})
	Resolve(h, &Ctx{Source: a, Controller: 0, Targets: []state.Target{{Obj: b}}, SVars: original.SVars}, body)
	if f := h.g.Obj(b).Face(); f == nil || f.Name != "Insectile Aberration" {
		name := "<nil>"
		if f != nil {
			name = f.Name
		}
		t.Fatalf("chosen-name copy of a back-face name is %q, want the Insectile Aberration face", name)
	}
}
