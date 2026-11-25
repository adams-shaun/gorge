package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// copyExileGroupScen is the BLB scene and a creature card for the
// CopyPermanent AtEOT$ Exile rider's kernel tests.
func copyExileGroupScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Carrot Cake", Count: 1}, {Name: "Plains", Count: 8}})
	if sc.k.gen == nil {
		t.Fatal("BLB table has no generated extension; the copy entry needs one")
	}
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Types&kop.TCreature != 0 {
			return sc, CardID(i)
		}
	}
	t.Fatal("no creature card in the BLB table")
	return nil, 0
}

// copyExileGroupHead checks n0's record is the head: the tracked next-end-step
// exile of head, sourced on creator.
func copyExileGroupHead(t *testing.T, sc *scen, n0 int, head, creator Obj) delayed {
	t.Helper()
	if got := sc.s.ndly - uint8(n0); got < 1 {
		t.Fatalf("delayed records %d -> %d, want a head", n0, sc.s.ndly)
	}
	d := sc.s.dly[n0]
	if d.act != uint8(kop.OpExile) || d.when != 0 || d.obj != head || d.creator != creator {
		t.Fatalf("head record = %+v, want the tracked next-end-step exile of %d sourced on %d", d, head, creator)
	}
	return d
}

// copyExileGroupFollower checks rec is a follower of head for obj.
func copyExileGroupFollower(t *testing.T, rec delayed, head, obj Obj) {
	t.Helper()
	if rec.act != uint8(kop.OpExile) || rec.when != dlyGrpFollow || rec.creator != head || rec.obj != obj {
		t.Fatalf("follower record = %+v, want a follower of %d for %d", rec, head, obj)
	}
}

// A CopyPermanent token with AtEOT$ Exile over several copies of one
// resolution is one delayed triggered ability naming them all (CR 603.7a/b;
// gorge scheduleAtEOTGroup), so only the first copy's record is a head due at
// the end step; every later copy is a follower (trig_ateotexilegroup.go) and
// the end step queues one trigger. The copies go through the real multi-copy
// path (copyTokensRun and its entry batch), not a hand-set rfCopyGrp.
func TestCopyPermanentExileAtEndGroups(t *testing.T) {
	sc, c := copyExileGroupScen(t)
	src := sc.s.newToken(c, 0)
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: src, srcVer: sc.s.objs[src].ver}, active: true}
	sc.s.res.e.tgt[0], sc.s.res.e.tver[0] = TCode(src), sc.s.objs[src].ver
	in := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: kop.RefTarget0, A: kop.Const(2), B: kop.Const(kop.CpExileAtEnd)}
	if got := sc.s.objRefObj(in.Obj); got != src {
		t.Fatalf("precondition: copy source = %d, want the targeted %d", got, src)
	}

	n0 := int(sc.s.ndly)
	if sc.s.copyTokensRun(sc.k, &in, c, 2) {
		t.Fatal("the two-copy run parked")
	}
	if got := sc.s.ndly - uint8(n0); got != 2 {
		t.Fatalf("delayed records %d -> %d, want the head and one follower", n0, sc.s.ndly)
	}
	head := copyExileGroupHead(t, sc, n0, sc.s.dly[n0].obj, src)
	if head.obj == sc.s.res.lastTok {
		t.Fatal("precondition: the head and the last copy are one object")
	}
	copyExileGroupFollower(t, sc.s.dly[n0+1], head.obj, sc.s.res.lastTok)

	np := sc.s.npend
	sc.s.fireDelayed(sc.k)
	if got := sc.s.npend - np; got != 1 {
		t.Fatalf("the end step queued %d exile triggers, want one group trigger", got)
	}
}

// TestCopyPermanentExileAtEndEntryBatchGroups pins the entry-batch arm of the
// copy rider's grouping: a later copy of the same resolution whose batch
// already holds two entrants joins the first copy's exile promise even before
// rfCopyGrp is set (the copyExiledWith / copyAllEach paths share one entBatch
// without touching that flag).
func TestCopyPermanentExileAtEndEntryBatchGroups(t *testing.T) {
	sc, c := copyExileGroupScen(t)
	src := sc.s.newToken(c, 0)
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: src, srcVer: sc.s.objs[src].ver}, active: true}
	in := kop.Instr{Op: kop.OpCopyAsTokenEx, B: kop.Const(kop.CpExileAtEnd)}
	if got := sc.s.res.flags & rfCopyGrp; got != 0 {
		t.Fatalf("precondition: rfCopyGrp already set (flags %#x)", got)
	}

	n0 := int(sc.s.ndly)
	var eb entBatch
	first := sc.s.newToken(c, 0)
	sc.s.copyTokenEnterFrom(sc.k, &in, first, NoSlot, NoSlot, &eb)
	second := sc.s.newToken(c, 0)
	sc.s.copyTokenEnterFrom(sc.k, &in, second, NoSlot, NoSlot, &eb)
	if got := sc.s.ndly - uint8(n0); got != 2 {
		t.Fatalf("delayed records %d -> %d, want the head and one follower", n0, sc.s.ndly)
	}
	head := copyExileGroupHead(t, sc, n0, first, src)
	copyExileGroupFollower(t, sc.s.dly[n0+1], head.obj, second)
}

// TestCopyPermanentExileAtEndZeroTargetRegisters: a targeted CopyPermanent with
// no resolved target (X = 0, or a target that left before resolution) makes no
// copy but still creates its AtEOT$ Exile delayed triggered ability (CR
// 603.7a; gorge scheduleAtEOTNone), which the end step puts on the stack and
// which exiles nothing.
func TestCopyPermanentExileAtEndZeroTargetRegisters(t *testing.T) {
	sc, c := copyExileGroupScen(t)
	src := sc.s.newToken(c, 0)
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: src, srcVer: sc.s.objs[src].ver, tgt: noTargets}, active: true}
	in := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: kop.RefTarget0, A: kop.Const(0), B: kop.Const(kop.CpExileAtEnd)}
	if got := sc.s.objRefObj(in.Obj); got != NoObj {
		t.Fatalf("precondition: copy source = %d, want no target", got)
	}

	n0 := int(sc.s.ndly)
	sc.s.execCopyToken(sc.k, &in)
	if got := sc.s.ndly - uint8(n0); got != 1 {
		t.Fatalf("zero-target copy registered %d delayed records, want one (CR 603.7a)", got)
	}
	d := sc.s.dly[n0]
	if d.act != 0 || d.when != 0 || d.obj != src || d.creator != NoObj {
		t.Fatalf("empty-rider record = %+v, want a doing-nothing trigger sourced on %d", d, src)
	}

	np := sc.s.npend
	sc.s.fireDelayed(sc.k)
	if got := sc.s.npend - np; got != 1 {
		t.Fatalf("the end step queued %d triggers, want the empty one", got)
	}
}
