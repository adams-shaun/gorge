package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// attackingYou is kop.PredAttackingYou on battlefield slot sl read for the
// controller ctrl: sl is attacking and its defending player is ctrl, the
// attacked player, the controller of the attacked planeswalker or the
// protector of the attacked battle (gorge attackingYou: o.Attacking == you;
// CR 506.2). At two seats that is any attacker ctrl's opponent controls; in
// a pod it is only the attackers aimed at ctrl (Sentry Bot's count, Everett
// K. Ross's "attacks you with two or more creatures").
func (s *State) attackingYou(k *prep, sl Slot, ctrl uint8) bool {
	if s.perms[sl].flags&pfAttacking == 0 {
		return false
	}
	if DebugAttackingYouOpp {
		return s.perms[sl].ctrl != ctrl // the two-seat reading: an attacker an opponent controls
	}
	return s.atkSeat(k, sl) == ctrl
}

// DebugAttackingYouOpp is the lockstep mutation of attackingYou: it reads
// an attacker any opponent controls, the old two-seat reading
// (TestFourSeatAttackingYouLockstep must then diverge).
var DebugAttackingYouOpp bool

// isAttackingYou reports whether fl is the attackingYou predicate.
func isAttackingYou(fl *kop.Filter) bool { return fl.Pred == kop.PredAttackingYou }
