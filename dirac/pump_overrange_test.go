package dirac

import (
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
