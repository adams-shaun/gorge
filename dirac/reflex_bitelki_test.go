package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReflexBiteGoneDealsLastKnownPower: a reflexive entry's Bite from its
// captured creature (RefDelayedRemembered; Thorin, Mountain-king's "that
// creature deals damage equal to its power") deals, once the creature has
// left the battlefield, the power snapshotted as the entry was queued (CR
// 608.2h; gorge rules rememberedPTLKI), from the gone creature. Without a
// snapshot, or with the target gone, nothing is dealt (the stale skip). A
// creature still there bites with its live power.
func TestReflexBiteGoneDealsLastKnownPower(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Plains", Count: 4}})
	s, k := sc.s, sc.k
	if k.gen == nil {
		t.Fatal("setup: the scenario has no generated tables")
	}
	bite := kop.Instr{Op: kop.OpBite, Obj: kop.RefDelayedRemembered, C: int16(kop.RefTarget0)}
	// setup: Savannah Lions (2/1) with a +1/+1 counter is the captured
	// creature (power 3), Eager Trufflesnout the target (no SBA runs here); snap takes the entry's LKI
	// record as OpReflexiveTrigger does.
	setup := func(snap bool) (lions, target Obj) {
		lions = sc.perm(id["Savannah Lions"], 0)
		target = sc.perm(id["Eager Trufflesnout"], 1)
		s.perms[sc.slot(lions)].addCtrN(s, int(kop.CtrP1P1), 1)
		if got := s.power(k, sc.slot(lions)); got != 3 {
			t.Fatalf("setup: the countered Lions' power = %d, want 3", got)
		}
		e := &s.res.e
		*e = stackEntry{}
		e.ctrl, e.obj, e.card = 0, lions, id["Savannah Lions"]
		e.trigObj, e.trigAmt = lions, int16(s.objs[lions].ver)
		e.tgt[0], e.tver[0] = TCode(target), s.objs[target].ver
		s.res.bad, s.res.xbad = 0, 0
		if snap {
			e.li = s.snapLKI(lions)
			if e.li == 0 {
				t.Fatal("setup: no LKI record taken")
			}
		}
		return lions, target
	}

	t.Run("gone: last known power", func(t *testing.T) {
		lions, target := setup(true)
		s.destroy(k, sc.slot(lions))
		if sc.onBF(lions) || !s.reflexStale() {
			t.Fatal("setup: the captured Lions are still the same object on the battlefield")
		}
		s.exec(k, &bite)
		if d := s.perms[sc.slot(target)].damage; d != 3 {
			t.Fatalf("Eager Trufflesnout has %d damage, want 3 (the Lions' last known power, not the printed 2)", d)
		}
	})
	t.Run("gone without a snapshot: skipped", func(t *testing.T) {
		lions, target := setup(false)
		s.destroy(k, sc.slot(lions))
		s.exec(k, &bite)
		if d := s.perms[sc.slot(target)].damage; d != 0 {
			t.Fatalf("Eager Trufflesnout has %d damage, want 0", d)
		}
	})
	t.Run("gone with the target gone: nothing", func(t *testing.T) {
		lions, target := setup(true)
		s.destroy(k, sc.slot(lions))
		s.destroy(k, sc.slot(target))
		life := [2]int{int(s.pl[0].life), int(s.pl[1].life)}
		s.exec(k, &bite)
		if sc.onBF(target) || int(s.pl[0].life) != life[0] || int(s.pl[1].life) != life[1] {
			t.Fatal("a bite at a gone target changed the game")
		}
	})
	t.Run("still there: live power", func(t *testing.T) {
		lions, target := setup(true)
		s.perms[sc.slot(lions)].addCtrN(s, int(kop.CtrP1P1), 1) // pumped after the snapshot
		s.exec(k, &bite)
		if d := s.perms[sc.slot(target)].damage; d != 4 {
			t.Fatalf("Eager Trufflesnout has %d damage, want 4 (the Lions' live power)", d)
		}
	})
}
