package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestReturnEntersWithCounterPair: an OpReturnEx whose A carries a second
// counter kind (kop.RetCounterPairShift; WithCountersType$ with two kinds)
// returns the card with one counter of each, both as it enters (CR 122.6),
// and leaves no pending second kind behind for the next entrant.
func TestReturnEntersWithCounterPair(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = []kop.Expr{{Kind: kop.ExRememberedCount}} // any expression turns the generated interpreter on
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	bear := v.GrizzlyBears
	g := sc.grave(bear, 0, 1)[0]
	sc.s.res.e.obj, sc.s.res.e.ctrl = g, 0
	sc.s.res.e.tgt[0] = uint8(g)
	in := &kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefTarget0, C: int16(kop.ZoneBattlefield),
		A: kop.Const(1 | (int(kop.CtrIndestructible)+1)<<kop.RetCounterPairShift), B: kop.Const(int(kop.CtrHexproof))}
	if sc.s.objs[g].zone != kop.ZoneGraveyard {
		t.Fatalf("setup: the card is in zone %d, want the graveyard", sc.s.objs[g].zone)
	}
	if sc.s.execReturn(sc.k, in, NoSlot) {
		t.Fatal("execReturn asked for an Aura host")
	}
	if !sc.onBF(g) {
		t.Fatal("the card did not return")
	}
	p := sc.s.objs[g].slot
	for _, c := range []kop.CounterKind{kop.CtrHexproof, kop.CtrIndestructible} {
		if n := sc.s.perms[p].ctrN(sc.s, int(c)); n != 1 {
			t.Errorf("counters of kind %d: %d, want 1", c, n)
		}
	}
	if e := sc.s.g.enterCtr; e.n != 0 || e.kind2 != 0 {
		t.Errorf("entry counters left armed: %+v", e)
	}

	// the single-kind form is unchanged: one counter, no second kind
	h := sc.grave(bear, 0, 1)[0]
	sc.s.res.e.tgt[0] = uint8(h)
	one := &kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefTarget0, C: int16(kop.ZoneBattlefield), A: kop.Const(1), B: kop.Const(int(kop.CtrHexproof))}
	sc.s.execReturn(sc.k, one, NoSlot)
	q := sc.s.objs[h].slot
	if a, b := sc.s.perms[q].ctrN(sc.s, int(kop.CtrHexproof)), sc.s.perms[q].ctrN(sc.s, int(kop.CtrIndestructible)); a != 1 || b != 0 {
		t.Errorf("single kind: hexproof %d indestructible %d, want 1 and 0", a, b)
	}
}

// TestReturnCounterPairMergeWindow: with entry-counter merging on (a table
// with an AddCounter replacement, entryWinOpen's path), the pair's second
// kind is placed through the window too: neither kind merges, so each is its
// own put, and the window leaves no second kind armed.
func TestReturnCounterPairMergeWindow(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = []kop.Expr{{Kind: kop.ExRememberedCount}}
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if sc.k.gen == nil {
		t.Fatal("setup: no generated interpreter")
	}
	sc.k.gen.anyEntryMerge = true
	g := sc.grave(v.GrizzlyBears, 0, 1)[0]
	sc.s.res.e.obj, sc.s.res.e.ctrl = g, 0
	sc.s.res.e.tgt[0] = uint8(g)
	in := &kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefTarget0, C: int16(kop.ZoneBattlefield),
		A: kop.Const(1 | (int(kop.CtrIndestructible)+1)<<kop.RetCounterPairShift), B: kop.Const(int(kop.CtrHexproof))}
	sc.s.execReturn(sc.k, in, NoSlot)
	if !sc.onBF(g) {
		t.Fatal("the card did not return")
	}
	p := sc.s.objs[g].slot
	for _, c := range []kop.CounterKind{kop.CtrHexproof, kop.CtrIndestructible} {
		if n := sc.s.perms[p].ctrN(sc.s, int(c)); n != 1 {
			t.Errorf("counters of kind %d: %d, want 1", c, n)
		}
	}
	if e := sc.s.g.enterCtr; e.n != 0 || e.kind2 != 0 {
		t.Errorf("entry counters left armed: %+v", e)
	}
}
