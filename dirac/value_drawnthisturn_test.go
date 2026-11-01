package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDrawnThisTurn: ExDrawnThisTurn counts the controller's draws this
// turn (gorge CardsDrawnThisTurn), reset as a turn begins.
func TestDrawnThisTurn(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	x := kop.Expr{Kind: kop.ExDrawnThisTurn}
	ctx := sc.s.ctxEntry(&sc.s.res.e)
	ctx.ctrl = 0
	base := sc.s.exprCold(sc.k, &x, ctx)
	sc.s.draw(0)
	sc.s.draw(0)
	sc.s.draw(1)
	if got := sc.s.exprCold(sc.k, &x, ctx); got != base+2 {
		t.Fatalf("drawn %d, want %d", got, base+2)
	}
}
