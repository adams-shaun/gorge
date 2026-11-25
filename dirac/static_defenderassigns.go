package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StatDefenderAssigns (Defensive Formation): "Rather than the attacking
// player, you assign the combat damage of each creature attacking you". The
// fact is read live from the battlefield (the statics of a permanent p
// controls, as playerShroud walks them): no State bytes. A permanent that
// lost its abilities grants nothing (genLost).

// DebugNoDefenderAssigns is the lockstep mutation switch: the static reads as
// absent. It must make the staticzones lockstep diverge
// (TestStaticZonesDefenderAssignsMutation). Read in defenderAssigns only.
var DebugNoDefenderAssigns bool

// defenderAssigns reports whether the defending player of attacker a assigns
// a's combat damage (gorge defenderAssignsAttackerDamage). Only an attack on
// the player counts: a creature attacking a planeswalker or battle is not
// "attacking you", and gorge's AttackingBattle stays set when the walker has
// left, so a non-player attack target never counts.
//
//go:noinline
func (s *State) defenderAssigns(k *prep, a Slot) bool {
	if k.gen == nil || !k.gen.anyDefAssigns || DebugNoDefenderAssigns || !isPlayerTarget(s.perms[a].atk) {
		return false
	}
	for m := s.statM.and(s.ctrlM[CodeSeat(s.perms[a].atk)]); m.any(); m = m.rest() {
		sl := m.first()
		if s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			if ab := &cd.Abilities[i]; ab.Kind == kop.AbStatic && ab.Static.Kind == kop.StatDefenderAssigns {
				return true
			}
		}
	}
	return false
}
