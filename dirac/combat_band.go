package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The CR 702.22c band declaration ask. gorge (engine/rules/banding.go
// startBandAsks) poses it once the attack declaration is answered -- its
// events folded, the attackers tapped, the Attacks triggers queued, none yet
// placed -- for each declared attacker with banding that has a candidate:
// one Min 1 / Max 1 KChoose per leader in declaration order, option 0
// declining, then one option per other attacker of the same controller and
// defender in declaration order. The kernel poses the same ask in ctlBand
// from finishAttacks (before the pending triggers are placed) and forms the
// band an accepted candidate names.
//
// Band membership (CR 702.22c-e) is a band id in perm.atkPl's high nibble
// plus one mark on the members that joined a leader: the founder of a band is
// its only member without cseenBandMate. It adds no fixed State field
// (TestNoNewFixedStateFields) and no cold region. A leader is a printed
// banding (CardDef.Banding) or a granted one (kop.Override.Banding,
// hasBanding). An accepted candidate re-asks the same leader (a band grows),
// and a band that already holds a creature without banding takes only banding
// creatures (gorge bandCandidates).
//
// CR 702.22f/h: a member removed from combat leaves the band (leaveCombat,
// ctrlUncombat, phaseOut), and a blocker declared against one member also
// blocks every mate (bandExpandBlocks, from finishBlocks, in the declared
// pairs' order). The 702.22j/k damage choosers are bandDivider below.
//
// The cursor is s.lastPick, a write-only field once the declaration is done
// (applyAttack's declaration order; the blockers declaration resets it): the
// leader asked, so a decline resumes after it and an accepted candidate
// re-asks it.

// StallBand: a band declaration answered with a candidate (CR 702.22c). No
// longer raised: the kernel forms the band (combat_band.go). The value and
// its class stay so old recordings name the same reason.
const StallBand = 164

func init() { stallNames[StallBand] = "band" }

// useBanding marks a table with a banding source: a printed CardDef.Banding
// or an override record's kop.Override.Banding (a Pump's KW$ Banding, a
// Continuous static's AddKeyword$ Banding). Only such a table allocates the
// further-block records a band's 702.22h expansion needs.
var useBanding = newGenUse(tableHasBanding)

func tableHasBanding(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Banding {
			return true
		}
	}
	for i := range t.Overrides {
		if t.Overrides[i].Banding {
			return true
		}
	}
	return false
}

// A band's identity rides in perm.atkPl's high nibble (the low nibble is the
// attacked player's seat + 1): 0 is no band, 1..maxBands a band. The member
// that formed the band (the leader) is the only one without the
// cseenBandMate mark, so a slot's band dies with the permanent that occupied
// it (enterNF's perm{}). maxBands caps one combat at fifteen simultaneous
// bands; a sixteenth stalls at StallBandCap, a declared capacity.
const (
	maxBands      = 15
	atkPlBandMask = 0xF0 // perm.atkPl's high nibble: the band id
	atkPlSeatMask = 0x0F // perm.atkPl's low nibble: the attacked player's seat + 1
)

// StallBandCap: a sixteenth attacking band in one combat. perm.atkPl's high
// nibble holds maxBands ids, and gorge has no limit.
const StallBandCap = 77

func init() { stallNames[StallBandCap] = "bandcap" }

// bandID is the band id of the permanent at sl (0 none).
func (s *State) bandID(sl Slot) uint8 { return s.perms[sl].atkPl >> 4 & 0xF }

// setBandID writes sl's band id, keeping the attacked-player seat.
func (s *State) setBandID(sl Slot, id uint8) {
	s.perms[sl].atkPl = s.perms[sl].atkPl&atkPlSeatMask | id&0xF<<4
}

// bandOf is the founder of the band sl is in (NoSlot when it is in none): the
// member without the cseenBandMate mark. The id is the band's identity even
// after the founder leaves combat (CR 702.22e): the remaining members still
// share it, as gorge's BandMates does.
func (s *State) bandOf(sl Slot) Slot {
	id := s.bandID(sl)
	if id == 0 {
		return NoSlot
	}
	for m := s.ctrlM[s.perms[sl].ctrl].and(s.occ); m.any(); m = m.rest() {
		if o := m.first(); s.bandID(o) == id && s.perms[o].cseen&cseenBandMate == 0 {
			return o
		}
	}
	return NoSlot
}

// bandFreeID is the lowest band id no permanent holds (0: every id is taken).
// A band id lives on each member, so the used ids are the battlefield's.
func (s *State) bandFreeID() uint8 {
	var used uint16
	for m := s.occ; m.any(); m = m.rest() {
		used |= 1 << s.bandID(m.first())
	}
	for id := uint8(1); id <= maxBands; id++ {
		if used&(1<<id) == 0 {
			return id
		}
	}
	return 0
}

// bandJoin puts cand in leader's band, giving the leader a free band id when
// it has none. False when every id is taken (StallBandCap).
func (s *State) bandJoin(leader, cand Slot) bool {
	id := s.bandID(leader)
	if id == 0 {
		id = s.bandFreeID()
		if id == 0 {
			return false
		}
		s.setBandID(leader, id)
	}
	s.setBandID(cand, id)
	s.perms[cand].cseen |= cseenBandMate
	return true
}

// bandClearSlot removes sl from its band (CR 702.22f: a creature removed from
// combat leaves the band). The caller touches the slot.
func (s *State) bandClearSlot(sl Slot) {
	s.perms[sl].atkPl &^= atkPlBandMask
	s.perms[sl].cseen &^= cseenBandMate
}

// bandClear ends every band, ids and per-permanent records.
func (s *State) bandClear() {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.bandID(sl) != 0 || s.perms[sl].cseen&cseenBandMate != 0 {
			s.bandClearSlot(sl)
			s.touchSl(sl)
		}
	}
}

// anyBand reports whether any permanent is in a band now.
func (s *State) anyBand() bool {
	for m := s.occ; m.any(); m = m.rest() {
		if s.bandID(m.first()) != 0 {
			return true
		}
	}
	return false
}

// bandMates writes sl's other band members in its controller's battlefield
// order (gorge state.Game.BandMates) and returns how many.
func (s *State) bandMates(sl Slot, out *[MaxPerm]Slot) int {
	id := s.bandID(sl)
	if id == 0 {
		return 0
	}
	var m slotMask
	for x := s.ctrlM[s.perms[sl].ctrl]; x.any(); x = x.rest() {
		if o := x.first(); o != sl && s.perms[o].flags&pfAttacking != 0 && s.bandID(o) == id {
			m = m.with(o)
		}
	}
	return s.battlefieldOrder(s.perms[sl].ctrl, m, out[:], 0)
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
// order (gorge bandCandidates, engine/rules/banding.go): attacking the same
// defender, under the same controller, in no band yet, and -- once the band
// holds a creature without banding -- with banding themselves. A leader
// already in another leader's band has none.
func (s *State) bandCands(k *prep, leader Slot, out *[MaxPerm]Slot) int {
	lid := s.bandID(leader)
	if lid != 0 && s.bandOf(leader) != leader {
		return 0 // already a member of another band (gorge's Band != 0 && Band != leader)
	}
	var ord [MaxPerm]Slot
	no := s.declOrder(&ord)
	plainTaken := false
	if lid != 0 {
		for i := 0; i < no; i++ {
			if sl := ord[i]; sl != leader && s.bandID(sl) == lid && !s.hasBanding(k, sl) {
				plainTaken = true
			}
		}
	}
	seat, battle := s.bandDefender(k, leader)
	n := 0
	for i := 0; i < no; i++ {
		sl := ord[i]
		if sl == leader || s.bandID(sl) != 0 || s.perms[sl].ctrl != s.perms[leader].ctrl {
			continue
		}
		if sq, bt := s.bandDefender(k, sl); sq == seat && bt == battle {
			if plainTaken && !s.hasBanding(k, sl) {
				continue
			}
			out[n] = sl
			n++
		}
	}
	return n
}

// bandAsk poses the band ask of the next leader after the cursor (the first
// leader when no ask is pending) and reports whether one is posed. same
// re-asks the leader the cursor names (an accepted candidate grew it).
func (s *State) bandAsk(k *prep) bool { return s.bandAskFrom(k, false) }

func (s *State) bandAskFrom(k *prep, same bool) bool {
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
		if !same {
			i++ // after the leader just answered
		}
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

// applyBand applies one answer: an accepted candidate joins the leader's band
// (CR 702.22c) and re-asks the leader, so a band grows one creature per
// answer; the decline moves to the next leader, and the last one hands the
// declare-attackers step on to its priority round.
func (s *State) applyBand(k *prep, a Action) {
	if a.Mode() != 0 {
		leader := s.lastPick
		cand := s.objs[a.Source()].slot
		if !s.bandJoin(leader, cand) {
			s.stall(StallBandCap)
			return
		}
		s.touchSl(leader)
		s.touchSl(cand)
		if !s.bandAskFrom(k, true) && !s.exertAsk(k) {
			s.openPriority()
		}
		return
	}
	if !s.bandAsk(k) && !s.exertAsk(k) { // gorge asks the exert elections after the bands (combat_exert.go)
		s.openPriority()
	}
}

// bandExpandBlocks is CR 702.22h: a blocker declared against one member of a
// band also blocks every other member (gorge bandBlockPairs). The declared
// pairs are read in their declaration order, and each recorded mate follows
// them in the same further-block table an "additional creature" block uses
// (combat_extrablock.go), so the block list, the damage division and the
// 509.1h markers all see one blocker of several attackers. A table that would
// need a ninth further-block record stops at StallExtraBlockCap, a declared
// capacity.
func (s *State) bandExpandBlocks(k *prep) {
	if k.gen == nil || !k.gen.uses.has(useBanding) || !s.anyBand() {
		return
	}
	var pairs [2 * MaxPerm]blkPair
	n := s.blockPairs(&pairs) // the declared pairs, in declaration order
	for i := 0; i < n; i++ {
		b, a := pairs[i].b, pairs[i].a
		id := s.bandID(a)
		if id == 0 {
			continue
		}
		var mates [MaxPerm]Slot
		nm := s.bandMates(a, &mates)
		for j := 0; j < nm; j++ {
			m := mates[j]
			if s.bandID(m) != id || s.blocks(b, m) {
				continue
			}
			if !s.xblkAdd(b, m, s.blockSeq) {
				s.stall(StallExtraBlockCap)
				return
			}
			s.blockSeq++
			mp := &s.perms[m]
			mp.nblock++
			mp.flags |= pfBlocked
			s.touchSl(m)
		}
		s.touchSl(b)
	}
}

// bandDivider is who divides the combat damage of s.divA, whose options d
// holds (gorge divisionChooser). CR 702.22j: an attacker blocked by a
// creature with banding is divided by the defending player instead of its
// controller. CR 702.22k: a blocker blocking a creature with banding is
// divided by the attacking player instead of its controller. Only a banding
// reaches either (hasBanding), printed or granted.
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
	if s.defenderAssigns(k, s.divA) {
		d.Player = s.atkSeat(k, s.divA) // Defensive Formation (static_defenderassigns.go)
	}
}
