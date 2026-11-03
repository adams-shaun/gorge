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
