package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Ward—Sacrifice a <filter> (Vein Ripper, Ygra Eater of All;
// CR 702.21a). Gorge's ward resolution (effects effWard, rules kw_ward.go
// beginWardPayment's Sac arm) asks the warded spell's controller pay or
// don't pay (both offered always); on pay, fewer matching permanents than
// the cost counters at once, else a KChoose "ward_sac" of exactly one
// matching permanent the payer controls (posed even when one matches),
// which settleWardPayment sacrifices through pay.PaySacrificeCost: one
// sacrifice batch (BatchSacrifice) with an events.Sacrifice per pick, so
// Sacrificed / SacrificedOnce triggers fire and the sacrificed-this-turn
// tally counts it (the kernel's fireSac inside a cost-site
// sacrifice batch, as the unless-cost pick does). The filter is evaluated against the
// payer's live battlefield, layers and all (Ygra makes other creatures
// Food). Cold: reached only from a ward trigger whose amount carries
// wardSacBit, borrowing the discard form's pay election (PayDiscard +
// pfWard) and its ask dispatch.

// wardSacBit marks a ward trigger amount (stackEntry.trigAmt) as a
// sacrifice of one permanent; the filter is the source card's WardSac.
// 0x0400: wardMLBit is 0x0800 and packs N and L into bits 0-9.
const wardSacBit int16 = 0x0400

const raWardSac uint8 = 242

// ChWardSac is gorge's "ward_sac" pick: source = the permanent.
const ChWardSac ChoiceKind = 242

// DebugWardSacAnyPerm offers every permanent the payer controls in the
// pick, ignoring the filter (a mutation test).
var DebugWardSacAnyPerm bool

// wardSacFilter is the warding card's sacrifice filter.
func (s *State) wardSacFilter(k *prep) kop.FilterID {
	return k.t.Cards[s.res.e.card].WardSac
}

// wardSacCands counts the payer's permanents the pick offers.
func (s *State) wardSacCands(k *prep, each func(Slot)) int {
	p := SeatIx(s.g.pickP)
	f, src, n := s.wardSacFilter(k), s.resSrcSlot(k), 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if DebugWardSacAnyPerm || s.sacMatchGen(k, f, sl, p, src) {
			if each != nil {
				each(sl)
			}
			n++
		}
	}
	return n
}

// resolveWardSac poses the pay / don't-pay election.
//
//go:noinline
func (s *State) resolveWardSac(k *prep) {
	e := &s.res.e
	if s.wardedIndex(e) < 0 {
		s.finishResolve(k, false)
		return
	}
	s.g.pickF = pfWard
	s.g.pickZ = kop.PayDiscard // startWardDiscard hands a wardSacBit amount to startWardSac
	s.g.pickLeft = 1
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}

// startWardSac answers "pay": no matching permanent counters (gorge returns
// unpaid without an ask), else the pick.
//
//go:noinline
func (s *State) startWardSac(k *prep) {
	if s.wardSacCands(k, nil) < 1 {
		s.payDone(k, false)
		return
	}
	s.res.ask, s.res.askP = raWardSac, s.g.pickP
}

func (s *State) askOptionsWardSac(k *prep, d *Decision) bool {
	if s.res.ask != raWardSac {
		return s.askOptionsColGrant(k, d) // static_colourgrant.go
	}
	d.Sub = ChWardSac
	s.wardSacCands(k, func(sl Slot) {
		d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
	})
	return true
}

// applyAskWardSac sacrifices the pick and pays the ward.
func (s *State) applyAskWardSac(k *prep, a Action) bool {
	if s.res.ask != raWardSac {
		return s.applyAskColGrant(k, a) // static_colourgrant.go
	}
	sl := s.objs[Obj(a.Source())].slot
	if k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.sacBatchOpen()) // gorge PaySacrificeCost: one sacrifice batch
	}
	s.fireSac(k, sl)
	s.leave(k, sl, kop.ZoneGraveyard, false)
	s.payDone(k, true)
	return true
}

// Ward—Collect evidence N (CR 702.21a, 701.59a; Axebane
// Ferox). Gorge's ward resolution (effects effWard, rules kw_ward.go
// beginWardPayment / settleWardPayment) asks the warded spell's controller
// pay or don't pay (ChWardPay, both offered always); on pay, a graveyard
// whose cards' total mana value is short of N counters at once, else a
// KChoose "ward_evidence" of 1..all of the payer's graveyard cards in zone
// order; a selection short of N pays nothing (the ward counters), else the
// cards are exiled in pick order. Cold and out of line: reached only from a
// ward trigger whose amount carries wardEvBit.

// wardEvBit marks a ward trigger amount (stackEntry.trigAmt) as collect
// evidence N (the low bits); a mana ward is N >= 0, a life ward -N.
const wardEvBit int16 = 0x4000

// raWardEvidence: the ward evidence pick (ChWardEvidence).
const raWardEvidence uint8 = 240

// ChWardEvidence is gorge's "ward_evidence" KChoose: one graveyard card per
// pick, AkDone once one is picked.
const ChWardEvidence ChoiceKind = 240

// resolveWardEvidence poses the pay / don't-pay election.
//
//go:noinline
func (s *State) resolveWardEvidence(k *prep) {
	e := &s.res.e
	if s.wardedIndex(e) < 0 {
		s.finishResolve(k, false)
		return
	}
	s.g.pickF = pfWard
	s.g.pickZ = kop.PayEvidence
	s.g.pickLeft = uint8(e.trigAmt &^ wardEvBit)
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}

// graveMV is the mana value of the payer's graveyard card o (gorge
// wardManaValue: state.ManaValue in the graveyard, so a split card counts
// both halves, CR 709.4b).
func (s *State) graveMV(k *prep, o Obj) int {
	c := s.objs[o].card
	mv := int(k.cards[c].total)
	if k.gen.anySplit {
		mv = s.splitMV(k, c, o, mv) // face_castface.go
	}
	return mv
}

// startWardEvidence answers "pay": the graveyard's total short of N
// counters (gorge returns unpaid without an ask), else the pick begins.
//
//go:noinline
func (s *State) startWardEvidence(k *prep) {
	pl := &s.pl[SeatIx(s.g.pickP)]
	tot := 0
	for i := 0; i < int(pl.graveN); i++ {
		tot += s.graveMV(k, pl.grave[i])
	}
	if pl.graveN == 0 || tot < int(s.g.pickLeft) {
		s.payDone(k, false)
		return
	}
	s.g.evSel = 0
	s.res.ask, s.res.askP = raWardEvidence, s.g.pickP
}

func (s *State) askOptionsEvidence(k *prep, d *Decision) bool {
	if s.res.ask != raWardEvidence {
		return s.askOptionsCostEvidence(k, d) // cost_collectevidence.go
	}
	d.Sub = ChWardEvidence
	pl := &s.pl[SeatIx(s.g.pickP)]
	for i := 0; i < int(pl.graveN); i++ {
		if s.g.evSel&(1<<i) == 0 {
			d.add(MakeAction(AkChoose, uint8(pl.grave[i]), 0, 0))
		}
	}
	if s.g.evSel != 0 {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}

// applyAskEvidence takes one pick (or Done).
func (s *State) applyAskEvidence(k *prep, a Action) bool {
	if s.res.ask != raWardEvidence {
		return s.applyAskCostEvidence(k, a) // cost_collectevidence.go
	}
	if a.Kind() != AkDone && !s.evidencePick(s.g.pickP, a) {
		return true // more picks or Done
	}
	s.settleWardEvidence(k)
	return true
}

// settleWardEvidence pays the selection, or counters the warded entry when
// its total mana value is short of N (gorge settleWardPayment).
func (s *State) settleWardEvidence(k *prep) {
	if s.evSelTotal(k, s.g.pickP) < int(s.g.pickLeft) || DebugNoEvidence {
		s.g.evSel = 0
		s.payDone(k, false)
		return
	}
	if s.exileEvidence() > 0 && k.gen.anyExplEvTrig {
		s.fireEvidence(k, s.g.pickP) // the payment is a collect-evidence action (gorge pay.PayEvidence)
	}
	s.payDone(k, true)
}

// DebugNoEvidence (TestEvidenceLockstepMutation): a paid selection
// counters anyway.
var DebugNoEvidence bool

// Ward—Discard a card (Graveyard Trespasser; CR 702.21a). Gorge's
// ward resolution (effects effWard, rules kw_ward.go beginWardPayment's
// Discard arm) asks the warded spell's controller pay or don't pay (both
// offered always); on pay, a hand short of the card counters at once, else
// a KChoose "ward_discard" of exactly one hand card (posed even when the
// hand holds one card), which is discarded. Cold: reached only from a ward
// trigger whose amount carries wardDiscBit.

// wardDiscBit marks a ward trigger amount (stackEntry.trigAmt) as discard
// N (the low bits; N = 1).
const wardDiscBit int16 = 0x2000

const raWardDiscard uint8 = 245

// ChWardDiscard is gorge's "ward_discard" pick: source = the hand card.
const ChWardDiscard ChoiceKind = 245

// resolveWardDiscard poses the pay / don't-pay election.
//
//go:noinline
func (s *State) resolveWardDiscard(k *prep) {
	e := &s.res.e
	if s.wardedIndex(e) < 0 {
		s.finishResolve(k, false)
		return
	}
	s.g.pickF = pfWard
	s.g.pickZ = kop.PayDiscard
	s.g.pickLeft = uint8(e.trigAmt &^ wardDiscBit)
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}

// startWardDiscard answers "pay": a hand short of the cards counters
// (gorge returns unpaid without an ask), else the pick.
//
//go:noinline
func (s *State) startWardDiscard(k *prep) {
	if s.res.e.trigAmt&wardSacBit != 0 {
		s.startWardSac(k) // kw_wardcost.go
		return
	}
	if s.res.e.trigAmt&wardAltBits == wardAltBits {
		s.startWardAlt(k)
		return
	}
	if int(s.pl[SeatIx(s.g.pickP)].handN) < int(s.g.pickLeft) {
		s.payDone(k, false)
		return
	}
	s.res.ask, s.res.askP = raWardDiscard, s.g.pickP
}

func (s *State) askOptionsWardDisc(k *prep, d *Decision) bool {
	if s.res.ask == raWardAlt {
		s.askOptionsWardAlt(d)
		return true
	}
	if s.res.ask != raWardDiscard {
		return s.askOptionsWardSac(k, d) // kw_wardcost.go
	}
	d.Sub = ChWardDiscard
	pl := &s.pl[SeatIx(s.g.pickP)]
	for i := 0; i < int(pl.handN); i++ {
		d.add(MakeAction(AkChoose, uint8(pl.hand[i]), 0, 0))
	}
	return true
}

// applyAskWardDisc discards the pick and pays the ward.
func (s *State) applyAskWardDisc(k *prep, a Action) bool {
	if s.res.ask == raWardAlt {
		s.applyAskWardAlt(k, a)
		return true
	}
	if s.res.ask != raWardDiscard {
		return s.applyAskWardSac(k, a) // kw_wardcost.go
	}
	s.discardCost(k, Obj(a.Source()))
	s.payDone(k, true)
	return true
}

// Ward—Discard a card or pay {N} (Titania, Rugged Rumbler; CR 702.21a).
// Gorge's ward resolution (effects effWard, rules ward.go beginWardPayment's
// ">:" arm) asks the warded spell's controller pay or don't pay (both offered
// always); on pay, one KChoose "ward_alt": a "ward_discard" option per hand
// card in hand order, then "ward_mana" while the pool pays {N} or the payer
// controls an untapped mana source (hasUntappedManaSource), posed even with
// one option; with none the ward counters unasked. A discard pays; {N} is
// paid as the plain {N} ward's is, from the pool and then through the ward
// mana window, short of it countering (payWardN). Cold: reached only from a
// ward trigger whose amount carries wardAltBits.

// wardAltBits mark a ward trigger amount (stackEntry.trigAmt) as discard a
// card or pay {N} (N the low bits). No other form sets both bits.
const wardAltBits = wardDiscBit | wardEvBit

const raWardAlt uint8 = 239

// ChWardAlt is gorge's "ward_alt" choice: AkChoose of a hand card (mode 0,
// source = the card) or of {N} (mode 1).
const ChWardAlt ChoiceKind = 239

// resolveWardAlt poses the pay / don't-pay election.
//
//go:noinline
func (s *State) resolveWardAlt(k *prep) {
	e := &s.res.e
	if s.wardedIndex(e) < 0 {
		s.finishResolve(k, false)
		return
	}
	s.g.pickF = pfWard
	s.g.pickZ = kop.PayDiscard // startWardDiscard hands a wardAltBits amount to startWardAlt
	s.g.pickLeft = 1
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}

// wardAltMana: the choice offers {N} (gorge: the pool pays it, or the payer
// controls an untapped mana source).
func (s *State) wardAltMana(k *prep, p uint8) bool {
	c := kop.ManaCost{Generic: uint8(s.res.e.trigAmt &^ wardAltBits)}
	return s.poolPays(p, &c) || s.hasUntappedMana(k, p)
}

// startWardAlt answers "pay": no card in hand and no {N} counters unasked,
// else the choice, with the {N} offer kept in pickLeft.
//
//go:noinline
func (s *State) startWardAlt(k *prep) {
	p := SeatIx(s.g.pickP)
	s.g.pickLeft = 0
	if s.wardAltMana(k, p) {
		s.g.pickLeft = 1
	}
	if s.pl[p].handN == 0 && s.g.pickLeft == 0 {
		s.payDone(k, false)
		return
	}
	s.res.ask, s.res.askP = raWardAlt, p
}

func (s *State) askOptionsWardAlt(d *Decision) {
	d.Sub = ChWardAlt
	pl := &s.pl[SeatIx(s.g.pickP)]
	for i := 0; i < int(pl.handN); i++ {
		d.add(MakeAction(AkChoose, uint8(pl.hand[i]), 0, 0))
	}
	if s.g.pickLeft != 0 {
		d.add(MakeAction(AkChoose, 0, 0, 1))
	}
}

// applyAskWardAlt discards the card picked, or pays {N}.
func (s *State) applyAskWardAlt(k *prep, a Action) {
	if a.Mode() == 1 {
		s.res.ask = raNone
		s.payWardN(k, uint8(s.res.e.trigAmt&^wardAltBits), true)
		return
	}
	s.discardCost(k, Obj(a.Source()))
	s.payDone(k, true)
}

// Ward—Pay life equal to its power (Raubahn, Bull
// of Ala Mhigo; CR 702.21a, 119.4). Gorge's ward resolution (rules kw_ward.go
// beginWardPayment) prices any PayLife<X/…> cost as the ward permanent's
// power when the ward is paid, floored at 0: a pump in response changes the
// life paid. The ask is then the pay-N-life one (resolveWardLife). Cold:
// reached only from a ward trigger whose amount carries wardPowBit.

// wardPowBit marks a ward trigger amount (stackEntry.trigAmt) as pay life
// equal to the ward permanent's power (no low bits: the power is read at
// resolution).
const wardPowBit int16 = 0x1000

// resolveWardPower reads the ward permanent's power now and poses the
// pay-N-life ask with N = that power.
//
//go:noinline
func (s *State) resolveWardPower(k *prep) {
	e := &s.res.e
	idx := s.wardedIndex(e)
	if idx < 0 {
		s.finishResolve(k, false)
		return
	}
	// the ward permanent is the entry's source object (checkWard enqueues
	// it); off the battlefield objPT reads its card's printed power
	n, _ := s.objPT(k, e.obj, s.slotIfSame(e.obj, e.srcVer))
	if n < 0 {
		n = 0
	}
	if n > 255 {
		s.capFull(StallPT) // pickLeft is a byte: the power leaves the life ask's range
		n = 255
	}
	if int(s.pl[e.trigPl].life) < n {
		s.wardCounterAt(k, idx)
		s.finishResolve(k, false)
		return
	}
	s.g.pickF = pfWard
	s.g.pickZ = kop.PayLife
	s.g.pickLeft = uint8(n)
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}
