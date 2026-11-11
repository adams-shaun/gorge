package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fireBlockedGen fires the become-blocked triggers (TrigBecomesBlocked,
// gorge Mode$ AttackerBlocked) as blockers are declared like gorge's
// checkAttackerBlockedTriggers: listeners in battlefield order (seat 0's
// list, then seat 1's), each listener's abilities in ability order, an
// ability once per DISTINCT blocked attacker matching its filter, the
// attackers in the order of their first block in the declaration (gorge's
// DeclareBlockers pairs). The trigger object is the attacker.
//
//go:noinline
func (s *State) fireBlockedGen(k *prep) {
	var atk [MaxPerm]Slot
	na := 0
	var ps [2 * MaxPerm]blkPair
	nb := s.blockPairs(&ps)
	var seen slotMask
	for _, pr := range ps[:nb] {
		a := pr.a
		if seen.has(a) || s.perms[a].flags&pfBlocked == 0 {
			continue
		}
		seen.set(a)
		atk[na] = a
		na++
	}
	if na == 0 {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.subs[kop.TrigBecomesBlocked].and(s.occ), &ord)
	isBlk := func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigBecomesBlocked }
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, isBlk, func(l trigListener, a *kop.Ability) {
			if a.Trig.Flags2&kop.Tf2Unblocked != 0 {
				return // an unblocked-attacker line (fireUnblocked)
			}
			if a.Trig.Flags2&kop.Tf2PerPair != 0 {
				s.firePairTrig(k, l, a, ps[:nb])
				return
			}
			for _, as := range atk[:na] {
				if a.Trig.Flags2&kop.Tf2BlockedBySelf != 0 && !s.blocks(l.sl, as) {
					continue
				}
				s.objTrigGen(k, kop.TrigBecomesBlocked, l, a, as, s.perms[as].obj, s.perms[as].ctrl, 0)
			}
		})
	}
}

// fireBlocksGen fires the blocks triggers (TrigBlocks, gorge Mode$ Blocks)
// as blockers are declared like gorge's checkBlocksTriggers: listeners in
// battlefield order (seat 0's list, then seat 1's), each listener's
// abilities in ability order, an ability once per declared (attacker,
// blocker) pair in declaration order whose blocker matches its filter
// (ValidCard$; TfSelf: the listener blocks) and whose attacker matches
// Filter2 (ValidBlocked$). The trigger object is the ATTACKER (gorge's
// TriggerCard), its player the attacker's controller.
//
//go:noinline
func (s *State) fireBlocksGen(k *prep) {
	var ps [2 * MaxPerm]blkPair
	nb := s.blockPairs(&ps)
	if nb == 0 {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.subs[kop.TrigBlocks].and(s.occ), &ord)
	isBlocks := func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigBlocks }
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, isBlocks, func(l trigListener, a *kop.Ability) {
			for _, pr := range ps[:nb] {
				b, at := pr.b, pr.a
				if !s.objEventMatch(k, l, a, b) {
					continue
				}
				if a.Trig.Filter2 != 0 && !s.matchPermGen(k, a.Trig.Filter2, at, l.ctrl, l.sl, layerFull) {
					continue
				}
				if a.Trig.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl || a.Trig.Flags2&kop.Tf2OppTurn != 0 && s.active == l.ctrl {
					continue
				}
				if !s.trigOK(k, l, a) {
					continue
				}
				s.queueTrig(k, l, a, s.perms[at].obj, s.perms[at].ctrl, 0)
			}
		})
	}
}

// firePairTrig fires one by-creature become-blocked ability (Tf2PerPair,
// gorge Mode$ AttackerBlockedByCreature) of listener l once per declared
// (attacker, blocker) pair, blockers in declaration order (gorge's event
// Pairs), whose attacker matches the trigger filter and whose blocker
// matches Filter2. The trigger object is the pair member that is not the
// listener (gorge seeds it as Remembered/TriggerCard).
//
//go:noinline
func (s *State) firePairTrig(k *prep, l trigListener, a *kop.Ability, ps []blkPair) {
	for _, pr := range ps {
		b, at := pr.b, pr.a
		if a.Trig.Filter2 != 0 && !s.matchPermGen(k, a.Trig.Filter2, b, l.ctrl, l.sl, layerFull) {
			continue
		}
		other := b
		if l.sl == b {
			other = at
		}
		s.objTrigGen(k, kop.TrigBecomesBlocked, l, a, at, s.perms[other].obj, s.perms[other].ctrl, 0)
	}
}
