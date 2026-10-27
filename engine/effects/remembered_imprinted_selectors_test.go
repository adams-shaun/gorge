package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func selectorObjs(ts []state.Target) []state.ObjID {
	var out []state.ObjID
	for _, t := range ts {
		out = append(out, t.Obj)
	}
	return out
}

// TestRememberedImprintedSelectors pins the four Defined$ spellings that used
// to be unknown (so CopyPermanent made no token and the others fell back to
// the source): DirectRemembered, ExiledWithSource, Imprinted.<qualifier> and
// ParentTarget.<qualifier>. None may ever name the source for an empty set.
func TestRememberedImprintedSelectors(t *testing.T) {
	h := newHost(t, 2)
	add := func(name, types string, seat state.PlayerID) *state.Object {
		return h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:"+types+"\nOracle:x\n"), seat)
	}
	srcID := add("Src", "Artifact", 0).ID
	rememberedID := add("Remembered", "Creature", 1).ID
	exiledWithID := add("ExiledWith", "Creature", 1).ID
	imprintedOtherID := add("ImprintedOther", "Creature", 1).ID
	attackerID := add("Attacker", "Creature", 1).ID
	idlerID := add("Idler", "Creature", 1).ID
	// AddObject's pointer goes stale when the object table grows: bind the
	// live objects only after every card is added.
	src, remembered, exiledWith := h.g.Obj(srcID), h.g.Obj(rememberedID), h.g.Obj(exiledWithID)
	imprintedOther, attacker, idler := h.g.Obj(imprintedOtherID), h.g.Obj(attackerID), h.g.Obj(idlerID)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
	src.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{attacker.ID, idler.ID, remembered.ID})
	for _, o := range []*state.Object{attacker, idler, remembered} {
		o.Zone = state.ZBattlefield
	}
	h.g.SetZone(state.ZExile, 1, []state.ObjID{exiledWith.ID, imprintedOther.ID})
	exiledWith.Zone, imprintedOther.Zone = state.ZExile, state.ZExile
	exiledWith.ExiledWith, exiledWith.ExiledWithInc = src.ID, src.Incarnation+1
	src.Imprinted = []state.ObjID{exiledWith.ID, imprintedOther.ID}
	attacker.IsAttacking = true

	c := &Ctx{Source: src.ID, Controller: 0}
	empty := func(spec string) {
		t.Helper()
		got, ok := knownDefinedTargets(h, c, spec)
		if !ok || len(got) != 0 || slices.Contains(selectorObjs(got), src.ID) {
			t.Fatalf("%s with nothing bound = %v ok=%v, want known-empty (never the source)", spec, got, ok)
		}
	}
	// Empty sets first: no remembered object, no parent targets.
	empty("DirectRemembered")
	empty("ParentTarget.attacking")

	c.Remembered = []state.Target{{Obj: remembered.ID}}
	c.Targets = []state.Target{{Obj: attacker.ID}, {Obj: idler.ID}}

	if got, ok := knownDefinedTargets(h, c, "DirectRemembered"); !ok || !slices.Equal(selectorObjs(got), []state.ObjID{remembered.ID}) {
		t.Fatalf("DirectRemembered = %v ok=%v, want [%d]", got, ok, remembered.ID)
	}
	want, _ := knownDefinedTargets(h, c, "ExiledWith")
	if !slices.Equal(selectorObjs(want), []state.ObjID{exiledWith.ID}) {
		t.Fatalf("precondition: ExiledWith = %v, want [%d]", want, exiledWith.ID)
	}
	if got, ok := knownDefinedTargets(h, c, "ExiledWithSource"); !ok || !slices.Equal(selectorObjs(got), selectorObjs(want)) {
		t.Fatalf("ExiledWithSource = %v ok=%v, want %v", got, ok, want)
	}
	// The imprint pile holds both cards; the qualifier keeps only the one the
	// source exiled.
	if pile, _ := knownDefinedTargets(h, c, "Imprinted"); len(pile) != 2 {
		t.Fatalf("precondition: Imprinted = %v, want both cards", pile)
	}
	if got, ok := knownDefinedTargets(h, c, "Imprinted.ExiledWithSource"); !ok || !slices.Equal(selectorObjs(got), []state.ObjID{exiledWith.ID}) {
		t.Fatalf("Imprinted.ExiledWithSource = %v ok=%v, want [%d]", got, ok, exiledWith.ID)
	}
	if got, ok := knownDefinedTargets(h, c, "ParentTarget.attacking"); !ok || !slices.Equal(selectorObjs(got), []state.ObjID{attacker.ID}) {
		t.Fatalf("ParentTarget.attacking = %v ok=%v, want [%d]", got, ok, attacker.ID)
	}
	// A qualifier nothing matches is known-empty, not the source.
	src.Imprinted = []state.ObjID{imprintedOther.ID}
	empty("Imprinted.ExiledWithSource")
}
