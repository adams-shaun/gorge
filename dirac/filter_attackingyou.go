package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// attackingYou is kop.PredAttackingYou on battlefield slot sl read for the
// controller ctrl: sl is attacking and its defending player is ctrl, the
// attacked player, the controller of the attacked planeswalker or the
// protector of the attacked battle (gorge attackingYou: o.Attacking == you;
// CR 506.2). At two seats that is any attacker ctrl's opponent controls; in
// a pod it is only the attackers aimed at ctrl (Sentry Bot's count, Everett
// K. Ross's "attacks you with two or more creatures").
//
// pw 1 is gorge attackingYouOrYourPW(LKI) instead: the attacker is aimed at
// ctrl itself, or at a permanent whose face is a non-creature planeswalker
// controlled by ctrl. gorge reads the attacked object by id
// (Object.AttackingBattle) through any later zone move, so a planeswalker
// that left still answers by its card and its controller off the battlefield
// (its owner here, as atkSeatLeft reads it).
func (s *State) attackingYou(k *prep, sl Slot, ctrl uint8, pw int16) bool {
	if s.perms[sl].flags&pfAttacking == 0 {
		return false
	}
	if DebugAttackingYouOpp {
		return s.perms[sl].ctrl != ctrl // the two-seat reading: an attacker an opponent controls
	}
	if pw == 1 {
		return s.attackingYouOrPW(k, sl, ctrl)
	}
	return s.atkSeat(k, sl) == ctrl
}

// attackingYouOrPW is attackingYou's pw arm (gorge attackingYouOrYourPWLKI).
func (s *State) attackingYouOrPW(k *prep, sl Slot, ctrl uint8) bool {
	t := s.perms[sl].atk
	if isPlayerTarget(t) {
		return s.atkSeat(k, sl) == ctrl
	}
	if isPlayerOrNone(t) {
		return false
	}
	ob := &s.objs[t]
	var c uint8
	var ty kop.TypeMask
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		c, ty = s.perms[ob.slot].ctrl, k.cards[s.cardAt(ob.slot)].types
	} else {
		c, ty = ob.owner(), s.castRingTypes(k, Obj(t))
	}
	return c == ctrl && ty&kop.TPlaneswalker != 0 && ty&kop.TCreature == 0
}

// DebugAttackingYouOpp is the lockstep mutation of attackingYou: it reads
// an attacker any opponent controls, the old two-seat reading
// (TestFourSeatAttackingYouLockstep must then diverge).
var DebugAttackingYouOpp bool

// isAttackingYou reports whether fl is the attackingYou predicate.
func isAttackingYou(fl *kop.Filter) bool { return fl.Pred == kop.PredAttackingYou }
