package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// StatPlayerKeyword (kop.PlKw*): a Continuous `Affected$ You | AddKeyword$ ...`
// grant of a player keyword the kernel reads. The fact is read live from the
// battlefield (the statics of a permanent p controls, as playerShroud walks
// them): no State bytes. A permanent that lost its abilities grants nothing
// (genLost), as gorge's static scan drops a lost source.

// DebugNoPlayerKeyword is the lockstep mutation switch: bit kop.PlKw* set,
// that keyword reads as absent. It must make the staticzones lockstep diverge
// (TestStaticZonesPlayerKeywordMutation). Read in playerKeyword only.
var DebugNoPlayerKeyword uint8

// playerKeyword reports whether p controls a functioning StatPlayerKeyword
// static granting keyword kw (kop.PlKwPayLifeInsteadOfB, kop.PlKwDefenderAssigns).
//
//go:noinline
func (s *State) playerKeyword(k *prep, p uint8, kw int) bool {
	if k.gen == nil || k.gen.anyPlKw&(1<<uint(kw&7)) == 0 || DebugNoPlayerKeyword&(1<<uint(kw&7)) != 0 {
		return false
	}
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		if s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatPlayerKeyword && int(a.Static.A) == kw {
				return true
			}
		}
	}
	return false
}

// defenderAssigns reports whether the defending player of attacker a assigns
// a's combat damage (Defensive Formation: "Rather than the attacking player,
// you assign the combat damage of each creature attacking you"; gorge
// defenderAssignsAttackerDamage). Only an attack on the player counts: a
// creature attacking a planeswalker or battle is not "attacking you", and
// gorge's AttackingBattle stays set when the walker has left, so a non-player
// attack target never counts.
func (s *State) defenderAssigns(k *prep, a Slot) bool {
	if k.gen == nil || k.gen.anyPlKw&(1<<kop.PlKwDefenderAssigns) == 0 || !isPlayerTarget(s.perms[a].atk) {
		return false
	}
	return s.playerKeyword(k, CodeSeat(s.perms[a].atk), kop.PlKwDefenderAssigns)
}
