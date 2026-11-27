package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestReturnCtrlCounterKindIsNotAttachW: an OpReturnEx B.N is the counter
// kind when A carries counters, and every kind of 128 or more sets
// RetAttachW (bit 7) as a coincidence. That must not hide the W player
// control override (GainControl$ Player.IsRemembered): only the AttachedTo$
// bearer form carries RetAttachTo beside RetAttachW.
func TestReturnCtrlCounterKindIsNotAttachW(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = []kop.Expr{{Kind: kop.ExRememberedCount}} // any expression turns the generated interpreter on
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	o := sc.grave(v.GrizzlyBears, 1, 1)[0] // owned by seat 1
	sc.s.res.e.ctrl = 0
	sc.s.res.e.tgt[0] = TCode(o)
	in := &kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefTarget0, C: int16(kop.ZoneBattlefield),
		A: kop.Const(1), B: kop.Const(int(kop.CtrVow)), W: kop.RefController}

	if got := sc.s.objs[o].owner(); got != 1 {
		t.Fatalf("setup: owner = %d, want seat 1", got)
	}
	if int(kop.CtrVow)&kop.RetAttachW == 0 || int(kop.CtrVow)&kop.RetAttachTo != 0 {
		t.Fatalf("setup: counter kind %d does not set RetAttachW alone; the test is vacuous", kop.CtrVow)
	}
	if sc.s.execReturn(sc.k, in, NoSlot) {
		t.Fatal("execReturn asked for an Aura host")
	}
	if !sc.onBF(o) {
		t.Fatal("the card did not return")
	}
	if got := sc.s.perms[sc.slot(o)].ctrl; got != 0 {
		t.Fatalf("the returned card is controlled by %d, want the W player 0 (owner is 1)", got)
	}
}
