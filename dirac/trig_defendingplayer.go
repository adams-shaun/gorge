package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fireDefPlayer is a per-defender "whenever creatures attack a player"
// line (kop.Tf2DefPlayer: gorge's AttackersDeclared with AttackedTarget$
// Player / Opponent, or Mode$ AttackersDeclaredOneTarget) on one defender
// group: only the player group fires, with Tf2AtkOpp only when that player
// is an opponent of the listener's controller; the amount is the group's
// attackers matching Filter (none: no fire), held to MinAttackers.
//
//go:noinline
func (s *State) fireDefPlayer(k *prep, l trigListener, a *kop.Ability, grp []Slot) {
	if len(grp) == 0 || isObjTarget(s.perms[grp[0]].atk) {
		return
	}
	def := CodeSeat(s.perms[grp[0]].atk)
	if a.Trig.Flags2&kop.Tf2AtkOpp != 0 && def == l.ctrl {
		return
	}
	if a.Trig.Flags2&kop.Tf2AtkYou != 0 && (def != l.ctrl) != DebugTrigQualsFlip {
		return // AttackedTarget$ You (trig_qualifiers.go)
	}
	if a.Trig.Flags4&kop.Tf4DefEnch != 0 && !s.defEnchanted(k, l, def) {
		return // AttackedTarget$ Player.EnchantedBy (kop Tf4DefEnch)
	}
	n := int16(len(grp))
	if a.Trig.Filter != 0 {
		n = s.attackersMatching(k, l, a, grp)
	}
	if n > 0 {
		s.fireAtkDecl(k, l, a, n, grp) // trig_attackers.go
	}
}

// defEnchanted is the Tf4DefEnch gate: the attacked seat is enchanted by
// the LISTENING Aura itself. Forge's PlayerProperty.EnchantedBy is
// player.isEnchantedBy(source) -- the ability's source, not any enchantment
// -- so two curses on two seats do not fire on each other's declarations, and
// an Archnemesis on one seat does not fire when a different Aura's player is
// attacked. The source is the listener's own permanent: its player-attached
// seat (perm.atchP) plus its Aura type. A missing listener slot or a
// non-Aura source enchants nobody.
func (s *State) defEnchanted(k *prep, l trigListener, def uint8) bool {
	if l.sl == NoSlot {
		return false
	}
	p := &s.perms[l.sl]
	if p.atchP != def+1 {
		return false
	}
	return k.cards[s.objs[p.obj].card].types&kop.TAura != 0
}
