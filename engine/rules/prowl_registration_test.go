package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kw:Prowl and kw:Freerunning are altCastModes rows; the corpus reader must
// see the carriers (and Latchkey Faerie's prowled ETB gate) as supported.
func TestProwlFreerunningCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Auntie's Snitch", "Latchkey Faerie", "Brotherhood Ambushers", "Eagle Vision", "Monastery Raid"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
	// The prowled predicate and Count$Freerunning read FlagAltCostPaid on a
	// face carrying the keyword. That bit names the keyword's cost only while
	// no such face also reads Count$AltCost (faceWantsAltCost's stamp for an
	// S:Mode$ AlternativeCost static); pin the corpus to it.
	carriers := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			_, prowl := f.KeywordParam("Prowl")
			_, free := f.KeywordParam("Freerunning")
			if !prowl && !free {
				continue
			}
			carriers++
			if prowl && free {
				t.Errorf("%s carries both Prowl and Freerunning: either alternative cost stamps the one shared FlagAltCostPaid, so the other keyword's altCostKeywordPaid would read paid", f.Name)
			}
			if faceWantsAltCost(f) {
				t.Errorf("%s carries Prowl/Freerunning and reads Count$AltCost: FlagAltCostPaid would be ambiguous", f.Name)
			}
		}
	}
	if carriers < 20 {
		t.Fatalf("precondition: %d Prowl/Freerunning faces in the corpus, want at least 20", carriers)
	}
}
