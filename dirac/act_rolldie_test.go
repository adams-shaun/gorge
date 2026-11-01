package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRollDie: OpRollDie draws once from the engine rng exactly as gorge's
// effRollDice (h.Rand(sides) + 1, plus the modifier) and keeps the result
// as ExRollResult.
func TestRollDie(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	for i := 0; i < 20; i++ {
		want := sc.s.rng // a copy: the same PCG state
		d0 := sc.s.rng.Draws
		in := kop.Instr{Op: kop.OpRollDie, A: kop.Const(20), B: kop.Const(2)}
		sc.s.exec(sc.k, &in)
		w := want.IntN(20) + 1 + 2
		if sc.s.rng.Draws != d0+1 {
			t.Fatalf("roll %d drew %d times, want 1", i, sc.s.rng.Draws-d0)
		}
		x := kop.Expr{Kind: kop.ExRollResult}
		if got := sc.s.exprCold(sc.k, &x, evalCtx{}); got != w {
			t.Fatalf("roll %d: result %d, want %d", i, got, w)
		}
	}
}

// TestFlipCoinDraw: kgen's api:FlipCoin is OpRollDie with two sides; gorge
// effFlipCoin's heads (a win) is h.Rand(2) == 0, which must be result 1
// (the win branch's guard), tails result 2, one rng draw per flip.
func TestFlipCoinDraw(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	heads, tails := 0, 0
	for i := 0; i < 32; i++ {
		want := sc.s.rng
		d0 := sc.s.rng.Draws
		in := kop.Instr{Op: kop.OpRollDie, A: kop.Const(2)}
		sc.s.exec(sc.k, &in)
		win := want.IntN(2) == 0
		x := kop.Expr{Kind: kop.ExRollResult}
		got := sc.s.exprCold(sc.k, &x, evalCtx{})
		if sc.s.rng.Draws != d0+1 || (got == 1) != win || got < 1 || got > 2 {
			t.Fatalf("flip %d: result %d (draws %d), gorge heads=%v", i, got, sc.s.rng.Draws-d0, win)
		}
		if win {
			heads++
		} else {
			tails++
		}
	}
	if heads == 0 || tails == 0 {
		t.Fatalf("32 flips: %d heads %d tails", heads, tails)
	}
}
