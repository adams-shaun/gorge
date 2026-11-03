package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The capacities below were raised on 2026-11-03, each after a lockstep game of
// realistic play stalled on it; each test plays the shape that stalled past the
// old bound.

// TestGiftedTruceFitsTheEffectPool: a gifted Dawn's Truce gives each permanent
// its controller has hexproof and then indestructible, a keyword-grant record
// per grant (kwGrantFx). Twenty permanents take forty records, past the 32 the
// pool held (std-blb pairs 197 and 206 stalled with seventeen).
func TestGiftedTruceFitsTheEffectPool(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	var sls []Slot
	for i := 0; i < 20; i++ {
		sls = append(sls, sc.slot(sc.perm(v.Plains, 0)))
	}
	for _, m := range []kop.KwMask{kop.KwHexproof, kop.KwIndestructible} {
		for _, sl := range sls {
			sc.s.kwGrantFx(sl, m)
		}
	}
	if sc.s.Stalled() || sc.s.g.nfx != 40 {
		t.Fatalf("40 grants: %d records, stall %q", sc.s.g.nfx, StallName(sc.s.StallReason()))
	}
}

// TestTwentyFourEntriesSnapshotDistinctly: each creature entering beside an
// enters listener snapshots itself into the LKI pool as its trigger is queued
// (enqueueLKI), so twenty Cats from Ajani, Caller of the Pride beside Mentor of
// the Meek hold twenty records at once, past the 18 the pool held (std-fdn pair
// 269).
func TestTwentyFourEntriesSnapshotDistinctly(t *testing.T) {
	sc, id := newRemParamsScen(t)
	s := sc.s
	var cats []Obj
	for i := 0; i < 24; i++ {
		cats = append(cats, sc.perm(id["Savannah Lions"], 0))
	}
	for _, o := range cats {
		s.enqueueLKI(o, s.objs[o].ver, id["Savannah Lions"], 0, 0, o, 0, 0)
	}
	if s.Stalled() || s.npend != 24 {
		t.Fatalf("24 entries: %d pending, stall %q", s.npend, StallName(s.StallReason()))
	}
	for i := 0; i < 24; i++ {
		if s.pend[i].li == 0 {
			t.Fatalf("pending %d has no snapshot", i)
		}
	}
}
