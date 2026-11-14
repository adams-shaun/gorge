package dirac

import (
	"reflect"
	"testing"
	"unsafe"
)

// Two synthetic cold features, each its own declaration as two feature files
// would make them, in a registry of their own so the package's layout does
// not carry them.
var testColdRegs coldRegistry

var testColdA = coldRegister[[3]uint16](&testColdRegs, "synthA", func(g *genPrep) int { return 5 })

var testColdB = coldRegister[uint64](&testColdRegs, "synthB", func(g *genPrep) int {
	if g.anyAtkLast {
		return 2
	}
	return 0
})

// TestColdFeaturesRegisterIndependently: two regions registered from separate
// declarations both land in the layout, apart and aligned, with nothing of
// coldLayout, prepCold or place naming them; and the layout is the same
// whichever order they registered in.
func TestColdFeaturesRegisterIndependently(t *testing.T) {
	g := &genPrep{anyAtkLast: true}
	lay := func(r *coldRegistry) (l coldLayout) {
		r.prep(g, &l)
		r.size(&l)
		return l
	}
	l := lay(&testColdRegs)

	// the same two, registered the other way round
	var rev coldRegistry
	b := coldRegister[uint64](&rev, "synthB", testColdB.need)
	a := coldRegister[[3]uint16](&rev, "synthA", testColdA.need)
	r := lay(&rev)
	if l.n[testColdA.pos] != r.n[a.pos] || l.off[testColdA.pos] != r.off[a.pos] || l.n[testColdB.pos] != r.n[b.pos] || l.off[testColdB.pos] != r.off[b.pos] || l.words != r.words {
		t.Fatalf("registration order reached the layout: %+v vs %+v", l, r)
	}

	// the wider alignment first, then by key: synthB's 2 words, then synthA's
	// 5*6 bytes, 46 bytes in all
	if l.off[testColdB.pos] != 0 || l.off[testColdA.pos] != 16 || l.words != 6 {
		t.Fatalf("synthB at %d, synthA at %d, %d words; want 0, 16, 6", l.off[testColdB.pos], l.off[testColdA.pos], l.words)
	}

	var s State
	s.cold.layout = &l
	s.cold.buf = make([]uint64, l.words)
	av, bv := testColdA.of(&s), testColdB.of(&s)
	if len(av) != 5 || len(bv) != 2 {
		t.Fatalf("views of %d and %d, want 5 and 2", len(av), len(bv))
	}
	for i := range bv {
		bv[i] = ^uint64(0)
	}
	for i := range av {
		if av[i] != [3]uint16{} {
			t.Fatalf("synthB's writes reached synthA[%d]", i)
		}
		av[i] = [3]uint16{1, 2, 3}
	}
	for i := range bv {
		if bv[i] != ^uint64(0) {
			t.Fatalf("synthA's writes reached synthB[%d]", i)
		}
	}

	// a region the table does not need takes no bytes, not even padding
	g.anyAtkLast = false
	l = lay(&testColdRegs)
	if l.n[testColdB.pos] != 0 || l.off[testColdA.pos] != 0 || l.words != 4 {
		t.Fatalf("without synthB: synthA at %d, %d words; want 0, 4", l.off[testColdA.pos], l.words)
	}
	if s.cold.layout = &l; testColdB.of(&s) != nil {
		t.Fatal("an unneeded region has a view")
	}
}

// TestColdViewsAreRegistered: every view coldArrays keeps is bound by a
// registered region, so layOut points it into the buffer.
func TestColdViewsAreRegistered(t *testing.T) {
	coldRegs.seal()
	var l coldLayout
	for _, d := range coldRegs.descs {
		l.n[d.pos] = 1
	}
	l.size()
	var s State
	s.layOut(&l)
	c := reflect.ValueOf(&s.cold).Elem()
	base := uintptr(unsafe.Pointer(unsafe.SliceData(s.cold.buf)))
	for i := range c.NumField() {
		f := c.Type().Field(i)
		if f.Name == "buf" || f.Type.Kind() != reflect.Slice {
			continue
		}
		v := c.Field(i)
		if v.Len() != 1 || v.Pointer() < base || v.Pointer() >= base+uintptr(8*l.words) {
			t.Errorf("coldArrays.%s is not a registered region's view", f.Name)
		}
	}
}
