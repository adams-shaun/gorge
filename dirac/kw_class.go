package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Class enchantments (CR 716 threshold-gated abilities),
// copied from gorge (cards/kw_class.go kwClass, effects/class_level.go
// effClassLevelUp, rules/class_level.go classBandGateHolds,
// rules/trigmatch/class.go classLevelGainedMatches):
//
//   - a Class's level is its CtrLevel count plus one (no slot at level 1,
//     CR 716.2d); the kind sits above kop.NumCounterKinds2, so no counter
//     census, doubling, proliferation or projection sees it (gorge's level
//     is a designation, not a counter: CR 716.4). It goes with the
//     permanent's slot when the Class leaves (gorge resets the designation
//     on a zone change), and a copy enters at level 1 (CR 716.2b).
//   - each level-N ability carries Ability.Gate{CtrLevel, N-1}: a triggered
//     one is enumerated only while the gate holds (slotTriggers) and is
//     rechecked on resolution (trigResolveOK; gorge ANDs ClassBand$ into the
//     intervening-if walk, so a level-N trigger whose Class has left does
//     nothing); an activated one is offered only while it holds
//     (canActivate).
//   - the level-N activator is a sorcery-speed activated ability gated on
//     the level being exactly N-1 (Gate{CtrLevel, N-2, Eq}); it resolves
//     OpClassLevelUp, which sets the level and fires "when this Class
//     becomes level N" (TrigCounterAddedOnce with N = CtrLevel, gated Eq on
//     the new level).
//
// Every check sits behind prep.anyGate, false for FDN.

// gateOK: ability a's gate holds for its source at slot sl (NoSlot: the
// source has left the battlefield, where a gated ability does not exist).
//
//go:noinline
func (s *State) gateOK(sl Slot, a *kop.Ability) bool {
	g := &a.Gate
	if g.Ctr == 0 {
		return true
	}
	if sl == NoSlot {
		return false
	}
	if g.Ctr == kop.CtrMaxSpeed {
		return s.maxSpeedGate(sl, a) // kw_speed.go
	}
	if g.Ctr == kop.CtrMode {
		return s.modeGateOK(sl, a) // face_siegemode.go
	}
	if DebugStationGrantNoGate && g.Ctr == kop.CtrCharge {
		return true
	}
	n := s.perms[sl].ctrN(s, int(g.Ctr))
	if g.Eq {
		return n == int(g.N)
	}
	return n >= int(g.N)
}

// gateResolveOK rechecks a resolving gated trigger's gate on its source
// (gorge's classBandGateHolds in the resolution-time intervening-if walk).
//
//go:noinline
func (s *State) gateResolveOK(e *stackEntry, a *kop.Ability) bool {
	if a.Gate.Ctr == 0 || a.Gate.Ctr == kop.CtrMaxSpeed {
		return true
	}
	return s.gateOK(s.ctxEntry(e).srcSl, a)
}

// classLevelUp is OpClassLevelUp: the resolving ability's source Class
// becomes level lv (gorge effClassLevelUp: nothing when the source is off
// the battlefield or already at lv or higher).
//
//go:noinline
func (s *State) classLevelUp(k *prep, in *kop.Instr) {
	sl := s.objRef(k, kop.RefSelf)
	if sl == NoSlot {
		return
	}
	lv := s.value(k, in.A)
	p := &s.perms[sl]
	if lv < 2 || p.ctrN(s, int(kop.CtrLevel))+1 >= lv {
		return
	}
	s.touchSl(sl)
	if !p.setCtrN(s, int(kop.CtrLevel), lv-1) {
		s.ctrStall()
		return
	}
	if s.subs[kop.TrigCounterAddedOnce].any() {
		s.g.lastCK = kop.CtrLevel
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, 1)
	}
}
