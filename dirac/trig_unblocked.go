package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tableUnblocked reports whether some trigger of t fires for unblocked
// attackers (kop.Tf2Unblocked; genPrep.anyUnblocked).
func tableUnblocked(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Flags2&kop.Tf2Unblocked != 0 {
				return true
			}
		}
	}
	return false
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
				s.objTrigGen(k, kop.TrigBecomesBlocked, l, a, as, s.perms[as].obj, s.perms[as].ctrl, 0)
			}
		})
	}
}
