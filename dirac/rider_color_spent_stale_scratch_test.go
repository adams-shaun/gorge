package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestRiderColorSpentStaleScratch: riderColorSpent reads arm B of the
// shared scratch region only while a build is live. finishBuild does not
// zero the region, so a finished any-colour may-play cast (seSpell over the
// object with bflAnyColor set) leaves its bytes there, and a price over the
// same object taken outside a build must not fold the pips under that
// stale rider (plannedCastCostOf's plannedUnderRider arm): the record is
// spent, mayPlayIgnoreColor no longer holds it, and the mid-cast price is
// the only caller that ever sees a live build.
func TestRiderColorSpentStaleScratch(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	o := sc.obj(v.ThinkTwice, 0)
	s.objs[o].zone = kop.ZoneExile

	// with the record standing the may-play cast's pip is folded (the
	// unconsumed record's rider, mayPlayIgnoreColor): the arm this test
	// observes is reachable
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn), kind: kop.MayPlayIgnoreColor}
	s.g.nmay = 1
	folded, ok := s.plannedCastCostOf(sc.k, 0, o, castMayPlay, false, false)
	if !ok {
		t.Fatal("plannedCastCostOf did not price the may-play cast")
	}
	if folded.Generic != 2 || folded.Pips != [kop.NumManaKinds]uint8{} {
		t.Fatalf("the standing rider did not fold the pip: %+v, want {2} with no pips", folded)
	}

	// the cast spends the record and the build keeps bflAnyColor
	// (plannedUnderRider's mid-cast price); finishing the cast leaves the
	// bytes in the scratch region
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: v.ThinkTwice, ctrl: 0}}
	s.castFlagsGen(sc.k, o, castMayPlay)
	if s.g.nmay != 0 {
		t.Fatalf("the any-colour permission was not spent (%d records left)", s.g.nmay)
	}
	if s.bld().bfl&bflAnyColor == 0 {
		t.Fatal("the spent any-colour record did not set bflAnyColor")
	}
	if s.buildLive() {
		t.Fatal("precondition: the scratch must be stale (no build live) for the later price")
	}

	// a later offer over the same object is priced outside a build: the
	// spent rider is gone and the finished build's bytes must not fold the pip
	cost, ok := s.plannedCastCostOf(sc.k, 0, o, castMayPlay, false, false)
	if !ok {
		t.Fatal("plannedCastCostOf did not price the later offer")
	}
	if cost.Pips[kop.U] != 1 || cost.Generic != 1 {
		t.Fatalf("the finished build's stale rider folded the later offer's pip: %+v, want the printed {1}{U}", cost)
	}
}
