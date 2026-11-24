package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestOppMemberExprFigure: ExOppExtreme's OppExtMemberExpr counts the living
// opponents whose own ThisTurnEntered figure reaches the bound, each read
// from that opponent's side (YouCtrl is the opponent), not the table total
// and not the controller's own creatures.
func TestOppMemberExprFigure(t *testing.T) {
	tbl := *v.Table
	tbl.Filters = append(append([]kop.Filter(nil), v.Table.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	fid := kop.FilterID(len(v.Table.Filters))
	tbl.Exprs = []kop.Expr{{Kind: kop.ExThisTurnEntered, N: int16(kop.ZoneBattlefield), F: fid, F4: fid, From: -1}}
	sc := newScenTable(t, &tbl, Config{})
	sc.s.enteredRing()
	enter := func(p uint8) { sc.perm(v.GrizzlyBears, p) } // placing a permanent records its entry
	enter(1)
	enter(1)
	enter(0)
	read := func(ctrl uint8, bound int) int {
		x := &kop.Expr{Kind: kop.ExOppExtreme, N: kop.OppExtCountCmp | kop.OppExtMemberExpr, Op: kop.CmpGE,
			Sub: [2]kop.ValueRef{kop.Const(bound), {K: kop.VExpr, N: 0}}}
		return sc.s.exprCold(sc.k, x, evalCtx{ctrl: ctrl})
	}
	// precondition: three entries, the ledger is what the rows read
	if sc.s.g.enteredN != 3 {
		t.Fatalf("ring holds %d entries, want 3", sc.s.g.enteredN)
	}
	rows := []struct {
		what        string
		ctrl        uint8
		bound, want int
	}{
		{"seat 1 entered two: seat 0 sees an opponent at 2", 0, 2, 1},
		{"seat 0 entered one: seat 1 sees no opponent at 2", 1, 2, 0},
		{"seat 0 entered one: seat 1 sees an opponent at 1", 1, 1, 1},
		{"no opponent entered three", 0, 3, 0},
	}
	for _, r := range rows {
		if got := read(r.ctrl, r.bound); got != r.want {
			t.Errorf("%s: read %d, want %d", r.what, got, r.want)
		}
	}
	DebugOppMemberFromCtrl = true
	defer func() { DebugOppMemberFromCtrl = false }()
	if got := read(1, 2); got == 0 {
		t.Errorf("mutation: seat 1 read from its own side reaches 2 (it has the two creatures), got %d", got)
	}
}
