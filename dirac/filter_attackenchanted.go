package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// attackingEnchTrig is kop.W7AttackingEnchTrig on battlefield slot sl: a
// creature the current trigger entry's trigger player controls, attacking the
// player the source Aura (src) is attached to. gorge reads the curses'
// second-leg selector at resolution (Player.EnchantedBy is
// player.isEnchantedBy(source)), so the Aura leaving or moving (src is
// NoSlot once its trigger's source left, the srcVer check in ctxEntry), the
// attacker leaving combat, or the attacker's controller stopping being the
// declaring player all empty the set. An attack on a planeswalker or battle
// is not an attack on the player.
func (s *State) attackingEnchTrig(k *prep, sl, src Slot) bool {
	if src == NoSlot {
		return false
	}
	sp := &s.perms[src]
	at := s.atchSeat(k, src)
	if at == 0 || k.cards[s.objs[sp.obj].card].types&kop.TAura == 0 {
		return false
	}
	p := &s.perms[sl]
	return p.flags&pfAttacking != 0 && isPlayerTarget(TCode(p.atk)) && CodeSeat(TCode(p.atk)) == at-1 &&
		s.isCreature(k, sl) && s.trigPlayerCtrl(sl)
}
