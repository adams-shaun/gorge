package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsDeferSeenKeyKeepsRestrictedColours: a Taps-deferred restricted gap
// (restrCensusTapsGapShape, a Primal Beyond) answers its activation ask once
// per restricted colour, and restrCensusDeferGap floats that colour through
// floatRestrAb, which puts the unit in State.rb and leaves the raw pool
// untouched. gorge's scriptStateKey hashes Players[p].Pool, which counts every
// restricted unit with its colour, and ManaUnits, so two answers that differ
// only in the restricted colour are distinct states and both subtrees are
// visited. The deferred search's seenSet hashed the raw pool alone, so the
// second answer looked like a duplicate of the first and its subtree was
// dropped: an undercount of the 160-node budget (dirac offers what gorge
// withholds).
func TestTapsDeferSeenKeyKeepsRestrictedColours(t *testing.T) {
	var st State
	st.pl[0].pool[kop.C] = 1 // the ordinary pool is the same at every node
	seen := &deferSeen{}
	used := []int8{0} // gap 0 tapped on every path: the used set does not tell the answers apart
	for _, c := range []int{kop.W, kop.U, kop.B, kop.R, kop.G} {
		var n [kop.NumManaKinds]uint8
		n[c] = 1
		st.rb[0] = [maxRB]rbatch{{kind: 1, n: n}} // one restricted unit of this colour
		if got := st.poolProj(0); got[c] != st.pl[0].pool[c]+1 {
			t.Fatalf("precondition: colour %d's restricted unit is not in poolProj (%v)", c, got)
		}
		if deferDuplicate(true, false, seen, &st, 0, used) {
			t.Fatalf("colour %d: restricted mana of a different colour was dropped as a duplicate", c)
		}
	}
}
