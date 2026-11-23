package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
// Band formation (CR 702.22d-k) is modelled for the blocks of up to two
// attackers: an accepted candidate joins the asked leader's band (coldBand: a
// per-slot band id, the leader's slot + 1), the leader is asked again, and a
// blocker declared against a member blocks the other members too
// (bandBlocks, CR 702.22h) through the two-attacker blocker records
// (combat_extrablock.go), whose damage division is the band blocker's
// (CR 702.22j/k: bandDivider). A leader is a granted banding
// (kop.Override.Banding, hasBanding): the kernel has no printed banding. A
// band blocked by a creature that would need a third block stalls (StallBand).
//
// The cursor is s.lastPick, a write-only field once the declaration is done
// (applyAttack's declaration order; the blockers declaration resets it): the
// leader asked, so a decline resumes after it.

// StallBand: a blocker declared against a member of a band that would block a
// third attacker (CR 702.22h): the kernel's blocker records hold two.
const StallBand = 164

func init() { stallNames[StallBand] = "band" }

// useBanding: an override record grants banding (kop.Override.Banding): the
// band records and the second-block table (combat_extrablock.go) are laid out.
var useBanding = newGenUse(func(t *kop.Table) bool {
	for i := range t.Overrides {
		if t.Overrides[i].Banding {
			return true
		}
	}
	return false
})

// coldBand is each slot's band id: its leader's slot + 1, 0 for a creature in
// no band (nil in a pool with no banding grant). A per-slot region, so a slot
// that takes a new permanent starts in no band; bandClear drops every band as
// combat ends.
var coldBand = newCold[uint8]("band", func(g *genPrep) int {
	if g.uses.has(useBanding) {
		return MaxPerm
	}
	return 0
}).perSlot()

// bandOf is sl's band id (0: in no band).
func (s *State) bandOf(sl Slot) uint8 {
	if b := coldBand.of(s); b != nil {
		return b[sl]
	}
	return 0
}

// bandClear ends every band (CR 702.22e).
func (s *State) bandClear() { clear(coldBand.of(s)) }

// bandMates lists the other attackers in sl's band in battlefield (ts) order,
// as gorge's BandMates.
func (s *State) bandMates(sl Slot, out *[MaxPerm]Slot) int {
	id := s.bandOf(sl)
	if id == 0 || s.perms[sl].flags&pfAttacking == 0 {
		return 0
	}
	n := 0
	for m := s.ctrlM[s.active].and(s.occ); m.any(); m = m.rest() {
		q := m.first()
		if q == sl || s.perms[q].flags&pfAttacking == 0 || s.bandOf(q) != id {
			continue
		}
		j := n
		for ; j > 0 && s.perms[out[j-1]].ts > s.perms[q].ts; j-- {
			out[j] = out[j-1]
		}
		out[j] = q
		n++
	}
	return n
}

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
// attacked planeswalker or battle, if any (CR 702.22d).
func (s *State) bandDefender(k *prep, sl Slot) (seat uint8, battle Slot) {
	battle = NoSlot
	if q := s.attackedPerm(k, sl); q != NoSlot {
		battle = q // attackedPerm yields only a planeswalker or a battle
	}
	return s.atkSeat(k, sl), battle
}

// bandCands lists the attackers that may join leader's band, in declaration
// order: attacking the same defender, in no band yet and, once leader's band
// holds a creature without banding, with banding (gorge bandCandidates). A
// leader taken into another leader's band has none.
func (s *State) bandCands(k *prep, leader Slot, out *[MaxPerm]Slot) int {
	if id := s.bandOf(leader); id != 0 && id != uint8(leader)+1 {
		return 0
	}
	var ord, mates [MaxPerm]Slot
	no := s.declOrder(&ord)
	plainTaken := false
	for i, nm := 0, s.bandMates(leader, &mates); i < nm; i++ {
		if !s.hasBanding(k, mates[i]) {
			plainTaken = true
		}
	}
	seat, battle := s.bandDefender(k, leader)
	n := 0
	for i := 0; i < no; i++ {
		sl := ord[i]
		if sl == leader || s.bandOf(sl) != 0 || plainTaken && !s.hasBanding(k, sl) {
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

// applyBand applies one answer: a candidate joins the asked leader's band
// (CR 702.22c) and the leader is asked again while a candidate is left; the
// decline moves to the next leader, and the last one hands the
// declare-attackers step on to its priority round.
func (s *State) applyBand(k *prep, a Action) {
	if a.Mode() != 0 {
		leader := s.lastPick
		if b := coldBand.of(s); b != nil {
			b[leader] = uint8(leader) + 1
			b[s.objs[a.Source()].slot] = uint8(leader) + 1
		} else {
			s.stall(StallBand)
			return
		}
		var c [MaxPerm]Slot
		if s.bandCands(k, leader, &c) > 0 {
			return // gorge asks the same leader again
		}
	}
	if !s.bandAsk(k) {
		s.openPriority()
	}
}

// bandBlocks is CR 702.22h, run once the blocks are declared and checked: a
// blocker blocking one member of a band blocks every other member too
// (gorge bandBlockPairs). The new pairs follow the declared ones, in their
// order then the band's battlefield order. A blocker that would block a third
// attacker, or a ninth blocker of two, stalls the game.
func (s *State) bandBlocks(k *prep) {
	if coldBand.of(s) == nil {
		return
	}
	var ps [2 * MaxPerm]blkPair
	np := s.blockPairs(&ps)
	for i := 0; i < np; i++ {
		var mates [MaxPerm]Slot
		b := ps[i].b
		for j, nm := 0, s.bandMates(ps[i].a, &mates); j < nm; j++ {
			m := mates[j]
			if s.blocks(b, m) {
				continue
			}
			s.blockSeq++
			if s.xblkOf(b) != NoSlot || !s.xblkAdd(b, m, s.blockSeq) {
				s.stall(StallBand)
				return
			}
			s.perms[m].nblock++
			s.perms[m].flags |= pfBlocked
			s.touchSl(m)
			s.touchSl(b)
		}
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
