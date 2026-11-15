package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// removeNoncreatureCombatants is CR 506.4's "an attacking or blocking
// creature that stops being a creature is removed from combat" (Sugar Coat
// on an attacker or a blocker), gorge's rules/combat_removal.go
// removeNoncreatureCombatants run from its state-based-action pass loop.
// Each removed permanent stops attacking and blocking, as gorge's
// EndCombatReset{Obj} fold (events/apply.go resetCombat): an attacker it
// blocked stays blocked with one blocker fewer (509.1h), and a blocker of a
// removed attacker stays blocking but blocks nothing. Cold: sba calls it only
// while a combat is live.
func (s *State) removeNoncreatureCombatants(k *prep) {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		p := &s.perms[sl]
		if p.flags&(pfAttacking|pfBlocking) == 0 || s.typesAt(k, sl)&kop.TCreature != 0 {
			continue
		}
		s.leaveCombat(sl)
	}
}

// leaveCombat removes sl from combat exactly as gorge's EndCombatReset{Obj}
// fold (events/apply.go resetCombat) does: an attacker's blockers stay
// blocking but block nothing, and a blocker stops blocking while its
// attacker stays blocked with one blocker fewer (509.1h). Cold: regeneration,
// the noncreature state-based action and ExtRemoveFromCombat (gorge
// effRemoveFromCombat's EndCombatReset) are its callers.
func (s *State) leaveCombat(sl Slot) {
	p := &s.perms[sl]
	if p.flags&pfAttacking != 0 {
		s.nAttackers--
	}
	for q := s.occ; q.any(); q = q.rest() {
		o := q.first()
		s.forgetBlocked(o, sl)
	}
	if p.flags&pfBlocking != 0 {
		s.releaseBlocked(sl)
	}
	p.flags &^= pfAttacking | pfBlocking | pfBlocked
	p.blocking = NoSlot
	p.nblock = 0
	s.touchSl(sl)
}
