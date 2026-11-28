package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The ability union of a merged permanent (CR 702.140d: it "has all abilities
// of each card" in its pile; gorge engine/state/pile.go's Pile/Abilities walk,
// folded by events.Apply's Mutate). kw_mutate.go keeps the pile's data: the
// top card stays on the survivor (perm.card), so every characteristic read
// sees the top face (CR 730.2a), and the components under it park in
// kop.ZoneNone in mergeRec.und[]. The abilities of those under cards are this
// file's: a count/at walk over the record that each ability consumer runs
// after its walk of the top card.
//
//   - subscription: the under cards' triggers, statics and grants subscribe on
//     the survivor's slot (pileSubscribe, after each fold; pileUnsubscribe
//     when the pile leaves). Only abilities: an under card's types add no
//     legendary bit.
//   - keywords: kwGen and the layer-6 walk's base read pileKw. The printed
//     records outside KwMask (CR 702.140e: infect, wither, toxic, evasion,
//     printedKeyword's words) are read through pilePrints and eachPileCard,
//     and each card's ward is its own trigger (printedWards).
//   - statics: eachStaticOf, eachStaticC, eachStaticTS and mayPlayZones walk
//     pileStatics after the top card; the class gates read statClsAt. A
//     top-card swap that stays on the battlefield resubscribes the pile
//     (subscribeTop); phasing drops and restores it.
//   - triggers: slotTriggers walks pileTriggers after the printed walk
//     (trig_mutates.go already walks the pile for TrigMutates).
//   - activated abilities: pileActivations offers each under card's
//     activated abilities as a gained activation (static_gainsof.go's
//     AkActivate{source: the pile, target: the under component's object,
//     mode: 0xC0 | ability}); activateGen reads the parked object's card.
//
// Every arm is gated on pileAt: a table with no Mutate card and no merge op
// never walks the cold region. A pile that has lost its abilities loses the
// under cards' too, keeping what the top card's walk keeps (CR 613.1f: they
// are its abilities).

// pileAt is the record of the merged permanent at sl, nil when sl is not a
// pile or the table builds none (prep.piles: one inlinable branch).
func (s *State) pileAt(k *prep, sl Slot) *mergeRec {
	if !k.piles || sl == NoSlot {
		return nil
	}
	return s.pileRec(sl)
}

// pileRec is pileAt's record read on a table that builds piles.
func (s *State) pileRec(sl Slot) *mergeRec {
	if r := s.mergeRecOf(s.perms[sl].obj); r != nil && r.n != 0 {
		return r
	}
	return nil
}

// The hot static and layer scans whose body is too large to keep twice walk
// eachPileCard's cards in its order with no closure, one inlined branch on a
// table with no pile:
//
//	pn := 0
//	if k.piles {
//		pn = s.pileNAt(sl)
//	}
//	for pj := 0; pj <= pn; pj++ {
//		c := s.cardAt(sl)
//		if pj > 0 {
//			c = s.pileUnder(k, sl, pj-1)
//		}
//		...
//	}
//
// The small scans keep the top-card-only loop for such a table and move the
// pile walk to a *Pile twin (grantSrcScanPile, lfxCollectStaticsPile,
// charsFxMayPile).

// pileNAt is the number of under cards of the pile at sl (0: not a pile)
// on a table that builds piles.
func (s *State) pileNAt(sl Slot) int {
	if sl == NoSlot {
		return 0
	}
	if r := s.pileRec(sl); r != nil {
		return int(r.n)
	}
	return 0
}

// pileUnder is the under card j of the pile at sl.
func (s *State) pileUnder(k *prep, sl Slot, j int) CardID { return s.underCard(s.pileAt(k, sl), j) }

// underObj is the record's component j (0 is the first under the top).
func underObj(r *mergeRec, j int) Obj { return Obj(r.und[j] - 1) }

// underCard is component j's card: an entry stores the object, whose CardID
// names its face (turnFace, face_multiface.go).
func (s *State) underCard(r *mergeRec, j int) CardID { return s.objs[underObj(r, j)].card }

// pileKw is the printed keywords of the under cards of the pile at sl (0 for
// a permanent that is not one). The caller has checked the loss.
func (s *State) pileKw(k *prep, sl Slot) kop.KwMask {
	if !k.piles {
		return 0
	}
	return s.pileKwAt(k, sl)
}

// pileKwAt is pileKw on a table that builds piles.
func (s *State) pileKwAt(k *prep, sl Slot) kop.KwMask {
	r := s.pileAt(k, sl)
	if r == nil {
		return 0
	}
	var m kop.KwMask
	for j := 0; j < int(r.n); j++ {
		m |= k.cards[s.underCard(r, j)].kw
	}
	return m
}

// statClsAt is the static classes the permanent at sl prints: its top card's
// and its under cards'.
func (s *State) statClsAt(k *prep, sl Slot) uint16 {
	c := k.gen.sCls[s.cardAt(sl)]
	if !k.piles {
		return c
	}
	if r := s.pileAt(k, sl); r != nil {
		for j := 0; j < int(r.n); j++ {
			c |= k.gen.sCls[s.underCard(r, j)]
		}
	}
	return c
}

// pileSubscribe (un)subscribes the under cards' abilities on sl.
func (s *State) pileSubscribe(k *prep, sl Slot, r *mergeRec, on bool) {
	if sl == NoSlot {
		return
	}
	for j := 0; j < int(r.n); j++ {
		s.subscribeAbilities(sl, &k.cards[s.underCard(r, j)], on)
	}
}

// subscribeTop is subscribe for a top-card swap on a permanent that stays on
// the battlefield (a transform, a door, a copy layer, a face-down turn): the
// top's unsubscribe clears a trigger-event, static or grant bit an under card
// of a pile at sl shares, so the under cards' subscriptions are restored
// after either half of the swap.
func (s *State) subscribeTop(k *prep, sl Slot, pc *pcard, on bool) {
	s.subscribe(sl, pc, on)
	if r := s.pileAt(k, sl); r != nil {
		s.pileSubscribe(k, sl, r, true)
	}
}

// pileUnsubscribe drops the under cards' subscriptions as the pile at sl
// leaves the battlefield (removePerm, before mergeLeave splits the record) or
// phases out (CR 702.26b; phaseIn's subscribeTop restores them).
func (s *State) pileUnsubscribe(k *prep, sl Slot) {
	if r := s.pileAt(k, sl); r != nil {
		s.pileSubscribe(k, sl, r, false)
	}
}

// pileStatics calls f for each static and replacement ability the under cards
// of the pile at sl print, in pile order, as the static walks call it for the
// top card: keep (nil: all) filters by card, cls (0: all) by static class.
// A pile that has lost its abilities keeps what the caller's top walk keeps
// (CR 613.1f): with layered, the effects that began before layer 6
// (keptOnLossAt, eachStaticC's rule); without, only an override-gated one
// (ovGateKeptOnLoss, eachStaticOf's). It reports false when f stopped the
// walk.
func (s *State) pileStatics(k *prep, sl Slot, keep []bool, cls uint16, layered bool, f func(st statSrc) bool) bool {
	if !k.piles {
		return true
	}
	return s.pileStaticsAt(k, sl, keep, cls, layered, f)
}

// pileStaticsAt is pileStatics on a table that builds piles.
func (s *State) pileStaticsAt(k *prep, sl Slot, keep []bool, cls uint16, layered bool, f func(st statSrc) bool) bool {
	r := s.pileAt(k, sl)
	if r == nil {
		return true
	}
	lost := s.genLost(k, sl)
	p := &s.perms[sl]
	for j := 0; j < int(r.n); j++ {
		c := s.underCard(r, j)
		if keep != nil && !keep[c] || cls != 0 && k.gen.sCls[c]&cls == 0 {
			continue
		}
		cd := &k.t.Cards[c]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if lost && !(layered && s.keptOnLossAt(k, sl, a) || !layered && s.ovGateKeptOnLoss(k, sl, a)) {
				continue
			}
			if a.Kind == kop.AbStatic || a.Kind == kop.AbReplacement {
				if !f(statSrc{a: a, sl: sl, ctrl: p.ctrl, obj: p.obj}) {
					return false
				}
			}
		}
	}
	return true
}

// eachPileCard calls f for the card at sl (the top card) and then for each
// under card of the pile at sl, in pile order, until f returns false. It
// reports false when f stopped. A permanent that is not a pile calls f once.
// The caller has checked the loss: a pile that lost its abilities loses the
// under cards' too (CR 613.1f).
func (s *State) eachPileCard(k *prep, sl Slot, f func(c CardID) bool) bool {
	if !k.piles {
		return f(s.cardAt(sl))
	}
	if !f(s.cardAt(sl)) {
		return false
	}
	r := s.pileAt(k, sl)
	if r == nil {
		return true
	}
	for j := 0; j < int(r.n); j++ {
		if !f(s.underCard(r, j)) {
			return false
		}
	}
	return true
}

// pilePrints reports whether a card of the permanent at sl, its top card or
// one beneath it, prints what has reads (CR 702.140e). The caller applies the
// loss of abilities.
func (s *State) pilePrints(k *prep, sl Slot, has func(cd *kop.CardDef) bool) bool {
	return !s.eachPileCard(k, sl, func(c CardID) bool { return c == kop.NoCard || !has(&k.t.Cards[c]) })
}

// prepPileMana registers, for a table that builds piles, each card's plain
// mana ability in genPrep.grantPC so grantIdxs can offer an under card's
// ability on the pile's source slot (CR 702.140d: the merged permanent has
// every ability of its components; a mana ability is paid during a payment,
// never offered at priority). Only the planner's plain shapes are entries;
// an under card's dynamic or restricted one stays the census gap it is for
// the card's own slot.
func prepPileMana(t *kop.Table, g *genPrep) {
	if !g.anyMutate && !g.anyMerge {
		return
	}
	seen := make(map[grantRef]bool, len(g.grantOf))
	for _, r := range g.grantOf {
		seen[r] = true
	}
	for i := range t.Cards {
		cd := &t.Cards[i]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if !gainedManaPlanned(cd, a) || seen[grantRef{CardID(i), uint8(j)}] {
				continue
			}
			var gp pcard
			gp.manaAb, gp.manaCol, gp.manaN = int8(j), a.ManaColors, max(a.ManaN, 1)
			gp.manaOne = manaOneColour(a, gp.manaN)
			prepGrantCost(&gp, a)
			self := a.AFlags&kop.AfManaReflect != 0 && a.ManaAmong == 0
			if self {
				g.prepSelfGrant(gp)
			}
			g.addGrantPC(gp, self, a, CardID(i), uint8(j))
		}
	}
}

// pileTriggers is slotTriggers' printed walk over the under cards of the pile
// at sl, in pile order (gorge MergedTriggerPush). The caller has checked the
// loss.
func (s *State) pileTriggers(k *prep, sl Slot, ev func(a *kop.Ability) bool, f func(l trigListener, a *kop.Ability)) {
	r := s.pileAt(k, sl)
	if r == nil {
		return
	}
	p := &s.perms[sl]
	for j := 0; j < int(r.n); j++ {
		c := s.underCard(r, j)
		cd := &k.t.Cards[c]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbTriggered && ev(a) && a.AFlags&kop.AfLatent == 0 && (a.Trig.Zone == kop.ZoneNone || a.Trig.Zone == kop.ZoneBattlefield) && (!k.anyGate || s.gateOK(sl, a)) {
				f(trigListener{obj: p.obj, sl: sl, ctrl: p.ctrl, card: c, ab: uint8(i)}, a)
			}
		}
	}
}

// pileActivations offers p's piles' under-card activated abilities, each as
// a gained activation of the pile (static_gainsof.go's gainedOffer: the walk
// stalls with StallGainsOf on an ability shape it does not model, and skips a
// mana ability, which no priority walk offers).
func (s *State) pileActivations(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	if !k.gen.anyMutate && !k.gen.anyMerge {
		return
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		r := s.pileAt(k, sl)
		if r == nil || s.genLost(k, sl) {
			continue
		}
		pp := &s.perms[sl]
		st := statSrc{sl: sl, ctrl: pp.ctrl, obj: pp.obj}
		first := int(d.N)
		for j := 0; j < int(r.n); j++ {
			fo := underObj(r, j)
			if !s.gainedOffer(k, d, st, first, fo, &k.t.Cards[s.objs[fo].card], sorc, av) {
				return
			}
		}
	}
}
