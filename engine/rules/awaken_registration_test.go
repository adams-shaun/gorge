package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kw:Awaken is an altCastModes row plus cards.kwAwaken's rider; the corpus
// reader must see the carriers as supported.
func TestAwakenCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Coastal Discovery", "Clutch of Currents", "Earthen Arms"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
	// Awaken stamps the shared FlagAltCostPaid; the Condition$ Awakened read
	// is unambiguous only while no Awaken face also reads Count$AltCost or
	// carries another stamping keyword.
	carriers := 0
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if _, ok := f.KeywordParam("Awaken"); !ok {
				continue
			}
			carriers++
			if faceWantsAltCost(f) {
				t.Errorf("%s carries Awaken and reads Count$AltCost: FlagAltCostPaid would be ambiguous", f.Name)
			}
			if _, ok := f.KeywordParam("Prowl"); ok {
				t.Errorf("%s carries Awaken and Prowl: both stamp FlagAltCostPaid", f.Name)
			}
			if _, ok := f.KeywordParam("Freerunning"); ok {
				t.Errorf("%s carries Awaken and Freerunning: both stamp FlagAltCostPaid", f.Name)
			}
			if awakenLink(f) == nil {
				t.Errorf("%s carries Awaken but its spell ability chain has no awaken link", f.Name)
			}
		}
	}
	if carriers < 15 {
		t.Fatalf("precondition: %d Awaken faces in the corpus, want at least 15", carriers)
	}
}
