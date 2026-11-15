package dirac

// attacksDefEnch is the Attacks line's Tf4DefEnch gate (Attacked$
// Player.EnchantedBy, Curse of Predation's "whenever a creature attacks
// enchanted player"): the event group's defender is a player enchanted by the
// LISTENING Aura (defEnchanted, trig_defendingplayer.go). A planeswalker or
// battle defender is no player, so it never matches.
//
//go:noinline
func (s *State) attacksDefEnch(k *prep, l trigListener, grp []Slot) bool {
	def := s.perms[grp[0]].atk
	return !isObjTarget(def) && s.defEnchanted(k, l, CodeSeat(def))
}
