package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

type incTypeChars struct {
	types []string
}

func (c incTypeChars) Power(state.ObjID) int32     { return 2 }
func (c incTypeChars) Toughness(state.ObjID) int32 { return 2 }
func (c incTypeChars) Keywords(state.ObjID) []string {
	return nil
}
func (c incTypeChars) Characteristics(state.ObjID) (int32, int32, []string) {
	return 2, 2, nil
}
func (c incTypeChars) ViewTypes(state.ObjID) ([]string, bool) {
	return c.types, c.types != nil
}

// TestBoardIncDeriveUsesLayer4Types pins the incremental cache to the same
// layer-4 type line the view half projects. A row's printed face says
// Creature, but the engine reports it as a Land, so the cached Creature/Land
// and TypeSig facts must follow the derived line; otherwise the incremental
// fill and the scratch fill disagree the moment a type-changing effect lands.
func TestBoardIncDeriveUsesLayer4Types(t *testing.T) {
	face := &cards.Face{Name: "Test Bear", Types: []string{"Creature", "Bear"}}
	inc := &boardInc{fill: 1, gen: 1}
	r := &boardRow{face: face, creature: true, typeSig: TypeSigOf(face.Types)}

	inc.derive(r, 7, incTypeChars{types: []string{"Land"}}, incTypeChars{types: []string{"Land"}})

	if r.creature {
		t.Error("Creature = true, want false from the derived Land line")
	}
	if !r.land {
		t.Error("Land = false, want true from the derived Land line")
	}
	if want := TypeSigOf([]string{"Land"}); r.typeSig != want {
		t.Errorf("TypeSig = %#x, want %#x", r.typeSig, want)
	}
}
