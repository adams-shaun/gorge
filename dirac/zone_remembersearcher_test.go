package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRememberSearcherCounts: kop.ExtRememberSearcher adds the searcher to the
// remembered count (ExRememberedCount) without putting an object in the set,
// and ClearRemembered empties it again.
func TestRememberSearcherCounts(t *testing.T) {
	mark := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtRememberSearcher))}
	sc, _, _ := imprintScen(t, []kop.Instr{mark})
	s := sc.s
	if got := remCount(sc); got != 0 {
		t.Fatalf("precondition: remembered count %d before the marker", got)
	}
	s.runProg(sc.k)
	if got := remCount(sc); got != 1 || s.g.nrem != 0 {
		t.Fatalf("remembered count %d with %d objects, want 1 and 0", got, s.g.nrem)
	}
	clear := kop.Instr{Op: kop.OpClearRemembered}
	s.exec(sc.k, &clear)
	if got := remCount(sc); got != 0 {
		t.Fatalf("after ClearRemembered: count %d, want 0", got)
	}
}
