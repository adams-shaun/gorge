package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDestroyAllTargetedMatcherAnchors holds the Targeted.-prefixed
// DestroyAll ValidCards$ shapes (Turn to Slag, Isle of Vesuva, Eaten by
// Spiders) to Forge's filterListByType reading: the first object target is
// the relative source, Attached means attached to it, Other excludes it and
// sameName names it.
func TestDestroyAllTargetedMatcherAnchors(t *testing.T) {
	g, ids := board(t)
	anchor := ids["myBear"]
	namesake := addBattlefieldCard(t, g, "Bear", "Creature Bear")
	bystander := ids["myFlier"]
	mine := addBattlefieldCard(t, g, "Sword", "Artifact Equipment")
	theirs := addBattlefieldCard(t, g, "Axe", "Artifact Equipment")
	g.Obj(mine).AttachedTo = anchor
	g.Obj(theirs).AttachedTo = bystander
	if g.Obj(anchor).Face().Name != g.Obj(namesake).Face().Name || g.Obj(anchor).Face().Name == g.Obj(bystander).Face().Name {
		t.Fatal("precondition: namesake shares the anchor's name, bystander does not")
	}
	sc := SpecContext{You: 0, ResolutionTargets: []state.Target{{Obj: anchor}}, Resolving: true}
	for _, c := range []struct {
		rest string
		want map[state.ObjID]bool
	}{
		{"Equipment+Attached", map[state.ObjID]bool{mine: true}},
		{"sameName+Other", map[state.ObjID]bool{namesake: true}},
		{"Self,Card.Equipment+Attached", map[state.ObjID]bool{anchor: true, mine: true}},
	} {
		match := destroyAllTargetedMatcher(g, c.rest, sc)
		for _, id := range []state.ObjID{anchor, namesake, bystander, mine, theirs} {
			if got := match(id); got != c.want[id] {
				t.Errorf("Targeted.%s on %d: got %v, want %v", c.rest, id, got, c.want[id])
			}
		}
	}
	noTarget := destroyAllTargetedMatcher(g, "Equipment+Attached", SpecContext{You: 0, Resolving: true})
	if noTarget(mine) {
		t.Error("with no object target Targeted. must match nothing")
	}
}
