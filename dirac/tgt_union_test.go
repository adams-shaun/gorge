package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// unionScen is a resolving entry with targets a (slot 0) and b (slot 1).
func unionScen(t *testing.T) (*scen, Obj, Obj, Obj) {
	t.Helper()
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	src, a, b := sc.perm(fc, 0), sc.perm(fc, 1), sc.perm(fc, 1)
	sc.k.gen.anyUnion = true
	e := &sc.s.res.e
	e.ctrl, e.obj, e.card = 0, src, fc
	e.tgt[0], e.tver[0] = TCode(a), sc.s.objs[a].ver
	e.tgt[1], e.tver[1] = TCode(b), sc.s.objs[b].ver
	e.tgt[2] = TCode(NoTarget)
	sc.s.res.bad = 0
	return sc, src, a, b
}

// TestUnionDestroysEverySlot: Defined$ Targeted over a two-slot chain
// (Fumarole's "destroy target creature and target land") destroys both
// slots' targets, as gorge's chain target union does, not only the last
// slot's; the slots' targets are restored after.
func TestUnionDestroysEverySlot(t *testing.T) {
	sc, src, a, b := unionScen(t)
	in := kop.Instr{Op: kop.OpDestroy, Obj: kop.RefTargets01}
	sc.s.exec(sc.k, &in)
	if sc.onBF(a) || sc.onBF(b) {
		t.Fatalf("after Destroy Targets01: slot 0 on the battlefield %v, slot 1 %v; want both destroyed", sc.onBF(a), sc.onBF(b))
	}
	if !sc.onBF(src) {
		t.Fatal("the source was destroyed")
	}
	if sc.s.res.e.tgt[0] != TCode(uint8(a)) || sc.s.res.e.tgt[1] != TCode(uint8(b)) || sc.s.g.inLoop {
		t.Fatal("the entry's targets or the loop flag were not restored")
	}
}

// TestUnionSkipsIllegalSlot: a slot whose target is illegal at resolution
// (CR 608.2b) is left out of the union; the other slot's is still
// destroyed, and the illegal mark survives the op.
func TestUnionSkipsIllegalSlot(t *testing.T) {
	sc, _, a, b := unionScen(t)
	sc.s.res.bad = 1 << 0
	in := kop.Instr{Op: kop.OpDestroy, Obj: kop.RefTargets01}
	sc.s.exec(sc.k, &in)
	if !sc.onBF(a) {
		t.Fatal("the illegal slot 0 target was destroyed")
	}
	if sc.onBF(b) {
		t.Fatal("the legal slot 1 target survived")
	}
	if sc.s.res.bad != 1<<0 {
		t.Fatalf("res.bad %b after the union, want 1", sc.s.res.bad)
	}
}

// TestUnionUpToNone: an up-to slot chosen empty (NoTarget) contributes
// nothing; slots outside the mask are not read.
func TestUnionUpToNone(t *testing.T) {
	sc, _, a, b := unionScen(t)
	sc.s.res.e.tgt[1] = TCode(NoTarget)
	sc.s.res.e.tgt[2], sc.s.res.e.tver[2] = TCode(b), sc.s.objs[b].ver
	in := kop.Instr{Op: kop.OpDestroy, Obj: kop.RefTargets01}
	sc.s.exec(sc.k, &in)
	if sc.onBF(a) || !sc.onBF(b) {
		t.Fatalf("slot 0 on the battlefield %v (want false), slot 2 %v (want true)", sc.onBF(a), sc.onBF(b))
	}
}
