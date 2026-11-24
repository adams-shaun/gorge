package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kw:Enlist (CR 702.154; gorge engine/rules/enlist.go): "As this creature
// attacks, you may tap a nonattacking creature you control without summoning
// sickness. When you do, add its power to this creature's power until end of
// turn." The printed K:Enlist is CardDef.Enlist (kop.KwEnlist).
//
// Gorge poses the election once the declaration is answered and its attack
// cost paid, before the declaration's DeclareAttackers events (so before the
// attackers tap and before any Attacks trigger is matched): one Min 0 / Max 1
// KChoose of kind "enlist" per declared attacker with Enlist that has an
// eligible creature, in declaration order, option 0 declining (no object)
// and then one option per eligible creature in its controller's battlefield
// order. The kernel poses the same ask in ctlEnlist from finishAttacks, after
// atkTaxPay and before the rest of the declaration (declareAttacksDone).
//
// An eligible creature is one the attacker's controller controls, untapped,
// not declared as an attacker, without summoning sickness (pfSick; gorge's
// SummonSick, so a hasty newcomer does not qualify either) and not the
// attacker itself. Gorge re-reads the candidates per ask (an earlier answer
// taps one), and so does the kernel. A yes taps the creature (its Taps
// triggers fire with the tap, ahead of the declaration's) and gives the
// attacker +X/+0 until end of turn, X the enlisted creature's power at that
// moment (CR 702.154a; gorge's UntilEOT LPT modify, the kernel's end-of-turn
// pump, eotAdd).
//
// Gorge reads the derived keyword (hasKeywordH): the kernel reads the
// printed field and drops it under a lose-all-abilities effect (lossAt, as
// unleashNoBlock does); a GRANTED Enlist is refused at lowering. A face-down
// permanent has no abilities (CR 708.2). The Enlist event's per-combat stamp
// is read only by enlistedThisCombat and Mode$ Enlisted, both refused.
//
// The cursor is s.lastPick (the attacker asked), as the band ask's
// (combat_band.go): applyAttack's declaration order is done with it once the
// declaration's Done is answered.
const _ = kop.KwEnlist

// tableHasEnlist reports whether some card of t prints Enlist.
func tableHasEnlist(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].Enlist {
			return true
		}
	}
	return false
}

// enlistCarrier reports whether the attacker at sl has its printed Enlist.
func (s *State) enlistCarrier(k *prep, sl Slot) bool {
	if !k.t.Cards[s.cardAt(sl)].Enlist || s.perms[sl].gty&gtyFaceDown != 0 {
		return false
	}
	_, lost := s.lossAt(k, sl)
	return !lost
}

// enlistCands lists the creatures the attacker at atk may enlist, in its
// controller's battlefield order (gorge enlistCandidates walks the zone).
func (s *State) enlistCands(k *prep, atk Slot, out *[MaxPerm]Slot) int {
	ctrl := s.perms[atk].ctrl
	var m slotMask
	for x := s.ctrlM[ctrl].and(s.occ); x.any(); x = x.rest() {
		sl := x.first()
		p := &s.perms[sl]
		if sl == atk || p.flags&(pfTapped|pfAttacking|pfSick) != 0 || s.typesAt(k, sl)&kop.TCreature == 0 {
			continue
		}
		m = m.with(sl)
	}
	return s.battlefieldOrder(ctrl, m, out[:], 0)
}

// enlistAsk poses the election of the next carrier after the cursor (the
// first when no election is pending) and reports whether one is posed.
func (s *State) enlistAsk(k *prep) bool {
	if !k.enlist || k.gen == nil || s.nAttackers == 0 {
		return false
	}
	var ord, c [MaxPerm]Slot
	n := s.declOrder(&ord)
	i := 0
	if s.ctl == ctlEnlist {
		for i < n && ord[i] != s.lastPick {
			i++
		}
		i++ // after the attacker just answered
	}
	for ; i < n; i++ {
		if s.enlistCarrier(k, ord[i]) && s.enlistCands(k, ord[i], &c) > 0 {
			s.lastPick = ord[i]
			s.ctl = ctlEnlist
			return true
		}
	}
	return false
}

// enlistOptions poses the attacker's election: the decline, then each
// eligible creature, then Done (gorge's ask is Min 0: an empty answer
// declines too).
func (s *State) enlistOptions(k *prep, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChEnlist, s.perms[s.lastPick].ctrl
	d.add(MakeAction(AkChoose, TCode(NoObj), 0, 0))
	var c [MaxPerm]Slot
	n := s.enlistCands(k, s.lastPick, &c)
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, TCode(s.perms[c[i]].obj), 0, uint8(1+i)))
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// applyEnlist applies one answer: a yes taps the creature and pumps the
// attacker by its power (the decline and Done enlist nothing); then the next
// carrier is asked, or the declaration goes on.
func (s *State) applyEnlist(k *prep, a Action) {
	if a.Kind() == AkChoose && a.Mode() != 0 {
		atk, sl := s.lastPick, s.objs[a.Source()].slot
		ctrl := s.perms[atk].ctrl
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		if s.subs[kop.TrigTaps].any() {
			s.fireTapsBy(k, sl, ctrl)
		}
		s.markTapped(k, sl)
		if pw := s.power(k, sl); pw != 0 {
			s.touchSl(atk)
			s.eotAdd(k, atk, pw, 0)
		}
	}
	if !s.enlistAsk(k) {
		s.declareAttacksDone(k)
	}
}
