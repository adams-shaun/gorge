package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestOutsideTypesHaveDistinctCodes: two chosen creature types outside the
// matchup's subtypes get different perm.chosen codes (so two sources that
// chose different outside types compare exactly instead of stalling as
// outsidetype), while the same type maps to the same code.
func TestOutsideTypesHaveDistinctCodes(t *testing.T) {
	sc, _ := newObjScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	var outs []int
	for v, si := range sc.k.t.TypeVocab {
		if si == 255 {
			outs = append(outs, v)
		}
	}
	if len(outs) < 2 {
		t.Skip("fewer than two outside creature types")
	}
	a, b := typeVocabSub(sc.k, outs[0]), typeVocabSub(sc.k, outs[1])
	if !outsideSub(sc.k, a) || !outsideSub(sc.k, b) {
		t.Fatalf("codes %d %d are not outside codes", a, b)
	}
	if a == b {
		t.Fatalf("two different outside types share code %d", a)
	}
	if typeVocabSub(sc.k, outs[0]) != a {
		t.Fatalf("one outside type has two codes")
	}
	if len(sc.k.t.Subtypes) > 0 && outsideSub(sc.k, 0) || outsideSub(sc.k, 255) {
		t.Fatalf("a matchup subtype or none counts as outside")
	}
}
