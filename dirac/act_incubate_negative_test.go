package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestIncubateNegativeMintsNoCounterEvent: a negative A clamps to zero (CR
// 107.1b) and still mints the Incubator token (CR 701.53a), with no counter
// event -- gorge effIncubate's clamp (Bloated Processor at negative LKI power).
func TestIncubateNegativeMintsNoCounterEvent(t *testing.T) {
	sc, front := incubateScen(t)
	sc.incubate(t, front, -1, 1)
	toks := sc.incubatorTokens(front)
	if len(toks) != 1 {
		t.Fatalf("%d Incubator tokens, want 1 (CR 701.53a + 107.1b)", len(toks))
	}
	if n := sc.s.perms[sc.slot(toks[0])].ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("+1/+1 count %d, want 0", n)
	}
	if sc.s.npend != 0 {
		t.Fatalf("%d pending triggers, want none (no counter event)", sc.s.npend)
	}
}
