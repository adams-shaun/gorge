package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestNewNonbasicLandTypes checks actual corpus faces, not just spelling in
// the choice vocabulary: these later land subtypes must satisfy the Sage's
// predicate and be offered by both Type$ Land and Type$ Nonbasic Land asks.
func TestNewNonbasicLandTypes(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	for _, tc := range []struct{ name, subtype string }{
		{"Omenpath to Naya", "Omenpath"},
		{"Kavaron, Memorial World", "Planet"},
		{"Vector, Imperial Capital", "Town"},
	} {
		t.Run(tc.subtype, func(t *testing.T) {
			o := corpusObject(t, reg, g, tc.name)
			if o.Zone != state.ZBattlefield || o.Face() == nil ||
				!slices.Contains(o.Face().Types, "Land") || !slices.Contains(o.Face().Types, tc.subtype) {
				t.Fatalf("precondition: %s must be a battlefield Land %s; got %+v", tc.name, tc.subtype, o)
			}
			if !MatchesObjectCtx(g, "Land.hasANonBasicLandType", o, SpecContext{You: 0}) {
				t.Errorf("%s (Land %s) did not match hasANonBasicLandType", tc.name, tc.subtype)
			}
		})
	}
}

func TestNewNonbasicLandTypesInChoices(t *testing.T) {
	for _, category := range []string{"Land", "Nonbasic Land"} {
		t.Run(category, func(t *testing.T) {
			h := &chooseTypeHost{}
			h.g = state.NewGame(names(2))
			src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nOracle:x\n"), 0).ID
			d := categoryAsk(t, h, src, &Ctx{Source: src, Controller: 0},
				"SP$ ChooseType | Defined$ You | Type$ "+category)
			labels := optionLabelsOf(d.Options)
			if !slices.IsSorted(labels) || hasNotePrefix(&h.fakeHost, "ChooseType Type$") {
				t.Fatalf("%s must offer sorted land types without a fallback Note: %v", category, labels)
			}
			for _, subtype := range []string{"Omenpath", "Planet", "Town"} {
				if !slices.Contains(labels, subtype) {
					t.Errorf("%s choices omit %s: %v", category, subtype, labels)
				}
			}
			if category == "Nonbasic Land" && slices.Contains(labels, "Forest") {
				t.Errorf("Nonbasic Land choices include a basic type: %v", labels)
			}
		})
	}
}
