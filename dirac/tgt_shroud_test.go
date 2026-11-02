package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestShroudTargeting: shroud (CR 702.18a; gorge shroudBlocksTarget) makes
// a permanent an illegal target for every player, its controller included.
func TestShroudTargeting(t *testing.T) {
	sc, a, _ := kwEvasionScen(t)
	sl := sc.slot(a)
	if !sc.s.objTargetable(sc.k, sl, 0) || !sc.s.objTargetable(sc.k, sl, 1) {
		t.Fatal("a creature without shroud is targetable")
	}
	sc.s.perms[sl].eotKw |= kop.KwShroud
	if sc.s.objTargetable(sc.k, sl, 0) || sc.s.objTargetable(sc.k, sl, 1) {
		t.Fatal("shroud: targetable")
	}
}
