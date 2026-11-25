package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The whole-declaration attack requirement (S:Mode$ AttackRequirement, War's
// Toll: "If a creature an opponent controls attacks, all creatures that player
// controls attack if able"). gorge resolves it as a closure over the ACTIVE
// player's declaration (engine/rules/combat AttackRequirementClauses and
// decision.AttackRequirements): a declaration that includes any creature its
// ValidAttacker$ matches must include every able creature its ValidCard$
// matches. The kernel's declaration is incremental, so the same closure is
// read in two places:
//
//   - attackRequirementDuty makes each member a required pick once a trigger
//     is attacking (mustAttackLeft withholds Done, and declareMustAttack
//     pre-declares it in the two-seat path);
//   - attackRequirementTriggered tells the Done guard that the requirement is
//     live even when Config.GorgeMustAttack is off, so the declaration cannot
//     close with a member undeclared.
//
// Both read the static's filters with its own controller as the reference
// (Creature.OppCtrl is the attacker's side), over the active player's
// battlefield, exactly as gorge's AttackRequirementClauses does. "Able" is the
// declaration's own canAttack test, as gorge narrows to the options it offers.

// attackRequirementStatics walks every AttackRequirement static in force whose
// gate holds.
func (s *State) attackRequirementStatics(k *prep, f func(st statSrc) bool) {
	if !s.statM.any() && !(k.embComb && s.anyEmb()) {
		return
	}
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatAttackRequirement || !s.statCond(k, st) {
			return true
		}
		return f(st)
	})
}

// attackRequirementTriggered reports whether some AttackRequirement static has
// one of its ValidAttacker$ creatures among the active player's attackers.
func (s *State) attackRequirementTriggered(k *prep) bool {
	live := false
	s.attackRequirementStatics(k, func(st statSrc) bool {
		f := st.a.Static.Filter2
		for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
			sl := m.first()
			if s.perms[sl].flags&pfAttacking == 0 {
				continue
			}
			if f == 0 || s.matchPermGen(k, f, sl, st.ctrl, st.sl, layerFull) {
				live = true
				return false
			}
		}
		return true
	})
	return live
}

// attackRequirementDuty reports whether the creature at sl must attack because
// an AttackRequirement static's ValidCard$ binds it and one of that static's
// ValidAttacker$ creatures is already attacking.
func (s *State) attackRequirementDuty(k *prep, sl Slot) bool {
	duty := false
	s.attackRequirementStatics(k, func(st statSrc) bool {
		a := st.a
		if f := a.Static.Filter; f != 0 && !s.matchPermGen(k, f, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		f := a.Static.Filter2
		for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
			ts := m.first()
			if s.perms[ts].flags&pfAttacking == 0 {
				continue
			}
			if f == 0 || s.matchPermGen(k, f, ts, st.ctrl, st.sl, layerFull) {
				duty = true
				return false
			}
		}
		return true
	})
	return duty
}
