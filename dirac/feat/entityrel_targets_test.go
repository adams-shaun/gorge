package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// An entry whose multi-object slots name more objects than the old fixed
// seen-set held (16) counts each of them once, and does not run off the set.
func TestBuildRelCountsEveryObjectAnEntryTargets(t *testing.T) {
	var e Encoder
	e.v.seat = 0
	se := &e.kp.Stack[0]
	e.kp.NStack = 1
	se.Ctrl = 1
	n := 0
	for i := range se.Targets {
		se.Targets[i] = dirac.TCode(n)
		n++
	}
	for i := 0; i < 20; i++ {
		se.Extra[i] = dirac.TCode(n)
		n++
	}
	se.NExtra = 20
	se.Extra[se.NExtra] = 0 // beyond NExtra: not read
	e.v.stack = []vstack{{cv: -1}}
	for o := 0; o < n; o++ {
		e.v.cards = append(e.v.cards, vcard{obj: dirac.Obj(o), zone: zHand})
	}
	e.buildRel(nil)
	for _, c := range e.v.cards {
		if c.rel.tgtOpp != 1 || c.rel.tgtMe != 0 {
			t.Fatalf("object %d: targeted by an opponent %d times, by the viewer %d, want 1 and 0", c.obj, c.rel.tgtOpp, c.rel.tgtMe)
		}
	}
}
