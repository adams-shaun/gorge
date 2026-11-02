package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// preventCombatEv takes combat damage event ev out of r's accounting: the
// event is prevented whole (a shield counter, protection), so it is not
// damage dealt — no DamageDone/DamageDealtOnce trigger, no lifelink, no
// deathtouch mark. The source's dealt totals lose it, the recipient's
// assigned damage loses it, and its deathtouch mark is recomputed from the
// events that remain. Every combat prevention goes through here
// (replCombatGen's protection, shieldCombat, dealCombatEvent and
// dealAsked's shield), so a new prevention cannot leave r thinking the
// damage was dealt and fire its triggers.
func (s *State) preventCombatEv(k *prep, r *dmgRec, ev *dmgEv) {
	if ev.n <= 0 {
		return
	}
	r.dealt[ev.src] -= ev.n
	if ev.dst == NoSlot {
		r.dealtPl[ev.src] -= ev.n
		if a := s.perms[ev.src].atk; a >= PlayerTarget0 {
			r.toPl[a-PlayerTarget0] -= ev.n
		}
	} else {
		r.toPerm[ev.dst] -= ev.n
		r.dtPerm.clr(ev.dst)
		for i := 0; i < r.nev; i++ {
			if e := &r.ev[i]; e.dst == ev.dst && e.n > 0 && s.kw(k, e.src)&kop.KwDeathtouch != 0 {
				r.dtPerm.set(ev.dst)
			}
		}
	}
	ev.n = 0
}

// preventFrozenEv syncs the recomputed pass's accounting for a frozen event
// already dealt in the paused pass. The resumed pass rebuilds r from the
// board, so a shield that prevented the event before the ask is not in it;
// replOrderCombat skips the already-dealt event, and without this the
// rebuilt event would fire the DamageDone trigger at the end of the pass.
func (s *State) preventFrozenEv(k *prep, r *dmgRec, fe *fzEv) {
	if fe.n > 0 {
		return
	}
	for i := 0; i < r.nev; i++ {
		if e := &r.ev[i]; e.src == fe.src && e.dst == fe.dst {
			s.preventCombatEv(k, r, e)
			return
		}
	}
}
