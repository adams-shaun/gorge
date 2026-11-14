package dirac

import "sync/atomic"

// PrevFromStats counts the combat damage events preventFromGen prevents.
var PrevFromStats struct{ Events atomic.Int64 }

// preventFromGen prevents the combat damage a source with
// KwPreventCombatDamageFrom would deal (gorge: an Effect's bodyless
// Prevent$ True IsCombat$ True ValidSource$ Card.IsRemembered replacement,
// effects/effect.go; damageReplacementPrevents): each of its events is
// removed, so nothing is dealt, no lifelink, no damage trigger.
//
//go:noinline
func (s *State) preventFromGen(k *prep, r *dmgRec) {
	for i := 0; i < r.nev; i++ {
		ev := &r.ev[i]
		if ev.n <= 0 || !s.prevCombatFrom(k, ev.src) {
			continue
		}
		if ev.dst == NoSlot {
			if t := s.perms[ev.src].atk; isPlayerOrNone(t) {
				r.toPl[CodeSeat(t)] -= ev.n
			}
			r.dealtPl[ev.src] -= ev.n
		} else {
			r.toPerm[ev.dst] -= ev.n
		}
		r.dealt[ev.src] -= ev.n
		ev.n = 0
		PrevFromStats.Events.Add(1)
	}
}
