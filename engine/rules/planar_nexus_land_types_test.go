package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPlanarNexusIsEveryNonbasicLandTypeOnly: Planar Nexus's CDA
// (AddType$ AllNonBasicLandType, "is every nonbasic land type") grants the
// nonbasic land types of CR 205.3i and NOT the five basic ones (CR 305.6: a
// basic land type would also grant a mana ability). Forge's snapshot of a
// battlefield Planar Nexus carries no plains/island/swamp/mountain/forest.
func TestPlanarNexusIsEveryNonbasicLandTypeOnly(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg, []*cards.Card{lookup(t, reg, "Planar Nexus")}, nil)
	nx := moveByName(t, e, 0, "Planar Nexus", state.ZBattlefield)

	types := e.Derived(nx).Types
	// Precondition: the expansion ran at all (nonbasic words present).
	for _, want := range []string{"Desert", "Gate", "Locus", "Urza's"} {
		if !slices.Contains(types, want) {
			t.Fatalf("precondition: Planar Nexus lacks %q: %v", want, types)
		}
	}
	for _, basic := range []string{"Plains", "Island", "Swamp", "Mountain", "Forest"} {
		if slices.Contains(types, basic) {
			t.Errorf("Planar Nexus carries basic land type %q: %v", basic, types)
		}
	}
}
