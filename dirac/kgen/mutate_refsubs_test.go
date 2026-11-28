package kgen

import (
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterRefSubsWalksMutateTgt pins CR 702.140a's "non-Human" through the
// matchup remap: the builder keeps only the subtypes refSubs returns, so a
// mutate target filter it never sees loses Human from every card's mask and
// the mutating cast offers a Human.
func TestFilterRefSubsWalksMutateTgt(t *testing.T) {
	pt := &poolTables{filters: []kop.Filter{{}, {NotSub: 3}}} // subtype id 2, as "Human"
	d := &kop.CardDef{HasMutate: true, MutateTgt: kop.TargetSpec{Kind: kop.TgtObject, Filter: 1}}
	if got := refSubs(pt, d, nil); !slices.Equal(got, []uint8{2}) {
		t.Fatalf("refSubs = %v, want [2] (the mutate target's negated subtype)", got)
	}
}

// TestFilterRefSubsWalksEveryCardDefFilter sets each FilterID a CardDef holds
// outside its abilities, one at a time, to a filter naming a subtype, and
// wants refSubs to return it: a field the walk misses is one whose subtype the
// matchup drops.
func TestFilterRefSubsWalksEveryCardDefFilter(t *testing.T) {
	pt := &poolTables{filters: []kop.Filter{{}, {Sub: 3}}}
	paths := filterIDPaths(reflect.TypeOf(kop.CardDef{}), nil)
	if len(paths) < 10 {
		t.Fatalf("found %d FilterID fields in CardDef, want the card-level costs, targets and filters", len(paths))
	}
	for _, p := range paths {
		d := &kop.CardDef{}
		f := reflect.ValueOf(d).Elem()
		name := ""
		for _, i := range p {
			name += "." + f.Type().Field(i).Name
			f = f.Field(i)
		}
		f.SetUint(1)
		if got := refSubs(pt, d, nil); !slices.Equal(got, []uint8{2}) {
			t.Errorf("CardDef%s: refSubs = %v, want [2]", name, got)
		}
	}
}

// TestFilterRefSubsWalksGrantedAndOverload: the abilities a card only grants
// and its overloaded program carry filters of their own.
func TestFilterRefSubsWalksGrantedAndOverload(t *testing.T) {
	pt := &poolTables{
		filters: []kop.Filter{{}, {Sub: 3}},
		conds:   []kop.Cond{{}}, // the "no condition" slot every ability names
	}
	granted := &kop.CardDef{Granted: []kop.Ability{{Static: kop.Static{Filter: 1}}}}
	if got := refSubs(pt, granted, nil); !slices.Equal(got, []uint8{2}) {
		t.Errorf("Granted: refSubs = %v, want [2]", got)
	}
	cost := &kop.CardDef{Abilities: []kop.Ability{{Cost: kop.Cost{Discard2F: 1}}}}
	if got := refSubs(pt, cost, nil); !slices.Equal(got, []uint8{2}) {
		t.Errorf("Cost.Discard2F: refSubs = %v, want [2]", got)
	}
	over := &kop.CardDef{OverloadProg: []kop.Instr{{Op: kop.OpDestroyAll, F: 1}}}
	if got := refSubs(pt, over, nil); !slices.Equal(got, []uint8{2}) {
		t.Errorf("OverloadProg: refSubs = %v, want [2]", got)
	}
}

// filterIDPaths is the field index path of every FilterID reachable from typ
// through struct fields alone (slices are walked by their own tests).
func filterIDPaths(typ reflect.Type, prefix []int) [][]int {
	var out [][]int
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		p := append(slices.Clone(prefix), i)
		switch {
		case f.Type == reflect.TypeOf(kop.FilterID(0)):
			out = append(out, p)
		case f.Type.Kind() == reflect.Struct:
			out = append(out, filterIDPaths(f.Type, p)...)
		}
	}
	return out
}
