package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// S:Mode$ OptionalAttackCost | Cost$ Exert<1/CARDNAME> (CR 702.100a "you may
// exert CARDNAME as it attacks"; kop.StatOptionalAttackCost, lowered by
// kgen/static_optattack.go; gorge engine/rules/combat.go startExertAsks).
//
// Gorge poses the election once the declaration is answered and folded: after
// the DeclareAttackers events (so after the Attacks triggers are queued and the
// attackers tapped) and after the band asks (CR 702.22c), still inside the
// declare-attackers step, one Min 1 / Max 1 KChoose of kind "exert" per
// declared attacker carrying the offer, in declaration order (chosen order,
// deduped), option 0 declining (no object) and option 1 naming the attacker.
// The kernel poses the same ask in ctlExert from declareAttacksDone (or the
// last band answer), before the priority round.
//
// A yes pays Exert<1/CARDNAME> (exertCost: the one-shot no-untap mark, CR
// 702.100a; gorge's Exert event fold) and queues the card's own Exerted
// triggers (the static's Trigger$ rider, which kgen lowers as a self-Exerted
// trigger: gorge's checkExertTriggers queues it off the same event), after the
// declaration's own triggers. The offer reads the printed static (not under a
// lose-all-abilities effect, as enlistCarrier does) and needs no gate: kgen
// refuses a gated carrier. The cursor is s.lastPick, the band ask's.
const _ = kop.StatOptionalAttackCost

// tableHasExertOffer reports whether some card of t prints the exert offer.
func tableHasExertOffer(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOptionalAttackCost {
				return true
			}
		}
	}
	return false
}

// exertOffered reports whether the declared attacker at sl carries a live exert
// offer (gorge exertOfferHolds: a face-up battlefield attacker whose own
// stat:OptionalAttackCost static applies).
func (s *State) exertOffered(k *prep, sl Slot) bool {
	if s.perms[sl].gty&gtyFaceDown != 0 {
		return false
	}
	if _, lost := s.lossAt(k, sl); lost {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOptionalAttackCost {
			return true
		}
	}
	return false
}

// exertAsk poses the exert election of the next offered attacker after the
// cursor (the first when no election is pending) and reports whether one is
// posed.
func (s *State) exertAsk(k *prep) bool {
	if !k.exertOffer || s.nAttackers == 0 {
		return false
	}
	var ord [MaxPerm]Slot
	n := s.declOrder(&ord)
	i := 0
	if s.ctl == ctlExert {
		for i < n && ord[i] != s.lastPick {
			i++
		}
		i++ // after the attacker just answered
	}
	for ; i < n; i++ {
		if s.exertOffered(k, ord[i]) {
			s.lastPick = ord[i]
			s.ctl = ctlExert
			return true
		}
	}
	return false
}

// exertOptions poses the attacker's election: the decline, then the exert
// (gorge askNextExert's option 0 "Don't exert" and option 1 "Exert").
func (s *State) exertOptions(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChExert, s.active
	d.add(MakeAction(AkChoose, TCode(NoObj), 0, 0))
	d.add(MakeAction(AkChoose, TCode(s.perms[s.lastPick].obj), 0, 1))
}

// applyExert applies one answer: a yes exerts the attacker and queues its
// Exerted triggers; then the next offered attacker is asked, or the priority
// round opens.
func (s *State) applyExert(k *prep, a Action) {
	if a.Mode() == 1 {
		sl := s.lastPick
		s.exertCost(sl)
		s.fireExerted(k, sl)
	}
	if !s.exertAsk(k) {
		s.openPriority()
	}
}

// fireExerted queues the Exerted triggers of the permanent at sl (its own
// lines: the exert offer's rider).
func (s *State) fireExerted(k *prep, sl Slot) {
	p := &s.perms[sl]
	card := s.cardAt(sl)
	cd := &k.t.Cards[card]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigExerted {
			s.enqueue(p.obj, s.objs[p.obj].ver, card, uint8(i), p.ctrl, p.obj, p.ctrl, 0)
		}
	}
}
