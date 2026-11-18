package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// PrevFromStats counts the combat damage events preventFromGen prevents.
var PrevFromStats struct{ Events atomic.Int64 }

// recomputeDT rebuilds the deathtouch mark of sl from the combat damage events
// that still stand: a deathtouch source whose event was prevented dealt
// nothing, so it leaves no mark (gorge marks on damage dealt only).
func (s *State) recomputeDT(k *prep, r *dmgRec, sl Slot) {
	r.dtPerm.clr(sl)
	for i := 0; i < r.nev; i++ {
		if e := &r.ev[i]; e.dst == sl && e.n > 0 && s.kw(k, e.src)&kop.KwDeathtouch != 0 {
			r.dtPerm.set(sl)
			return
		}
	}
}

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
		if ev.dst != NoSlot {
			s.recomputeDT(k, r, ev.dst) // CR 702.2b: prevented damage marks nothing
		}
		PrevFromStats.Events.Add(1)
	}
}
