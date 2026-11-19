package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Two copies of a card are twins for the fee-only prefix search only while
// nothing sets them apart: gorge's scriptTwin compares the activate options
// the decision offers, so a source that lost its abilities (an override
// record) is no twin of one that did not, and the search must still walk the
// usable one.
func TestFiltTwinSlotFailsOpenOnAnOverride(t *testing.T) {
	sc, id := fixedTapsScen(t, []cardset.Entry{{Name: "Grizzly Bears", Count: 1}, {Name: "Forest", Count: 20}})
	a, b := sc.untappedPerm(id["Forest"], 0), sc.untappedPerm(id["Forest"], 0)
	sa, sb := sc.slot(a), sc.slot(b)
	fs := &filtSearch{}
	if !fs.twinSlot(sc.k, sc.s, sa, sb) {
		t.Fatal("precondition: two untapped Forests are twins")
	}
	sc.s.perms[sb].ovr.flags |= ovLoseAll
	if fs.twinSlot(sc.k, sc.s, sa, sb) {
		t.Fatal("a Forest that lost its abilities is no twin of one that did not")
	}
}
