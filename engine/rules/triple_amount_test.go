package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func paramsTriple(sa *cards.SA, key string) bool {
	return sa != nil && sa.Params[key] == "Triple"
}

// TestTifaTriplePumpTriplesTargetsPowerAndToughness resolves the real compiled
// Final Heaven SVar (NumAtt$ Triple | NumDef$ Triple) against fixture
// creatures: a 2/2 becomes 6/6 and a 1/1 becomes 3/3.
func TestTifaTriplePumpTriplesTargetsPowerAndToughness(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Tifa's Limit Break")
	if !ok {
		t.Fatal("Tifa's Limit Break missing from the corpus")
	}
	f := c.Faces[0]
	var sv *cards.SA
	f.EachSVarAbility(func(sa *cards.SA) {
		if sa.API == "Pump" && paramsTriple(sa, "NumAtt") && paramsTriple(sa, "NumDef") {
			sv = sa
		}
	})
	if sv == nil {
		t.Fatal("precondition: no Pump SVar with NumAtt$/NumDef$ Triple")
	}
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	flier := onBoard(t, e, 0, "Name:Flier\nTypes:Creature Bird\nPT:1/1\nOracle:x\n")
	if e.Power(bear) != 2 || e.Toughness(bear) != 2 || e.Power(flier) != 1 || e.Toughness(flier) != 1 {
		t.Fatalf("precondition: sizes bear %d/%d flier %d/%d", e.Power(bear), e.Toughness(bear), e.Power(flier), e.Toughness(flier))
	}
	for _, id := range []state.ObjID{bear, flier} {
		effects.Resolve(e, &effects.Ctx{Controller: 0, SVars: f.SVars, Targets: []state.Target{{Obj: id}}, TargetsOffered: true}, sv)
	}
	if p, to := e.Power(bear), e.Toughness(bear); p != 6 || to != 6 {
		t.Errorf("bear after Final Heaven = %d/%d, want 6/6", p, to)
	}
	if p, to := e.Power(flier), e.Toughness(flier); p != 3 || to != 3 {
		t.Errorf("flier after Final Heaven = %d/%d, want 3/3", p, to)
	}
}

// TestTripleAmountCarrierCensus mirrors TestDoubleAmountCarrierCensus for the
// bare `Triple` P/T amount: every carrier must be a Pump/PumpAll body. Pinned
// at FORGE_REF 76fd7691ab45ab1b42cef19ad04b41bf364501f4: 1 face (Tifa's Limit
// Break). A new Triple carrier must be classified, not left resolving to zero.
func TestTripleAmountCarrierCensus(t *testing.T) {
	const tripleCarrierFiles = 1
	reg := testutil.CorpusRegistry(t)
	var carriers, bad []string
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			seen := false
			classify := func(sa *cards.SA) {
				if !paramsTriple(sa, "NumAtt") && !paramsTriple(sa, "NumDef") {
					return
				}
				seen = true
				if sa.API != "Pump" && sa.API != "PumpAll" {
					bad = append(bad, f.Name+": api:"+sa.API)
				}
			}
			for _, a := range f.Abilities {
				classify(a)
			}
			for _, tr := range f.Triggers {
				classify(tr.Effect)
			}
			f.EachSVarAbility(classify)
			if seen {
				carriers = append(carriers, f.Name)
			}
		}
	}
	if len(bad) > 0 {
		t.Fatalf("Triple P/T amount on an unclassified primitive: %v", bad)
	}
	if len(carriers) != tripleCarrierFiles {
		t.Errorf("Triple P/T carriers = %d, want %d (%v); a FORGE_REF move changes this -- re-measure and update",
			len(carriers), tripleCarrierFiles, carriers)
	}
}
