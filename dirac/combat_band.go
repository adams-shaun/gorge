package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 702.22c band declaration ask. gorge (engine/rules/banding.go
// startBandAsks) poses it once the attack declaration is answered -- its
// events folded, the attackers tapped, the Attacks triggers queued, none yet
// placed -- for each declared attacker with banding that has a candidate:
// one Min 1 / Max 1 KChoose per leader in declaration order, option 0
// declining, then one option per other attacker of the same controller and
// defender in declaration order. The kernel poses the same ask in ctlBand
// from finishAttacks (before the pending triggers are placed) and mirrors
// the decline.
//
// Band formation itself (CR 702.22d-k: the band blocked as a group, the
// damage division choosers) is NOT modelled: an accepted candidate stalls the
// game at StallBand. The ask is mirrored so the grant is observable, never
// so a band forms. A leader is a granted banding (kop.Override.Banding,
// hasBanding): the kernel has no printed banding. No candidate is ever in a
// band (none forms), so gorge's "no plain creature once one joined" rule
// never filters.
//
// The cursor is s.lastPick, a write-only field once the declaration is done
// (applyAttack's declaration order; the blockers declaration resets it): the
// leader asked, so a decline resumes after it.

// StallBand: a band declaration answered with a candidate (CR 702.22c). The
// kernel poses gorge's ask and mirrors the decline; band formation is not
// modelled.
const StallBand = 164

func init() { stallNames[StallBand] = "band" }

// declOrder lists the active player's attackers in declaration order.
func (s *State) declOrder(out *[MaxPerm]Slot) int {
	n := 0
	for m := s.ctrlM[s.active].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfAttacking == 0 {
			continue
		}
		j := n
		for ; j > 0 && s.perms[out[j-1]].bord > s.perms[sl].bord; j-- {
			out[j] = out[j-1]
		}
		out[j] = sl
		n++
	}
	return n
}

// bandDefender is the defender an attacker attacks as gorge compares it for a
// band (Object.Attacking, Object.AttackingBattle): the defending seat and the
// attacked battle, if any.
func (s *State) bandDefender(k *prep, sl Slot) (seat uint8, battle Slot) {
	battle = NoSlot
	if q := s.attackedPerm(k, sl); q != NoSlot && s.typesAt(k, q)&kop.TBattle != 0 {
		battle = q
	}
	return s.atkSeat(k, sl), battle
}

// bandCands lists the attackers that may join leader's band, in declaration
// order: attacking the same defender (gorge bandCandidates).
func (s *State) bandCands(k *prep, leader Slot, out *[MaxPerm]Slot) int {
	var ord [MaxPerm]Slot
	no := s.declOrder(&ord)
	seat, battle := s.bandDefender(k, leader)
	n := 0
	for i := 0; i < no; i++ {
		sl := ord[i]
		if sl == leader {
			continue
		}
		if sq, bt := s.bandDefender(k, sl); sq == seat && bt == battle {
			out[n] = sl
			n++
		}
	}
	return n
}

// bandAsk poses the band ask of the next leader after the cursor (the first
// leader when no ask is pending) and reports whether one is posed.
func (s *State) bandAsk(k *prep) bool {
	if k.gen == nil || s.nAttackers < 2 {
		return false // the grant read is a generated table's; a band needs a second attacker
	}
	var ord, c [MaxPerm]Slot
	n := s.declOrder(&ord)
	i := 0
	if s.ctl == ctlBand {
		for i < n && ord[i] != s.lastPick {
			i++
		}
		i++ // after the leader just answered
	}
	for ; i < n; i++ {
		if s.hasBanding(k, ord[i]) && s.bandCands(k, ord[i], &c) > 0 {
			s.lastPick = ord[i]
			s.ctl = ctlBand
			return true
		}
	}
	return false
}

// bandOptions poses the leader's ask: the decline, then each candidate.
func (s *State) bandOptions(k *prep, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChBand, s.active
	d.add(MakeAction(AkChoose, TCode(NoObj), 0, 0))
	var c [MaxPerm]Slot
	n := s.bandCands(k, s.lastPick, &c)
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, TCode(s.perms[c[i]].obj), 0, uint8(1+i)))
	}
}

// applyBand applies one answer: the decline moves to the next leader, and
// the last one hands the declare-attackers step on to its priority round; a
// candidate would form a band, which the kernel does not model.
func (s *State) applyBand(k *prep, a Action) {
	if a.Mode() != 0 {
		s.stall(StallBand)
		return
	}
	if !s.bandAsk(k) {
		s.openPriority()
	}
}

// bandDivider is who divides the combat damage of s.divA, whose options d
// holds (gorge divisionChooser). CR 702.22j: an attacker blocked by a
// creature with banding is divided by the defending player instead of its
// controller. CR 702.22k: a blocker blocking a creature with banding is
// divided by the attacking player instead of its controller. Only a granted
// banding reaches either (hasBanding): gorge's printed banding has no kernel
// card.
func (s *State) bandDivider(k *prep, d *Decision) {
	if k.gen == nil {
		return
	}
	if s.perms[s.divA].flags&pfBlocking != 0 {
		for am := s.attackingMask(); am.any(); am = am.rest() {
			if a := am.first(); s.blocks(s.divA, a) && s.hasBanding(k, a) {
				d.Player = s.active
				return
			}
		}
		return
	}
	var bl [MaxPerm]Slot
	for i, nb := 0, s.liveBlockers(s.divA, &bl); i < nb; i++ {
		if s.hasBanding(k, bl[i]) {
			d.Player = s.atkSeat(k, s.divA)
			return
		}
	}
}
