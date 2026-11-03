package dirac

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPumpBelowToughnessRange: a pump of -0/-99999 (a kill spell written as a
// pump) leaves the int16 end-of-turn range but the creature's toughness stays
// at zero or less, so the amount is saturated rather than stalling the game;
// a power amount out of range, or a creature the saturated amount would not
// kill, still stalls loudly.
func TestPumpBelowToughnessRange(t *testing.T) {
	sc, c := actProliferateScen(t)
	bear := sc.perm(c.bear, 0)
	sl := sc.slot(bear)
	if !sc.s.eotAdd(sc.k, sl, 0, -99999) || sc.s.Stalled() {
		t.Fatalf("-0/-99999 on a 2/2 stalled (%d)", sc.s.g.stalled)
	}
	if _, tough := sc.s.pt(sc.k, sl); tough > 0 {
		t.Fatalf("the pumped bear has toughness %d, want 0 or less", tough)
	}

	sc, c = actProliferateScen(t)
	sl = sc.slot(sc.perm(c.bear, 0))
	if sc.s.eotAdd(sc.k, sl, -99999, 0) || sc.s.g.stalled != StallPT {
		t.Fatalf("a power of -99999 did not stall (%d)", sc.s.g.stalled)
	}

	sc, c = actProliferateScen(t)
	sl = sc.slot(sc.perm(c.bear, 0))
	p := &sc.s.perms[sl]
	p.setCtrN(sc.s, int(kop.CtrP1P1), 40000) // 40002 toughness: -32768 does not kill it
	if sc.s.eotAdd(sc.k, sl, 0, -99999) || sc.s.g.stalled != StallPT || p.eotT != 0 {
		t.Fatalf("a pump that would not kill stalled=%d eotT=%d, want a StallPT stall and no change", sc.s.g.stalled, p.eotT)
	}
}

// TestAttackPumpOf9999Fits: Jumbo Cactuar's +9999/+0 on attack is a plain end-of-turn
// pump, and a second one in the turn stacks on it (CR 611.2a).
func TestAttackPumpOf9999Fits(t *testing.T) {
	sc, c := actProliferateScen(t)
	sl := sc.slot(sc.perm(c.bear, 0))
	for i := 0; i < 2; i++ {
		if !sc.s.eotAdd(sc.k, sl, 9999, 0) || sc.s.Stalled() {
			t.Fatalf("pump %d of +9999/+0 stalled (%d)", i+1, sc.s.g.stalled)
		}
	}
	if pw, _ := sc.s.pt(sc.k, sl); pw != 2+2*9999 {
		t.Fatalf("power %d, want %d", pw, 2+2*9999)
	}
}

// TestOverrangePumpTable: a pump past int8 keeps its true amount in the small
// table, saturated inline; the amount stacks and reads back exactly, it is
// dropped when the pump returns to range and at cleanup, and a fifth
// permanent past the table stalls (StallPT) and changes nothing.
func TestOverrangePumpTable(t *testing.T) {
	sc, c := actProliferateScen(t)
	sl := sc.slot(sc.perm(c.bear, 0))
	p := &sc.s.perms[sl]
	if !sc.s.eotAdd(sc.k, sl, 300, -200) || sc.s.nptx != 1 || p.eotP != math.MaxInt8 || p.eotT != math.MinInt8 {
		t.Fatalf("+300/-200: nptx=%d inline %d/%d", sc.s.nptx, p.eotP, p.eotT)
	}
	if pw, tg := sc.s.pt(sc.k, sl); pw != 302 || tg != -198 {
		t.Fatalf("pt %d/%d, want 302/-198", pw, tg)
	}
	if !sc.s.eotAdd(sc.k, sl, -300, 200) || sc.s.nptx != 0 || p.eotP != 0 || p.eotT != 0 {
		t.Fatalf("back in range: nptx=%d inline %d/%d", sc.s.nptx, p.eotP, p.eotT)
	}
	sc.s.eotAdd(sc.k, sl, 9999, 0)
	sc.s.finishCleanup(sc.k)
	if sc.s.nptx != 0 || p.eotP != 0 {
		t.Fatalf("after cleanup nptx=%d eotP=%d", sc.s.nptx, p.eotP)
	}
	for i := 0; i < maxPtX; i++ {
		sc.s.ptx[i] = ptRepX{sl: Slot(100 + i), p: 200}
	}
	sc.s.nptx = maxPtX
	if sc.s.eotAdd(sc.k, sl, 500, 0) || sc.s.g.stalled != StallPT || p.eotP != 0 {
		t.Fatalf("a fifth overrange permanent stalled=%d eotP=%d, want StallPT and no change", sc.s.g.stalled, p.eotP)
	}
}
