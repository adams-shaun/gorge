package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttachObjEachTarget: OpAttachObj whose Ref C target slot holds several
// objects (Thorin, Mountain-king's Object$ ParentTarget over "any number of
// target Equipment", gorge attachObjectParentTarget) attaches each to the
// destination in pick order; B = Const 1 remembers exactly the objects
// attached; an extra that left the battlefield attaches nothing; B = 0
// remembers nothing.
func TestAttachObjEachTarget(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Goldvein Pick", Count: 3}, {Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}})
	s, k := sc.s, sc.k
	if k.gen == nil {
		t.Fatal("setup: the scenario has no generated tables")
	}
	setup := func() (lions Obj, picks [3]Obj) {
		lions = sc.perm(id["Savannah Lions"], 0)
		for i := range picks {
			picks[i] = sc.perm(id["Goldvein Pick"], 0)
		}
		s.res.e.ctrl, s.res.e.obj, s.res.e.card = 0, lions, id["Savannah Lions"]
		s.res.e.srcVer = s.objs[lions].ver
		s.res.bad, s.res.xbad = 0, 0
		s.res.e.tgt[0], s.res.e.tver[0] = uint8(picks[0]), s.objs[picks[0]].ver
		s.res.e.tgt[1], s.res.e.tver[1] = uint8(lions), s.objs[lions].ver
		xt := s.xtMut(&s.res.e)
		if xt == nil {
			t.Fatal("setup: no extra-target pool entry")
		}
		*xt = xTargets{n: 2}
		for j, o := range picks[1:] {
			xt.slot[j], xt.code[j], xt.ver[j] = 0, uint8(o), s.objs[o].ver
		}
		s.g.nrem = 0
		return lions, picks
	}
	in := kop.Instr{Op: kop.OpAttachObj, Obj: kop.RefTarget1, B: kop.Const(1), C: int16(kop.RefTarget0)}

	t.Run("three", func(t *testing.T) {
		lions, picks := setup()
		s.exec(k, &in)
		for i, p := range picks {
			if at := s.perms[sc.slot(p)].attached; at != sc.slot(lions) {
				t.Errorf("pick %d attached to slot %d, want the Lions' %d", i, at, sc.slot(lions))
			}
		}
		if s.g.nrem != 3 {
			t.Fatalf("remembered %d objects, want 3", s.g.nrem)
		}
		for i, p := range picks {
			if s.g.rem[i] != p {
				t.Errorf("rem[%d] = %d, want pick %d (%d) in pick order", i, s.g.rem[i], i, p)
			}
		}
	})
	t.Run("one extra gone", func(t *testing.T) {
		lions, picks := setup()
		s.destroy(k, sc.slot(picks[1]))
		if sc.onBF(picks[1]) {
			t.Fatal("setup: the destroyed pick is still on the battlefield")
		}
		s.exec(k, &in)
		for _, p := range []Obj{picks[0], picks[2]} {
			if at := s.perms[sc.slot(p)].attached; at != sc.slot(lions) {
				t.Errorf("pick %d attached to slot %d, want the Lions' %d", p, at, sc.slot(lions))
			}
		}
		if s.g.nrem != 2 || s.g.rem[0] != picks[0] || s.g.rem[1] != picks[2] {
			t.Fatalf("remembered %v (n=%d), want the two picks still there", s.g.rem[:s.g.nrem], s.g.nrem)
		}
	})
	t.Run("B=0 remembers nothing", func(t *testing.T) {
		lions, picks := setup()
		plain := in
		plain.B = kop.ValueRef{}
		s.exec(k, &plain)
		if at := s.perms[sc.slot(picks[2])].attached; at != sc.slot(lions) {
			t.Fatalf("the last pick attached to slot %d, want the Lions' %d", at, sc.slot(lions))
		}
		if s.g.nrem != 0 {
			t.Fatalf("remembered %d objects with B=0", s.g.nrem)
		}
	})
}
