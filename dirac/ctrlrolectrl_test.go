package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCtrlRoleTgtDepartedOwner: a Filter.CtrlTgt link is judged from the raw
// earlier target slot. The earlier target's permanent was destroyed in
// response (owner 0, stolen by seat 1), so resolveTop has marked its slot
// illegal (res.bad) before the later link is rechecked. gorge's
// controlReferentPlayers reads the object as it now is: off the battlefield
// its controller is its owner (events.Apply resets Controller to Owner), so
// the later link, a creature seat 0 controls, is still legal. Reading the
// slot through tgtCode would answer NoTarget for the illegal slot and turn
// the whole chain false.
func TestCtrlRoleTgtDepartedOwner(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	// the earlier target: owned by seat 0, controlled by seat 1
	pw := sc.obj(fc, 0)
	sc.s.enter(sc.k, pw, 1, false, kop.ZoneNone)
	// the later target: a creature controlled by the owner
	creature := sc.perm(fc, 0)
	e := &sc.s.res.e
	e.ctrl, e.obj, e.card = 0, pw, fc
	e.nt = 2
	e.tgt[0], e.tver[0] = uint8(pw), sc.s.objs[pw].ver
	e.tgt[1], e.tver[1] = uint8(creature), sc.s.objs[creature].ver
	sc.s.res.active = true
	if !sc.s.ctrlRoleTgt(sc.k, 0, 1) || sc.s.ctrlRoleTgt(sc.k, 0, 0) {
		t.Fatalf("before departure: slot 0 ctrl 1 %v, ctrl 0 %v; want true, false",
			sc.s.ctrlRoleTgt(sc.k, 0, 1), sc.s.ctrlRoleTgt(sc.k, 0, 0))
	}
	sc.s.leave(sc.k, sc.slot(pw), kop.ZoneGraveyard, false)
	sc.s.res.bad = 1 << 0 // resolveTop's fizzle pass, before slot 1 is judged
	if sc.s.objs[pw].zone != kop.ZoneGraveyard || sc.s.objs[pw].owner() != 0 {
		t.Fatalf("the earlier target is at zone %d owner %d, want graveyard (owner 0)",
			sc.s.objs[pw].zone, sc.s.objs[pw].owner())
	}
	if !sc.s.ctrlRoleTgt(sc.k, 0, 0) {
		t.Fatal("after the earlier target left, slot 0 no longer names its owner 0")
	}
	if sc.s.ctrlRoleTgt(sc.k, 0, 1) {
		t.Fatal("after the earlier target left, slot 0 still names its last-known controller 1")
	}
}

// TestCtrlRoleTgtStackSlot: a slot naming a stack ability is not an object
// index. The build arm must read the ability's controller through the
// stack table, not size an object with the target byte.
func TestCtrlRoleTgtStackSlot(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	src := sc.perm(fc, 0)
	sc.s.push(stackEntry{kind: seAbility, obj: src, ctrl: 1, trigObj: NoObj})
	sc.s.stackSid(&sc.s.stack[0])
	sc.s.ctl = ctlBuild
	*sc.s.bld() = build{e: stackEntry{kind: seAbility, ctrl: 0, obj: src, nt: 1, trigObj: NoObj}}
	sc.s.bld().e.tgt[0] = uint8(StackTarget0)
	sc.s.bld().e.tver[0] = sc.s.stack[0].sid
	if !sc.s.ctrlRoleTgt(sc.k, 0, 1) {
		t.Fatal("a stack ability slot did not name its controller 1")
	}
	if sc.s.ctrlRoleTgt(sc.k, 0, 0) {
		t.Fatal("a stack ability slot named seat 0, not its controller 1")
	}
}
