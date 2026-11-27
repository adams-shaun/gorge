package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTgtCtrlLiveStackTargetFailsClosed: the live read answers a stack
// ability's controller while it is on the stack (CR 113.8), and names nobody
// once it left (counterAt drops the entry, so no live owner is recoverable).
// kgen refuses a ParentTargetedController whose parent slot may name a stack
// object (lower_effect_defined.go), so this arm is only reachable from a
// hand-built program; fail closed rather than read the last-known controller.
func TestTgtCtrlLiveStackTargetFailsClosed(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	src := sc.perm(v.TyphoidRats, 1)
	s.push(stackEntry{kind: seAbility, obj: src, ctrl: 1, trigObj: NoObj})
	s.stackSid(&s.stack[0])
	s.res.e.tgt[0] = TCode(StackTarget0)
	s.res.e.tver[0] = s.stack[0].sid
	if got := s.tgtCtrlLive(0); got != 1 {
		t.Errorf("stack ability on the stack: controller %d, want 1", got)
	}
	s.sp = 0      // the ability was countered: the entry is gone
	s.g.stkCt = 1 // the counter recorded controller 0+1 (the last-known read)
	if got := s.tgtCtrlLive(0); got != 255 {
		t.Errorf("countered stack ability: controller %d, want nobody (255)", got)
	}
}

// TestRefTargetCtrlLiveResolvesToController is the resolution-level half of
// Defined$ ParentTargetedController (Unlicensed Disintegration): a DealDamage
// whose obj is RefTargetCtrlLive must hit the target's controller's life, not
// fall through to the object-ref path and deal nothing. The target is a
// stolen permanent (controller 1, owner 0) so every arm is distinguishable.
func TestRefTargetCtrlLiveResolvesToController(t *testing.T) {
	sc := newScen(t, Config{})
	s, k := sc.s, sc.k
	victim := sc.perm(v.TyphoidRats, 1) // controller 1
	s.objs[victim].setOwner(0)          // stolen: owner 0, controller still 1
	slot := sc.slot(victim)
	s.res.e.ctrl = 0
	s.res.e.tgt[0] = TCode(victim)
	hit := kop.Instr{Op: kop.OpDealDamage, Obj: kop.RefTargetCtrlLive, A: kop.Const(3)}

	// The live battlefield permanent names its CONTROLLER (seat 1), not its
	// owner (seat 0), and the permanent itself takes no damage.
	life1, life0 := s.pl[1].life, s.pl[0].life
	s.exec(k, &hit)
	if s.pl[1].life != life1-3 {
		t.Errorf("battlefield target: controller life %d, want %d", s.pl[1].life, life1-3)
	}
	if s.pl[0].life != life0 {
		t.Errorf("battlefield target: owner life %d, want %d (untouched)", s.pl[0].life, life0)
	}
	if d := s.perms[slot].damage; d != 0 {
		t.Errorf("battlefield target: the permanent took %d damage, want 0 (controller is the referent)", d)
	}

	// A phased-out permanent is still the same permanent (CR 702.26d): it
	// keeps its controller, so the live read must not reset to its owner.
	s.objs[victim].zone = kop.ZonePhased
	life1, life0 = s.pl[1].life, s.pl[0].life
	s.exec(k, &hit)
	if s.pl[1].life != life1-3 {
		t.Errorf("phased-out target: controller life %d, want %d", s.pl[1].life, life1-3)
	}
	if s.pl[0].life != life0 {
		t.Errorf("phased-out target: owner life %d, want %d (untouched)", s.pl[0].life, life0)
	}

	// A target that left the battlefield is a new object under its owner
	// (CR 400.7), which is the live read the lowering depends on.
	s.objs[victim].zone = kop.ZoneGraveyard
	life1, life0 = s.pl[1].life, s.pl[0].life
	s.exec(k, &hit)
	if s.pl[0].life != life0-3 {
		t.Errorf("departed target: owner life %d, want %d", s.pl[0].life, life0-3)
	}
	if s.pl[1].life != life1 {
		t.Errorf("departed target: old controller life %d, want %d (untouched)", s.pl[1].life, life1)
	}
}
