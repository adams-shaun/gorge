package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tableUnblocked reports whether some trigger of t fires for unblocked
// attackers (kop.Tf2Unblocked; genPrep.anyUnblocked) and whether some fires
// once per combat for them (kop.Tf2UnblockedOnce; genPrep.anyUnbOnce).
func tableUnblocked(t *kop.Table) (any, once bool) {
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Flags2&kop.Tf2Unblocked != 0 {
				any = true // an Effect's end-of-turn unblocked-attacker line (fireEmblemsUnblocked)
			}
		}
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbTriggered {
				continue
			}
			if a.Trig.Flags2&kop.Tf2Unblocked != 0 {
				any = true
			}
			if a.Trig.Event == kop.TrigBecomesBlocked && a.Trig.Flags2&kop.Tf2UnblockedOnce != 0 {
				once = true // the bit is shared with Tf2SrcInPlay (kop.Tf2UnblockedOnce)
			}
		}
	}
	return any, once
}

// fireUnblocked fires the unblocked-attacker triggers (gorge
// checkAttackerUnblockedTriggers at the declare-blockers round's
// completion, after the declaration's own triggers): each listener, in
// listener order, once per attacking creature that is not blocked, in the
// attacking player's battlefield order, matching its filter. The trigger
// object is the attacker.
//
//go:noinline
func (s *State) fireUnblocked(k *prep) {
	var atk [MaxPerm]Slot
	na := s.battlefieldOrder(s.active, s.ctrlM[s.active], atk[:], 0)
	n := 0
	for _, a := range atk[:na] {
		if f := s.perms[a].flags; f&pfAttacking != 0 && f&pfBlocked == 0 {
			atk[n] = a
			n++
		}
	}
	if n == 0 {
		return
	}
	var ord [MaxPerm]Slot
	m := s.listenerOrder(s.subs[kop.TrigBecomesBlocked].and(s.occ), &ord)
	isUnb := func(a *kop.Ability) bool {
		return a.Trig.Event == kop.TrigBecomesBlocked && a.Trig.Flags2&kop.Tf2Unblocked != 0
	}
	for _, sl := range ord[:m] {
		s.slotTriggers(k, sl, isUnb, func(l trigListener, a *kop.Ability) {
			for _, as := range atk[:n] {
				n0 := s.npend
				s.objTrigGen(k, kop.TrigBecomesBlocked, l, a, as, s.perms[as].obj, s.perms[as].ctrl, 0)
				if !s.duel() {
					s.stampDefSeat(n0, s.atkSeat(k, as), NoTarget)
				}
			}
		})
	}
	s.fireEmblemsUnblocked(k, atk[:n]) // after the listeners: gorge checkEffectCombatTriggers
}

// DebugUnblockedOnceFires counts the Mode$ AttackerUnblockedOnce triggers
// queued (oracle tests only; nil-like zero in normal play).
var DebugUnblockedOnceFires int

// fireUnblockedOnce fires Mode$ AttackerUnblockedOnce lines (Coveted Jewel):
// gorge checkAttackerUnblockedOnceTriggers at the declare-blockers round's
// completion, after fireUnblocked's per-attacker walk. It queues ONE trigger
// per listener per combat when at least one unblocked attacker matches the
// line's controller qualifier (ValidAttackingPlayer$) and attacks the
// listener's controller (ValidDefenders$ You), as gorge's one-instance
// "one or more" semantics do. The trigger object is the first matching
// attacker in battlefield order, its controller the trigger player
// (TriggeredAttackingPlayer). The completion runs once per combat, so the
// line needs no per-combat stamp of its own.
//
//go:noinline
func (s *State) fireUnblockedOnce(k *prep) {
	var atk [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		n += s.battlefieldOrder(p, s.ctrlM[p], atk[n:], 0)
	}
	m := 0
	for _, a := range atk[:n] {
		if f := s.perms[a].flags; f&pfAttacking != 0 && f&pfBlocked == 0 {
			atk[m] = a
			m++
		}
	}
	if m == 0 {
		return
	}
	var ord [MaxPerm]Slot
	ln := s.listenerOrder(s.subs[kop.TrigBecomesBlocked].and(s.occ), &ord)
	isOnce := func(a *kop.Ability) bool {
		return a.Trig.Event == kop.TrigBecomesBlocked && a.Trig.Flags2&kop.Tf2UnblockedOnce != 0
	}
	for _, sl := range ord[:ln] {
		s.slotTriggers(k, sl, isOnce, func(l trigListener, a *kop.Ability) {
			for _, as := range atk[:m] {
				if !whoseOK(a.Trig.Whose, l.ctrl, s.perms[as].ctrl) {
					continue
				}
				if s.atkSeat(k, as) != l.ctrl {
					continue // ValidDefenders$ You
				}
				b := s.npend
				s.objTrigGen(k, kop.TrigBecomesBlocked, l, a, as, s.perms[as].obj, s.perms[as].ctrl, 0)
				if s.npend > b {
					DebugUnblockedOnceFires++
					return // one instance per trigger per combat
				}
			}
		})
	}
}
