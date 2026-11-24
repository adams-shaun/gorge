package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOppExtLifeLostFigure: ExOppExtreme's OppExtLifeLost reads each living
// member's life lost this turn in seat order (gorge playerCountExtreme's
// LifeLostThisTurn arm): the opponents of the reading controller, or with
// OppExtAllPl every living player; a player who has lost the game is skipped;
// the tally is zero again after a turn start.
func TestOppExtLifeLostFigure(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.s.g.nseat = 4 // a pod: seats 0..3 live
	for p, n := range []int16{9, 2, 7, 5} {
		sc.s.pl[p].lifeLost = n
	}
	read := func(ctrl uint8, n int) int {
		x := &kop.Expr{Kind: kop.ExOppExtreme, N: int16(n)}
		return sc.s.exprCold(sc.k, x, evalCtx{ctrl: ctrl})
	}
	const fig = kop.OppExtLifeLost
	hi, lo, all := fig, fig|kop.OppExtLowest, kop.OppExtAllPl
	type row struct {
		what string
		ctrl uint8
		n    int
		want int
	}
	rows := []row{
		{"highest opponent of seat 0", 0, hi, 7},
		{"lowest opponent of seat 0", 0, lo, 2},
		{"highest opponent of seat 2 (its own 7 is not read)", 2, hi, 9},
		{"lowest opponent of seat 1", 1, lo, 5},
		{"highest of every player, controller included", 1, hi | all, 9},
		{"lowest of every player, controller included", 0, lo | all, 2},
		{"highest opponent of seat 3", 3, hi, 9},
	}
	for _, r := range rows {
		if got := read(r.ctrl, r.n); got != r.want {
			t.Errorf("%s: read %d, want %d", r.what, got, r.want)
		}
	}
	// precondition of the next rows: the walk is over seats, so the figure
	// differs between the opponents-only and every-player groups for seat 0
	if read(0, hi) == read(0, hi|all) {
		t.Fatal("seat 0's highest opponent equals the all-player highest: the group is not what the rows exercise")
	}
	// a seat that lost the game is outside the group
	sc.s.pl[0].lost = true
	if got := read(1, hi); got != 7 {
		t.Errorf("seat 0 out: highest opponent of seat 1 = %d, want 7", got)
	}
	if got := read(1, hi|all); got != 7 {
		t.Errorf("seat 0 out: highest of all = %d, want 7", got)
	}
	sc.s.pl[0].lost = false
	// the turn start zeroes every seat's tally
	sc.s.beginTurn(1)
	if got := read(0, hi|all); got != 0 {
		t.Errorf("after a turn start: %d, want 0", got)
	}
}
