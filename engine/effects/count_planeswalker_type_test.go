package effects

// cr-g222: the `$PlaneswalkerType` property on the Count$Valid<zone> head is
// the planeswalker-subtype sibling of `$CreatureType` (count_type_distinct_
// test.go). Before this the property was unrecognised, so the whole token
// became the spec and matched nothing -- Tam, the Possibility counted zero
// planeswalker types whatever was on the battlefield.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	pwtAjani1  = "Name:Ajani, Fixture One\nTypes:Legendary Planeswalker Ajani\nLoyalty:3\nOracle:x\n"
	pwtAjani2  = "Name:Ajani, Fixture Two\nTypes:Legendary Planeswalker Ajani\nLoyalty:4\nOracle:x\n"
	pwtChandra = "Name:Chandra, Fixture\nTypes:Legendary Planeswalker Chandra\nLoyalty:3\nOracle:x\n"
	pwtRelic   = "Name:Fixture Relic\nTypes:Artifact\nOracle:x\n"
)

// TestEvalCountPlaneswalkerTypeDedups pins the planeswalker-type spelling:
// two Ajanis and one Chandra (distinct names, same Ajani subtype twice) give
// TWO distinct planeswalker types, and a non-planeswalker artifact adds
// nothing.
func TestEvalCountPlaneswalkerTypeDedups(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZBattlefield, pwtAjani1)
	diffObject(t, g, 0, state.ZBattlefield, pwtAjani2)
	diffObject(t, g, 0, state.ZBattlefield, pwtChandra)
	diffObject(t, g, 0, state.ZBattlefield, pwtRelic)
	// PRECONDITION: all four fixtures really are on seat 0's battlefield,
	// the plain Valid count sees the three planeswalkers (so the fold has
	// candidates to visit), and the artifact is the non-planeswalker.
	if got := len(g.Zone(state.ZBattlefield, 0)); got != 4 {
		t.Fatalf("battlefield holds %d permanents, want 4", got)
	}
	if got := EvalCount(h, c, "Count$Valid Planeswalker.YouCtrl"); got != 3 {
		t.Fatalf("Count$Valid Planeswalker.YouCtrl = %d, want 3 (two Ajani, one Chandra)", got)
	}
	if got := EvalCount(h, c, "Count$Valid Artifact.YouCtrl"); got != 1 {
		t.Fatalf("Count$Valid Artifact.YouCtrl = %d, want 1", got)
	}
	const body = "Count$Valid Planeswalker.YouCtrl$PlaneswalkerType"
	if got := EvalCount(h, c, body); got != 2 {
		t.Fatalf("%s = %d, want 2 distinct planeswalker types (Ajani, Chandra); the artifact adds nothing", body, got)
	}
}

// TestEvalCountPlaneswalkerTypeLimitMaxClamps pins the bounded-property
// suffix: PlaneswalkerType must clamp like its CreatureType sibling rather
// than ignore /LimitMax.<n>. Three distinct types clamp to 2.
func TestEvalCountPlaneswalkerTypeLimitMaxClamps(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZBattlefield, pwtAjani1)
	diffObject(t, g, 0, state.ZBattlefield, pwtChandra)
	diffObject(t, g, 0, state.ZBattlefield, "Name:Gideon, Fixture\nTypes:Legendary Planeswalker Gideon\nLoyalty:3\nOracle:x\n")
	// PRECONDITION: three distinct planeswalker types are on the
	// battlefield, so the clamp has something to reduce.
	if got := EvalCount(h, c, "Count$Valid Planeswalker.YouCtrl$PlaneswalkerType"); got != 3 {
		t.Fatalf("unclamped PlaneswalkerType = %d, want 3", got)
	}
	const body = "Count$Valid Planeswalker.YouCtrl$PlaneswalkerType/LimitMax.2"
	if got := EvalCount(h, c, body); got != 2 {
		t.Fatalf("%s = %d, want 2 (3 distinct planeswalker types clamped to the cap)", body, got)
	}
}
