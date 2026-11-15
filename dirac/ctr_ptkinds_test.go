package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPTCounterKindsAdjustPT: CR 122.1a. Each P/T counter kind adds its own
// power and toughness per counter, as state.CounterPTDelta does for gorge; a
// table that places none of them leaves the counters unread, and a -0/-N pile
// that takes toughness to 0 kills at SBA (CR 704.5f).
func TestPTCounterKindsAdjustPT(t *testing.T) {
	sc, id := honeScen(t)
	s := sc.s
	giant := sc.perm(id["Hill Giant"], 0)
	g := sc.slot(giant)
	chk := func(step string, wp, wt int) {
		t.Helper()
		if p, tt := s.pt(sc.k, g); p != wp || tt != wt {
			t.Errorf("%s: %d/%d, want %d/%d", step, p, tt, wp, wt)
		}
	}
	if sc.k.gen.uses.has(usePTKinds) {
		t.Fatal("usePTKinds set for a table that places no P/T counter kind")
	}
	s.perms[g].addCtrN(s, int(kop.CtrP2P2), 1)
	chk("P2P2 with the gate off", 3, 3)
	sc.k.gen.uses.set(usePTKinds)

	for _, c := range []struct {
		name   string
		kind   kop.CounterKind
		dp, dt int
	}{
		{"P1P0", kop.CtrP1P0, 1, 0},
		{"P0P1", kop.CtrP0P1, 0, 1},
		{"P1P2", kop.CtrP1P2, 1, 2},
		{"P2P2", kop.CtrP2P2, 2, 2},
		{"M0M1", kop.CtrM0M1, 0, -1},
		{"M0M2", kop.CtrM0M2, 0, -2},
		{"M2M2", kop.CtrM2M2, -2, -2},
	} {
		s.perms[g].subCtrN(s, int(kop.CtrP2P2), s.perms[g].ctrN(s, int(kop.CtrP2P2)))
		chk(c.name+" cleared", 3, 3)
		s.perms[g].addCtrN(s, int(c.kind), 1)
		chk(c.name+" x1", 3+c.dp, 3+c.dt)
		s.perms[g].addCtrN(s, int(c.kind), 1)
		chk(c.name+" x2", 3+2*c.dp, 3+2*c.dt)
		s.perms[g].subCtrN(s, int(c.kind), 2)
		chk(c.name+" removed", 3, 3)
	}

	s.perms[g].addCtrN(s, int(kop.CtrP1P1), 1)
	s.perms[g].addCtrN(s, int(kop.CtrM0M2), 1)
	s.perms[g].addCtrN(s, int(kop.CtrP1P0), 2)
	chk("mixed with a +1/+1 counter", 3+1+2, 3+1-2)

	// Toughness 2 after the first -0/-2 stays; the second takes it to 0.
	s.sba(sc.k)
	if !sc.onBF(giant) {
		t.Fatal("Hill Giant (toughness 2) died at SBA")
	}
	s.perms[g].addCtrN(s, int(kop.CtrM0M2), 1)
	chk("two -0/-2 counters", 3+1+2, 3+1-4)
	s.sba(sc.k)
	if sc.onBF(giant) {
		t.Fatal("Hill Giant with toughness 0 from -0/-2 counters is still on the battlefield")
	}
}
