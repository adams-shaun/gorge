package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHybridNWitnessOrder: the planner witness of a three-pip cost is the
// first resolution with pip 0 the fastest varying (gorge planHybridCost).
// {W/U}{W/U}{W/B} over W W U B: all-first {W}{W}{W} is out of reach, so the
// first affordable mask is bit 0 (pip 0 takes U): {U}{W}{W}. A witness that
// varied pip 2 fastest would reach {W}{W}{B} first.
func TestHybridNWitnessOrder(t *testing.T) {
	cost := kop.ManaCost{Hyb: [4]kop.ColorMask{kop.ColW | kop.ColU, kop.ColW | kop.ColU, kop.ColW | kop.ColB}}
	var a avail
	a.addUnits(uint8(kop.ColW), 2)
	a.addUnits(uint8(kop.ColU), 1)
	a.addUnits(uint8(kop.ColB), 1)
	got := a.hybWitness(cost, 0)
	want := kop.ManaCost{}
	want.Pips[kop.W], want.Pips[kop.U] = 2, 1
	if got != want {
		t.Errorf("witness %+v, want %+v", got, want)
	}
	DebugHybridNPipOrder = true
	rev := a.hybWitness(cost, 0)
	DebugHybridNPipOrder = false
	if rev == got {
		t.Errorf("the reversed enumeration found the same witness: the case no longer separates the orders")
	}
}

// TestHybridNCosts: four-pip costs afford, resolve, count and pack their
// pip answers without allocating.
func TestHybridNCosts(t *testing.T) {
	cost := kop.ManaCost{Hyb: [4]kop.ColorMask{kop.ColW | kop.ColG, kop.ColU | kop.ColG, kop.ColB | kop.ColG, kop.ColR | kop.ColG}, HybOrd: 3}
	if n := hybN(&cost); n != 4 {
		t.Errorf("hybN = %d, want 4", n)
	}
	if mv := cost.MV(); mv != 4 {
		t.Errorf("MV = %d, want 4", mv)
	}
	r := cost.Resolve(kop.G)
	if r.Hyb != [4]kop.ColorMask{kop.ColU | kop.ColG, kop.ColB | kop.ColG, kop.ColR | kop.ColG} || r.HybOrd != 1 || r.Pips[kop.G] != 1 || hybN(&r) != 3 {
		t.Errorf("Resolve(G): %+v", r)
	}
	var a avail
	a.addUnits(uint8(kop.ColG), 4)
	if !a.affords(&cost, pipMaskOf(&cost), 0) || !a.affordsAnyColor(&cost, 0) {
		t.Error("four Forests do not pay {G/W}{G/U}{B/G}{R/G}")
	}
	var b avail
	b.addUnits(uint8(kop.ColW), 4)
	if b.affords(&cost, pipMaskOf(&cost), 0) {
		t.Error("four Plains pay {G/W}{G/U}{B/G}{R/G}")
	}
	if n := testing.AllocsPerRun(20, func() {
		a.affords(&cost, pipMaskOf(&cost), 0)
		a.affordsAnyColor(&cost, 0)
		a.hybWitness(cost, 0)
	}); n != 0 {
		t.Errorf("%.0f allocations, want 0", n)
	}

	var bl build
	cols := [4]uint8{kop.G, kop.U, kop.B, kop.R}
	for i, c := range cols {
		bl.setPipCol(uint8(i), c)
	}
	bl.setPipCol(1, kop.W) // an answer is overwritten, not or-ed
	cols[1] = kop.W
	for i, c := range cols {
		if got := bl.pipCol(uint8(i)); got != int(c) {
			t.Errorf("pipCol(%d) = %d, want %d", i, got, c)
		}
	}
}
