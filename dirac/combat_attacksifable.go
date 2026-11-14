package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Target creature attacks this turn if able." (CR 508.1d; gorge Effect
// StaticAbilities$ Mode$ MustAttack | ValidCreature$ Card.IsRemembered over
// RememberObjects$ Targeted, ExileOnMoved$ Battlefield): an ATTACK
// requirement on the one remembered creature, read where the MustAttack
// static is (declareMustAttack, mustAttackLeft, State.MustAttack).
//
// ExtAttacksIfAble marks the target's slot in the arena bit
// mechAttacksIfAble (0 B): cleared as the next turn begins (the effect lasts
// this turn; mustBlockNewTurn) and when a new permanent takes the slot
// (arenaClrSlot: the effect ends as the creature leaves, ExileOnMoved$
// Battlefield, CR 400.7). The duty sits on the Effect, not on the creature,
// so losing its abilities keeps it.

// attacksIfAbleExt is ExtAttacksIfAble: the battlefield object Obj is marked.
func (s *State) attacksIfAbleExt(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	if !s.mSl(k, mechAttacksIfAble, sl) {
		s.mSlSet(k, mechAttacksIfAble, sl)
		s.touchSl(sl)
	}
	if in.C&kop.AttacksOpponent != 0 && !s.mSl(k, mechAttacksPlayer, sl) {
		s.mSlSet(k, mechAttacksPlayer, sl)
		s.touchSl(sl)
	}
}

// attackPlayerOnly reports whether the attack duty on sl names the opponent
// (kop.AttacksOpponent): only the player satisfies it, so no planeswalker or
// battle is offered as its target, the way gorge's encoreAttackDefender names
// the sole defender.
func (s *State) attackPlayerOnly(k *prep, sl Slot) bool { return s.mSl(k, mechAttacksPlayer, sl) }

// attackDutyOn reports whether the table can mark an attack duty at all.
func attackDutyOn(k *prep) bool {
	return k.gen != nil && k.gen.ix[mechAttacksIfAble] != arenaNone
}

// attackDuty reports whether the permanent at sl attacks this turn if able
// by an Effect's mark.
func (s *State) attackDuty(k *prep, sl Slot) bool { return s.mSl(k, mechAttacksIfAble, sl) }
