package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Resolution decisions of the generated-table ops: posed through the core's
// ctlResolve (resolveOptions / applyResolve hooks), one pick per item,
// AkDone-closed where the answer may stop early.

const (
	pfIfPaid   = 0x8000 // OpIfPaid / OpUnlessSwitched (else OpUnlessPaid)
	pfWard     = 0x4000 // a ward payment (counter the warded entry unless paid)
	pfNoPay    = 0x2000 // the cost can't be paid: the ask offers decline only
	pfModes    = 0x1000 // a cost inside the effect (not a trigger's own cost): ChGenPayMode
	pfEachNext = 0x0800 // an any-player ask: player 1 is still to be asked after a decline
)

// payPickBlight is pickZ while a Blight cost (CR 701.68) chooses its
// creature: a PaySac whose B carries PayBSubBlight.
const payPickBlight = 250

// --- unless / if-you-do costs ---

func (s *State) canPayGen(k *prep, p uint8, kind int, amt int, pips int16, f kop.FilterID) bool {
	switch kind {
	case kop.PayMana:
		if f != 0 && int(s.pl[p].life) < int(f) {
			return false // a trigger's own mana-and-life cost: F is the life
		}
		c := manaOf(amt, pips)
		var av avail
		av.xd, av.xdOn = payDesc{class: pdOther}, true // an unless, ward or trigger cost is no spell's, an ability's or a cast's
		s.availWindowFor(k, p, &av)
		av.xdOn = false
		return av.affords(&c, pipMaskOf(&c), 0)
	case kop.PayDiscard, kop.PayDiscardRandom:
		return int(s.pl[p].handN) >= amt
	case kop.PayDiscardHand:
		return true // gorge unlessflow: the whole hand; empty pays with nothing
	case payKindBehold:
		return s.unlessBeholdCands(k, p, f, nil) >= amt // cost_unlessbehold.go
	case kop.PaySac:
		if pips == kop.PayBSubBlight {
			return s.countPermMatch(k, p, f, s.resSrcSlot(k), false) >= 1 // a Blight: one creature, source included
		}
		return s.countSac(k, p, f, s.resSrcSlot(k)) >= amt // "another": not the source
	case kop.PayLife:
		return int(s.pl[p].life) >= amt
	case kop.PayEnergy:
		return int(s.g.energy[SeatIx(p)]) >= amt
	case kop.PayReturn:
		return s.countReturn(k, p, f) >= amt
	case kop.PayForage:
		return s.foragePayableRes(k, p, f)
	case kop.PaySacSelf:
		sl := s.resSrcSlot(k)
		return sl != NoSlot && s.perms[sl].ctrl == p && !s.sacBlocked(k, sl)
	case payKindSacHost:
		return s.sacHostSlot(p) != NoSlot // the granting host, still the payer's (cost_sachost.go)
	case kop.PayDraw:
		return amt > 0 // gorge triggeredCostDrawCounts: payable once the count resolves; an unresolved or zero X stays unpriceable (decline only)
	case payKindDamage:
		return amt > 0 // gorge: DamageYou<X> folds only for a positive X (cost_restokens.go)
	case kop.PaySubCounter:
		sl := s.resSrcSlot(k)
		return sl != NoSlot && permCtrN(s, &s.perms[sl], int(pips)) >= amt // pips carries the counter kind (CtrAll sums every kind)
	case kop.PayTapX:
		return s.countUntapped(k, p, f, s.resSrcSlot(k)) >= amt
	case payKindWaterbend:
		return s.unlessWaterbendPayable(k, p, amt) // cost_unlesswaterbend.go
	case payKindAddCounter:
		return s.addCounterTarget(k, pips) != NoSlot // cost_unlessaddcounter.go
	case kop.PayExileGrave:
		return s.countGraveExile(k, p, f) >= amt
	case payKindExileHand:
		return int(s.pl[SeatIx(p)].handN) >= amt // cost_exilefromhand.go
	case kop.PayManaX:
		return true // X = 0 is always payable
	case payKindEvidence:
		return s.graveTotalMV(k, p) >= amt // gorge evidenceStage: the graveyard reaches N
	case payKindTrigNewEvidence:
		return s.trigNewEvidencePayable(k, p, amt) // cost_collectevidencex.go
	case payKindTrigNew:
		return s.trigNewInGrave() != NoObj // the triggered card is still in a graveyard (trig_mayexile.go)
	}
	return false
}

func manaOf(generic int, pips int16) kop.ManaCost {
	c := kop.ManaCost{Generic: uint8(generic)}
	for i := 0; i < 5; i++ {
		c.Pips[i] = uint8(pips>>(3*i)) & 7
	}
	return c
}

// execPay starts OpUnlessPaid / OpIfPaid / OpUnlessSwitched; false = suspended.
func (s *State) execPay(k *prep, in *kop.Instr) bool {
	p := s.payerOf(k, in.Obj)
	kind := int(in.C & 0xF)
	if kind == kop.PayManaX && in.B.N == kop.PayBSubEvidence {
		kind = payKindEvidence // a trigger's own CollectEvidence cost rides PayManaX (cost_collectevidence.go)
	}
	if kind == kop.PaySacSelf && in.B.N == kop.PayBSubSacHost {
		kind = payKindSacHost // a granted trigger's own Sac<1/OriginalHost> cost rides PaySacSelf (cost_sachost.go)
	}
	if kind == kop.PayExileGrave && in.B.N == kop.PayBSubTrigNew {
		kind = payKindTrigNew // a trigger's own ExileAnyGrave<1/Card.TriggeredNewCard> cost rides PayExileGrave (trig_mayexile.go)
	}
	kind = resTokenPayKind(kind, in.B.N) // DamageYou<X> rides PayDraw (cost_restokens.go)
	kind = unlessPayKind(kind, in)       // an UnlessCost$ Waterbend<N> or AddCounter rides PayTapX (cost_unlesswaterbend.go)
	if kind == kop.PayExileGrave && in.B.N == kop.PayBSubExileHand {
		kind = payKindExileHand // a trigger's own ExileFromHand<N/Card> cost rides PayExileGrave (cost_exilefromhand.go)
	}
	if kind == kop.PayExileGrave && in.B.N == kop.PayBSubTrigNewEvidence {
		kind = payKindTrigNewEvidence // a dies trigger's own triggered-card exile beside evidence (cost_collectevidencex.go)
	}
	skip := uint16(in.C >> 4)
	amt := s.value(k, in.A)
	pips := in.B.N
	if k.gen != nil && k.gen.anyNumTimes && kind == kop.PayMana {
		amt, pips = s.numTimesScale(k, amt, pips) // cast_announcetimes.go
	}
	if in.Op == kop.OpIfPaid || in.Op == kop.OpUnlessSwitched {
		skip |= pfIfPaid
	}
	if kind == kop.PayTapX && in.Op == kop.OpIfPaid && s.canPayGen(k, p, kind, amt, pips, in.F) {
		// gorge's literal tap election IS the payment (cumulative.go:1386):
		// there is no pay/decline ask when it can be paid. An unpayable one
		// keeps the decline-only ask below.
		s.g.pickF = skip
		s.g.pickZ = uint8(kind)
		s.g.pickLeft = uint8(amt)
		s.g.pickP = p
		s.res.ask, s.res.askP = raGenPayPick, p
		return false
	}
	if p == 255 {
		if k.eachPay && in.Obj == kop.RefEachPlayer {
			return s.execPayEach(k, in, kind, skip, amt) // cost_anyplayerpays.go
		}
		if skip&pfIfPaid != 0 {
			s.res.pc += uint8(skip)
		}
		return true
	}
	if e := &s.res.e; e.kind != seTrigger || s.res.pc != 0 && !s.afterNumTimes(k) || in.Op != kop.OpIfPaid {
		skip |= pfModes // only a trigger's own cost is gorge's trigger_cost pay/decline; an unless is its modes (Painful Quandary)
	}
	if kind == kop.PayMana {
		if ch := s.exileChain(k); ch != nil {
			return s.execPayChain(k, p, ch, skip, amt, pips) // cost_exilepool.go
		}
	}
	if kind == kop.PayManaX && in.B.N == kop.PayBSubEvidenceX {
		return s.execTrigEvidenceX(k, p, skip, amt) // cost_collectevidencex.go
	}
	if kind == kop.PayManaX {
		return s.execTrigX(k, p, skip, amt) // trig_maypayx.go
	}
	stackSelfGone := kind == kop.PayExileGrave && in.B.N == kop.PayBSubStackSelf && s.objs[s.res.e.obj].zone != kop.ZoneStack
	if stackSelfGone || !s.canPayGen(k, p, kind, amt, pips, in.F) && !s.windowPays(k, p, kind, skip, amt, pips) &&
		!s.trigCostPotentialPays(k, p, kind, skip, amt, pips, in.F) {
		skip |= pfNoPay // still asked, decline only (gorge poses it)
	}
	s.g.pickF = skip
	s.g.pickZ = uint8(kind)
	if kind == kop.PaySac && in.B.N == kop.PayBSubBlight {
		s.g.pickZ = payPickBlight
		s.g.pickLeft = 1 // one creature, however many counters
	} else {
		s.g.pickLeft = uint8(amt)
	}
	s.g.pickP = p
	s.res.ask, s.res.askP = raGenPay, p
	return false
}

// trigCostPotentialPays mirrors gorge's trigger-cost offer gate. gorge asks
// whether the payer's potential mana can reach the cost (rules respay.go
// resPayNeedsMana, potential.go potentialManaFor), an over-bound: a dual counts
// once in each colour and an open production is unlimited. So a trigger's own mana
// cost can be offered when the sources cannot fund it: the payer elects pay, fails
// the charge (CR 118.3) and declines. Only that ask gets the over-bound: an unless
// cost, a ward, a cost inside an effect or a trigger body's own cost keeps the
// exact gate (skip&pfModes).
//
//go:noinline
func (s *State) trigCostPotentialPays(k *prep, p uint8, kind int, skip uint16, amt int, pips int16, life kop.FilterID) bool {
	if kind != kop.PayMana || skip&pfModes != 0 || s.res.e.kind != seTrigger {
		return false
	}
	if life != 0 && int(s.pl[SeatIx(p)].life) < int(life) {
		return false
	}
	other := payDesc{class: pdOther}
	if !s.anyManaSource(k, p, &other) {
		return false
	}
	c := manaOf(amt, pips)
	var av avail
	s.availPotentialFor(k, p, &av, &other)
	return av.affords(&c, pipMaskOf(&c), 0)
}

// anyManaSource reports whether p has an untapped source to fund payment d with:
// a tap source, the untapped sacrifice-only arm, a gap source with its cost payable,
// a deferred source, or a restricted mana ability d may spend (gorge
// untappedManaSourceFor: a source whose every ability's RestrictValid$ refuses d
// adds nothing to it, CR 106.6).
//
//go:noinline
func (s *State) anyManaSource(k *prep, p uint8, d *payDesc) bool {
	if k.gen == nil {
		return false
	}
	// a deferred source (Pyromancer's Goggles, mana_deferred.go) has no census
	// ability, yet its plain {R} is untapped mana gorge's untappedManaSourceFor
	// counts (CR 106.6: the when-spent rider restricts nothing)
	if s.scriptDeferred(k, p).any() {
		return true
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCGen(k, sl)
		if s.canTapForManaPC(k, sl, pc) || k.sacOnlyMana && s.canSacOnlyForMana(k, sl, pc) {
			return true
		}
		// a tap-another-permanent or counter-cost source: gorge's walk counts it too
		if c := s.cardAt(sl); k.cards[c].manaAb >= 0 {
			if j := int(k.cards[c].manaAb); s.potentialAdmits(k, p, sl, j, &k.t.Cards[c].Abilities[j], nil) {
				return true
			}
		}
		// an each-colour source (Bloom Tender) is a census gap with no manaAb,
		// yet gorge's walk counts it, and reads its production as open
		if c := s.cardAt(sl); k.gen.eachCol(c) >= 0 && s.potentialAdmits(k, p, sl, int(k.gen.eachCol(c)), &k.t.Cards[c].Abilities[k.gen.eachCol(c)], nil) {
			return true
		}
		// gorge untappedManaSourceFor: an available mana ability counts when its
		// RestrictValid$ admits this payment, so a source whose only ability is
		// restricted to something else (Queen's Diamond's commander-only mana)
		// is no source for a trigger's cost, while Hydraulic Helper's still is.
		cd := &k.t.Cards[s.cardAt(sl)]
		for j := range cd.Abilities {
			if ab := &cd.Abilities[j]; j < 16 && ab.Kind == kop.AbMana && ab.AFlags&kop.AfLatent == 0 && ab.ManaRestrict != 0 && s.restrSrcAdmits(k, p, sl, ab, d) && s.potentialAdmits(k, p, sl, j, ab, nil) {
				return true
			}
		}
	}
	return false
}

// openManaRefused reports whether sl's mana production is a granted ability
// or a printed first ability whose RestrictValid$ refuses the trigger's own
// payment (pay.PurposeOther). It reads the same view manaPCGen returned for
// the source: the granted ability when it is the source's only usable one,
// else the card's first mana ability.
//
//go:noinline
func (s *State) openManaRefused(k *prep, p uint8, sl Slot, d *payDesc) bool {
	if k.gen != nil && len(k.gen.grantPC) > 0 {
		if gi := s.grantIdx(k, sl); gi >= 0 && !s.ownManaUsable(k, sl) {
			if k.gen.grantRestr[gi] == 0 {
				return false
			}
			r := k.gen.grantOf[gi]
			return !s.restrSrcAdmits(k, p, sl, &k.t.Cards[r.card].Abilities[r.ab], d)
		}
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	j := firstManaAb(cd)
	return j >= 0 && cd.Abilities[j].ManaRestrict != 0 &&
		!s.restrSrcAdmits(k, p, sl, &cd.Abilities[j], d)
}

func (s *State) payerOf(k *prep, r kop.Ref) uint8 {
	if r == kop.RefEach {
		return s.g.eachPl
	}
	return s.playerRef(k, r)
}

// --- the resolution decision options ---

func (s *State) resolveOptionsGen(k *prep, d *Decision) bool {
	r := &s.res
	d.Player = r.askP
	d.Kind = DkChoose
	switch r.ask {
	case raGenBranch:
		d.Sub = ChGenBranch
		for i := 0; i < int(r.left); i++ {
			if s.branchPayable(k, uint8(i), r.askP) {
				d.add(MakeAction(AkChoose, 0, 0, uint8(i)))
			}
		}
	case raChooseType:
		d.Sub = ChGenType
		if c := s.prog(k)[r.pc].C; c != 0 {
			chooseCardTypeOptions(d, kop.TypeMask(c))
			break
		}
		for v := range k.t.TypeVocab {
			d.add(MakeAction(AkChoose, 0, uint8(v>>8), uint8(v)))
		}

	case raGenPay:
		d.Sub = ChGenPay
		if s.g.pickF&pfWard != 0 {
			d.Sub = ChWardPay
		} else if s.g.pickF&pfModes != 0 {
			d.Sub = ChGenPayMode
		}
		d.add(MakeAction(AkChoose, 0, 0, 0))
		if s.g.pickF&pfNoPay == 0 {
			d.add(MakeAction(AkChoose, 0, 0, 1))
		}
	case raGenPayPick, raGenSac:
		d.Sub = ChGenPayPick
		if r.ask == raGenSac {
			d.Sub = ChSacrifice
		}
		p := s.g.pickP
		if s.g.pickZ == payPickBlight {
			f := s.genPickFilter(k)
			src := s.resSrcSlot(k)
			for m := s.ctrlM[p]; m.any(); m = m.rest() {
				sl := m.first()
				if s.matchPermGen(k, f, sl, p, src, layerFull) {
					d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
				}
			}
			break
		}
		if s.g.pickZ == kop.PaySubCounter {
			d.Sub = ChCostSubCtr
			sl := s.resSrcSlot(k)
			if sl != NoSlot {
				pp := &s.perms[sl]
				for c := 0; c < int(kop.NumCounterKinds3); c++ {
					if kop.IsCounterKind(c) && pp.ctrN(s, c) > 0 {
						d.add(MakeAction(AkChoose, uint8(pp.obj), 0, uint8(c)))
					}
				}
			}
			break
		}
		if s.g.pickZ == payKindWaterbend {
			s.unlessWaterbendOptions(k, d) // cost_unlesswaterbend.go
			break
		}
		if s.g.pickZ == kop.PayExileGrave {
			s.graveExileOptions(k, d, p)
			break
		}
		if s.g.pickZ == payKindExileHand {
			s.handExileOptions(d, p) // cost_exilefromhand.go
			break
		}
		if s.g.pickZ == payKindBehold {
			s.unlessBeholdOptions(k, d, p) // cost_unlessbehold.go
			break
		}
		if s.g.pickZ == kop.PayDiscard {
			d.Sub = ChCostDiscard
			pl := &s.pl[p]
			for i := 0; i < int(pl.handN); i++ {
				d.add(MakeAction(AkChoose, uint8(pl.hand[i]), 0, 0))
			}
			break
		}
		f := s.genPickFilter(k)
		src := NoSlot
		if r.ask == raGenPayPick {
			src = s.resSrcSlot(k)
			if s.g.pickZ == kop.PayReturn {
				d.Sub = ChGenPayReturn
			} else if s.g.pickZ == kop.PayTapX {
				d.Sub = ChGenPayTap
			}
		}
		sac := r.ask == raGenSac || s.g.pickZ == kop.PaySac
		for m := s.ctrlM[p].andNot(s.g.sacM); m.any(); m = m.rest() {
			sl := m.first()
			if s.matchPermGen(k, f, sl, p, src, layerFull) && !(sac && s.sacBlocked(k, sl)) {
				d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
			}
		}
	case raGenDigArrange:
		d.Sub = ChArrange
		for i := 0; i < int(s.g.nlook); i++ {
			if s.g.lookSel&(1<<i) == 0 {
				d.add(MakeAction(AkChoose, uint8(s.resArm().look[i]), 0, 0))
			}
		}
	case raGenResTarget:
		in := &s.prog(k)[r.pc]
		t := tspecE(k, &r.e, int(in.C))
		var buf [MaxOptions]uint8
		n := s.targetCands(k, t, r.e.ctrl, s.resSrcSlot(k), r.e.obj, &buf)
		// gorge's mid-resolution target ask: a KChoose over the cards
		// ("card"), closed by done for an up-to slot
		d.Sub = ChGenResTarget
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, buf[i], 0, 0))
		}
		if t.UpTo {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	case raGenDivide:
		d.Sub = ChGenDivide
		var codes [maxXTgt + 1]uint8
		in := &s.prog(k)[r.pc]
		m := s.divTargets(k, &r.e, uint8(tgtSlotOf(in.Obj)), &codes)
		for i := 0; i < m; i++ {
			d.add(MakeAction(AkChoose, codes[i], 0, 0))
		}
	case raGenForage:
		s.forageResOptions(k, d)
	case raGenLookAck:
		d.Sub = ChGenLookAck
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenTopBottom:
		d.Sub = ChGenTopBottom
		d.add(MakeAction(AkChoose, 0, 0, 0))
		d.add(MakeAction(AkChoose, 0, 0, 1))
	case raGenSearch:
		// gorge's kinds: a library search, a hand move, else a hidden pick
		switch s.searchZone(k) {
		case kop.ZoneLibrary, kop.ZoneSideboard:
			d.Sub = ChGenSearch // a sideboard search is effSearchLibrary too (zone_sideboardsearch.go)
		case kop.ZoneHand:
			d.Sub = ChGenHandMove
			if s.g.pickF&kop.SrReveal != 0 {
				d.Sub = ChGenReveal
			}
		default:
			d.Sub = ChGenHiddenPick
			if s.handGraveOp(k) {
				d.Sub = ChGenSearch // gorge effSearchLibrary over Hand,Graveyard (zone_handgravepick.go)
			}
		}
		if s.altOriginOp(k) {
			d.Sub = ChGenSearch // an OriginAlternative$ search is effSearchLibrary whichever zones were elected
		}
		s.searchCands(k, d)
		if s.g.pickF&kop.SrMandatory == 0 && s.exactlyMayStop(&s.prog(k)[r.pc]) || d.N == 0 {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	case raGenDig:
		d.Sub = ChGenDig
		f := s.prog(k)[r.pc].F
		for i := 0; i < int(s.g.nlook); i++ {
			o := s.resArm().look[i]
			if s.g.lookSel&(1<<i) == 0 && s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), r.e.ctrl, o) && s.pickFits(k, f, o, true) {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
		if d.N == 0 || s.digMayStop(k) {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	case raGenDigAllOpt:
		// the ChangeNum$ All election: yes first, then no (gorge effDig's
		// allOrNothing options, "yes"/"no").
		d.Sub = ChGenDigOpt
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenPileSplit:
		d.Sub = ChGenPileSplit
		for i := 0; i < int(s.g.nrem); i++ {
			if s.g.lookSel&(1<<i) == 0 {
				d.add(MakeAction(AkChoose, uint8(s.g.rem[i]), 0, 0))
			}
		}
		d.add(MakeAction(AkDone, 0, 0, 0))
	case raGenPileChoose:
		d.Sub = ChGenPileChoose
		d.add(MakeAction(AkChoose, 0, 0, 0))
		d.add(MakeAction(AkChoose, 0, 0, 1))
	case raGenChooseEach:
		d.Sub = ChGenChooseEach
		// in object order, as gorge's cardChoices walks g.Objs (pair 2433)
		t := chooseEachTypes[s.g.pickZ]
		var buf [MaxPerm]Obj
		n := 0
		for m := s.ctrlM[s.g.pickP]; m.any(); m = m.rest() {
			sl := m.first()
			if s.typesGen(k, sl)&t != 0 {
				o := s.perms[sl].obj
				j := n
				for ; j > 0 && buf[j-1] > o; j-- {
					buf[j] = buf[j-1]
				}
				buf[j] = o
				n++
			}
		}
		for _, o := range buf[:n] {
			d.add(MakeAction(AkChoose, uint8(o), 0, s.g.pickZ))
		}
	case raGenCastFree:
		d.Sub = ChGenCastFree
		for i := 0; i < int(s.g.nfree); i++ {
			if s.freeRepeated(i) {
				continue // an AllowRepeats$ card is offered once, at its first place left (cast_freerepeat.go)
			}
			d.add(MakeAction(AkChoose, uint8(s.g.free[i]), 0, 0))
		}
		d.add(MakeAction(AkDone, 0, 0, 0))
	case raGenChooseCard:
		d.Sub = ChGenChooseCard
		if c := s.prog(k)[r.pc].C; c == chooseAttachPick || c == chooseAttachPickOpt {
			s.attachPickOptions(k, d, r.pc, r.askP) // tgt_attachpick.go
			break
		}
		if s.prog(k)[r.pc].C == choosePutCounterPick {
			d.Sub = ChGenBolster // gorge's counter_pick
		}
		if s.prog(k)[r.pc].C == chooseFaceDownPick {
			d.Sub = ChGenFaceDownPick // gorge's face_down_choice pick (CR 701.58a)
		}
		if s.prog(k)[r.pc].C == chooseCopyPick {
			d.Sub = ChGenCopyPick // gorge's copypermanent_choice pick (copy_picked.go)
		}
		s.cardCands(k, s.prog(k)[r.pc].F, r.askP, d)
		if c := s.prog(k)[r.pc].C; c == 1 || c == chooseLibUpTo {
			d.add(MakeAction(AkDone, 0, 0, 0)) // up to one (gorge choiceBounds Min 0)
		}
	case raGenTokChoice, raGenRetChoice:
		if s.g.pre.kind == akEntryOrder {
			d.Sub = ChGenReplOrder // a returning clone's order ask (copy_returnorder.go)
			s.cloneEntryOrderOptions(k, d)
			break
		}
		s.askOptions(k, d, s.g.pre.kind)
	case raGenEnterAsk:
		s.askOptions(k, d, s.g.ask[0].kind)
	case raGenEntryOrder:
		s.entryOrderOptions(k, d)
	case raGenReplOrder, raGenCtrOrder:
		s.replOrderOptions(d)
	case raGenCopyTarget, raGenCopyTargets:
		s.copyTargetOptions(k, d, int(r.left))
	case raGenEnchant:
		d.Sub = ChGenEnchant
		var buf [MaxOptions]uint8
		n := s.enchantCands(k, s.enchantingCard(k), r.askP, &buf)
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, buf[i], 0, 0))
		}
	case raGenCmdZone:
		s.cmdZoneOptions(d) // zone_cmdzone.go
	case raRecruitDiscard:
		s.recruitOptions(k, d) // act_recruit.go
	default:
		if !s.askOptionsCold1(k, d) {
			panic("kernel: bad resolution ask")
		}
	}
	return true
}

// enchantingCard is the card of the Aura the pending enchant ask is for:
// the returning card (OpReturnEx) or the pending token copy.
func (s *State) enchantingCard(k *prep) CardID { return s.objs[s.enchantingObj(k)].card }

// enchantingObj is the Aura an enchant ask is about.
func (s *State) enchantingObj(k *prep) Obj {
	if in := &s.prog(k)[s.res.pc]; in.Op == kop.OpReturnEx || in.Op == kop.OpReturn {
		return s.objRefObj(in.Obj)
	} else if in.Op == kop.OpChooseMove || in.Op == kop.OpSearchEx {
		return s.resArm().look[0] // the move's parked Aura (handAuraEnter)
	}
	return s.res.lastTok
}

// autoPayPick pays the pending pick at once when its candidates are
// exactly the number owed (in battlefield order); false leaves it asked.
func (s *State) autoPayPick(k *prep) bool {
	var d Decision
	s.resolveOptionsGen(k, &d)
	if s.g.pickZ == payPickBlight {
		if d.N != 1 {
			return false
		}
		s.blightSlot(s.objs[Obj(d.Options[0].Source())].slot, s.value(k, s.prog(k)[s.res.pc].A))
		s.g.pickLeft = 0
		return true
	}
	if int(d.N) != int(s.g.pickLeft) {
		return false
	}
	if s.g.pickZ == payKindBehold {
		s.g.pickLeft = 0 // exactly the candidates owed: beholding moves nothing
		return true
	}
	objs := d.Options[:d.N]
	for i := 1; i < len(objs); i++ { // battlefield (entry) order
		for j := i; j > 0; j-- {
			a, b := s.objs[objs[j-1].Source()], s.objs[objs[j].Source()]
			if a.zone != kop.ZoneBattlefield || b.zone != kop.ZoneBattlefield || s.perms[a.slot].ts <= s.perms[b.slot].ts {
				break
			}
			objs[j-1], objs[j] = objs[j], objs[j-1]
		}
	}
	if k.gen.anyBatch && s.g.pickZ != kop.PayDiscard && s.g.pickZ != kop.PayReturn && s.g.pickZ != kop.PayExileGrave && s.g.pickZ != payKindExileHand {
		defer s.batchClose(s.sacBatchOpen()) // gorge emitSacrifices (unlessflow): one sacrifice batch
	}
	if len(objs) > 0 && s.g.pickZ != kop.PayExileGrave && s.g.pickZ != payKindExileHand && s.g.pickZ != kop.PayDiscard && s.g.pickZ != kop.PayReturn {
		s.snapTrigSacrificed(k, s.objs[Obj(objs[0].Source())].slot) // trig_sacrificedlki.go
	}
	for _, a := range objs {
		o := Obj(a.Source())
		if s.g.pickZ == kop.PayExileGrave || s.g.pickZ == payKindExileHand {
			s.exileFromGrave(o)
		} else if s.g.pickZ == kop.PayDiscard {
			s.discardUnlessPay(k, o)
		} else if s.g.pickZ == kop.PayReturn {
			s.leave(k, s.objs[o].slot, kop.ZoneHand, false)
		} else {
			s.fireSac(k, s.objs[o].slot)
			s.leaveParked(k, s.objs[o].slot)
		}
	}
	s.g.pickLeft = 0
	return true
}

// genPickFilter is the permanent filter of the pick in progress.
func (s *State) genPickFilter(k *prep) kop.FilterID {
	return s.prog(k)[s.res.pc].F
}

var chooseEachTypes = [...]kop.TypeMask{kop.TArtifact, kop.TBattle, kop.TCreature, kop.TEnchantment, kop.TLand, kop.TPlaneswalker}

// continueRes resumes the program after an answered ask, skipping n
// instructions past the current one.
func (s *State) continueRes(k *prep, n uint8) {
	s.res.ask = raNone
	if len(s.cold.parked) != 0 && s.parkedWaiting() {
		s.res.pc += n // the answer to the parked move continues the program
		s.ctl = ctlExileOrder
		return
	}
	if n == 0 && s.g.nask != 0 && s.midResAsk(k) {
		return // the answer continues the program (raGenEnterAsk)
	}
	if n == 0 && s.g.retAt != 0 && s.returnRemembered(k, &s.prog(k)[s.res.pc]) {
		return
	}
	if n == 0 && k.gen != nil && s.parkReturnAsk() {
		return // the answer made a departure frame park a return: ask it before the program goes on (zone_untilleavesbatch.go)
	}
	s.res.pc += n + 1
	s.loopBack(k)
	s.runProg(k)
}

func (s *State) applyResolveGen(k *prep, a Action) {
	r := &s.res
	switch r.ask {
	case raGenBranch:
		s.g.branch = a.Mode()
		s.continueRes(k, 0)
	case raGenPay:
		if a.Mode() == 0 {
			s.payDone(k, false)
			return
		}
		switch s.g.pickZ {
		case kop.PayMana:
			if ch := s.exileChain(k); ch != nil {
				s.startChainExile(k, ch) // a mana-and-exile cost (cost_exilepool.go)
				return
			}
			c := manaOf(int(s.g.pickLeft), s.numTimesPips(k, s.prog(k)[r.pc].B.N))
			if s.g.pickF&(pfWard|pfModes) != 0 {
				paused, short := s.payWindowAsk(k, s.g.pickP, &c, winPay) // gorge's ward / unless mana window
				if paused {
					return // a tap-another source's pick (mana_windowtapx.go)
				}
				if short {
					s.payDone(k, false) // the colours taken left it unpaid
					return
				}
			} else if paid := s.payReplanned(k, s.g.pickP, &c); s.parkedWaiting() {
				s.ctl = ctlExileOrder // a sacrificed source's order ask (payManaResume)
				return
			} else if !paid {
				// a trigger cost gorge offered on its potential bound but the
				// sources cannot fund: the manual window's taps and floated
				// mana stay, the charge fails and the flow declines
				if s.res.ask == raGenWinTap {
					return // a tap-another source's pick (mana_windowtapx.go)
				}
				s.payDone(k, false)
				return
			}
		case kop.PayEvidence:
			s.startWardEvidence(k) // kw_wardcost.go
			return
		case payKindWaterbend:
			s.startUnlessWaterbend(k) // cost_unlesswaterbend.go
			return
		case payKindEvidence:
			s.startCostEvidence(k) // a trigger's own cost (cost_collectevidence.go)
			return
		case payKindTrigNewEvidence:
			s.startTrigNewEvidence(k) // cost_collectevidencex.go
			return
		case payKindTrigNew:
			s.exileTrigNew() // the triggered card, unasked (trig_mayexile.go)
		case kop.PayLife:
			if int(s.pl[s.g.pickP].life) < int(s.g.pickLeft) {
				s.payDone(k, false) // CR 119.4: a ward's unpayable life (gorge counters)
				return
			}
			s.loseLife(k, s.g.pickP, int(s.g.pickLeft))
		case kop.PayEnergy:
			s.payEnergyN(s.g.pickP, s.g.pickLeft)
		case kop.PayForage:
			s.forageRes(k)
			return
		case kop.PaySacSelf:
			s.paySacSelf(k)
		case payKindSacHost:
			s.paySacHost(k)
		case kop.PayDraw:
			s.payDraw(s.g.pickP, int(s.g.pickLeft))
		case payKindDamage, payKindPoison:
			s.payResToken(k) // cost_restokens.go
		case payKindAddCounter:
			s.payAddCounter(k, &s.prog(k)[r.pc]) // cost_unlessaddcounter.go
			return
		case kop.PaySubCounter:
			// B: drain the resolving source's own counters (gorge
			// unless_cost.go:47 / trigger_cost_subcounter.go:91).
			sl := s.resSrcSlot(k)
			kind := int(s.prog(k)[r.pc].B.N)
			if sl == NoSlot {
				s.payDone(k, false)
				return
			}
			if kind == int(kop.CtrAll) {
				s.paySubCounterAny(k, sl)
				return
			}
			if s.perms[sl].ctrN(s, kind) < int(s.g.pickLeft) {
				s.payDone(k, false)
				return
			}
			pp := &s.perms[sl]
			s.touchSl(sl)
			pp.seeCtr(s)
			if debugEntersTappedMutate != 3 { // TestSourceCounterCostsMutation: the drain
				pp.subCtrN(s, kind, int(s.g.pickLeft))
			}
		case kop.PayTapX:
			// C: the tap election (one pick per answer); an
			// UnlessCost$ one is recorded unasked with exactly N candidates
			r.ask = raGenPayPick
			if s.g.pickF&pfModes == 0 || !s.unlessTapAuto(k) { // cost_unlesstapward.go
				return
			}
		case kop.PayDiscardRandom:
			s.payDiscardRandom(k, s.g.pickP, int(s.g.pickLeft))
		case kop.PayDiscardHand:
			s.payDiscardHand(k, s.g.pickP)
		case kop.PayDiscard:
			if s.g.pickF&pfWard != 0 {
				s.startWardDiscard(k) // kw_wardcost.go
				return
			}
			fallthrough
		default:
			if s.g.pickLeft > 0 {
				r.ask = raGenPayPick
				if s.autoPayPick(k) {
					// exactly the needed cards: gorge picks them (cumulative.go advanceTriggeredMandatory, pay/unlessflow.go AdvanceUnless);
					// a card one of them held in exile until it left is asked now, as after the last answered pick (CR 610.3d)
					if s.payReturnPark() {
						return
					}
					break
				}
				return
			}
		}
		s.payDone(k, true)
	case raGenPayPick:
		if s.g.pickZ == kop.PaySubCounter {
			s.applyPaySubCounterPick(k, a)
			return
		}
		o := Obj(a.Source())
		if s.g.pickZ == payKindBehold {
			s.g.pickLeft = 0 // beholding moves and marks nothing (cost_unlessbehold.go)
			s.payDone(k, true)
			break
		}
		if s.g.pickZ == payKindWaterbend {
			s.applyUnlessWaterbendPick(k, o) // cost_unlesswaterbend.go
			return
		}
		if s.g.pickZ == payPickBlight {
			if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
				s.blightSlot(ob.slot, s.value(k, s.prog(k)[r.pc].A))
			}
			s.g.pickLeft = 0
			s.payDone(k, true)
			break
		}
		if s.g.pickZ == kop.PayExileGrave || s.g.pickZ == payKindExileHand {
			s.exileFromGrave(o) // a cost exile from either zone (cost_exilefromhand.go)
		} else if s.g.pickZ == kop.PayDiscard {
			s.discardUnlessPay(k, o)
		} else if s.g.pickZ == kop.PayReturn {
			s.leave(k, s.objs[o].slot, kop.ZoneHand, false) // gorge unlessflow: ReturnCost to the owner's hand
		} else if s.g.pickZ == kop.PayTapX {
			if s.objs[o].zone == kop.ZoneBattlefield && debugEntersTappedMutate != 4 { // TestSourceCounterCostsMutation: the tap
				s.tapCostPerm(k, s.objs[o].slot, o)
			}
		} else {
			// gorge collects every pick before it sacrifices any (CR 601.2h:
			// the payment is made in full after the choices): the picks stay
			// on the battlefield, so an Aura's pick does not return the
			// creature it stole before the next menu (unlessflow, PaySacrificeCost)
			s.addSac(s.objs[o].slot)
		}
		s.g.pickLeft--
		if s.g.pickLeft > 0 {
			return
		}
		s.payCommitSacs(k) // trig_sacbatch.go
		if s.payReturnPark() {
			return
		}
		s.payDone(k, true)
	case raGenSac:
		// every player's picks are sacrificed together once all have chosen
		// (CR 101.4, flushSac)
		s.addSac(s.objs[Obj(a.Source())].slot)
		s.g.pickLeft--
		if s.g.pickLeft > 0 && s.countSacLeft(k, s.g.pickP, s.genPickFilter(k)) > 0 {
			return
		}
		p := s.repPop()
		if !s.startSac(k, &s.prog(k)[r.pc], p) {
			return
		}
		s.continueRes(k, 0)
	case raGenTopBottom:
		in := &s.prog(k)[r.pc]
		if in.C == kop.LibChoiceSecond {
			s.toLibrarySecond(k, in, a.Mode() == 0) // zone_librarysecond.go
		} else if !s.topBottomStack(k, in.Obj, a.Mode() == 0) { // a spell target (tgt_permorspell.go)
			s.moveRef(k, in.Obj, kop.ZoneLibrary, a.Mode() == 0)
		}
		if s.res.ask == raGenCmdZone {
			return // a CR 903.9b commander move parked; its answer reruns this instruction
		}
		s.continueRes(k, 0)
	case raGenCopyTarget, raGenCopyTargets:
		s.applyCopyTarget(k, a)
	case raGenResTarget:
		in := &s.prog(k)[r.pc]
		i := int(in.C)
		c := uint8(NoTarget)
		if a.Kind() == AkChoose {
			c = a.Source()
		}
		r.e.tgt[i] = c
		if r.e.nt <= uint8(i) {
			r.e.nt = uint8(i) + 1
		}
		if c != NoTarget && isObjTarget(c) {
			r.e.tver[i] = s.objs[c].ver
		}
		r.bad &^= 1 << i
		s.continueRes(k, 0)
	case raGenDivide:
		s.answerDivide(k, a.Source())
	case raGenForage:
		s.answerForageRes(k, a)
	case raGenLookAck:
		s.continueRes(k, 0)
	case raGenSearch:
		if a.Kind() == AkDone {
			s.finishSearch(k)
			return
		}
		s.resArm().look[s.g.nlook] = Obj(a.Source())
		s.g.nlook++
		s.g.pickLeft--
		var d Decision
		s.searchCands(k, &d)
		if s.g.pickLeft == 0 || d.N == 0 {
			s.finishSearch(k)
		}
	case raGenDig:
		if a.Kind() == AkChoose {
			s.digPick(Obj(a.Source()))
			s.g.pickLeft--
			if s.g.pickLeft > 0 && s.digHasPick(k) {
				return
			}
		}
		r.ask = raNone
		if s.finishDig(k) {
			return // the rest are arranged on the bottom, or a taken card's as-enters ask
		}
		s.continueRes(k, 0)
	case raGenDigAllOpt:
		r.ask = raNone
		if a.Mode() == 1 {
			in := &s.prog(k)[r.pc]
			for i := 0; i < int(s.g.nlook); i++ {
				o := s.resArm().look[i]
				if s.matchCardGen(k, in.F, s.objs[o].card, s.objs[o].owner(), r.e.ctrl, o) {
					s.g.lookSel |= 1 << i
				}
			}
			if s.finishDig(k) {
				return
			}
		}
		// no (or nothing to place): the looked-at window stays in place,
		// with no rest placement, reveal or capture.
		s.continueRes(k, 0)
	case raGenDigArrange:
		o := Obj(a.Source())
		pl := &s.pl[s.g.pickP]
		s.detach(o)
		s.objs[o].zone = kop.ZoneLibrary
		s.objs[o].ver++
		pl.libBottom(o)
		for i := 0; i < int(s.g.nlook); i++ {
			if s.resArm().look[i] == o {
				s.g.lookSel |= 1 << i
			}
		}
		if s.g.lookSel != 1<<s.g.nlook-1 {
			return
		}
		s.continueRes(k, 0)
	case raGenPileSplit:
		if a.Kind() == AkChoose {
			for i := 0; i < int(s.g.nrem); i++ {
				if s.g.rem[i] == Obj(a.Source()) && s.g.lookSel&(1<<i) == 0 {
					s.resArm().pileOrd[bits.OnesCount32(s.g.lookSel)] = uint8(i)
					s.g.lookSel |= 1 << i
				}
			}
			if bits.OnesCount32(s.g.lookSel) < int(s.g.nrem) {
				return
			}
		}
		r.ask, r.askP = raGenPileChoose, s.nextAlive(r.e.ctrl) // gorge twoPilesPlayer: the first live opponent
	case raGenPileChoose:
		// mode 0: the face-down pile (the picks), 1: the face-up pile. The
		// chosen pile goes to its zone first: the picks in pick order
		// (gorge's pile is the chosen list as answered: FDN pair 641,
		// Elementalist Adept picked before Arcane Epiphany enters the hand
		// first), the face-up pile in the revealed order; then the other
		// pile to the graveyard in the revealed order (pair 111).
		np := bits.OnesCount32(s.g.lookSel)
		pickA := a.Mode() == 0
		if pickA {
			for j := 0; j < np; j++ {
				s.pileMove(s.g.rem[s.resArm().pileOrd[j]], kop.Zone(s.g.pickZ))
			}
		}
		for i := 0; i < int(s.g.nrem); i++ {
			if inA := s.g.lookSel&(1<<i) != 0; !pickA && !inA {
				s.pileMove(s.g.rem[i], kop.Zone(s.g.pickZ))
			}
		}
		for i := 0; i < int(s.g.nrem); i++ {
			s.pileMove(s.g.rem[i], kop.ZoneGraveyard)
		}
		s.continueRes(k, 0)
	case raGenChooseEach:
		o := Obj(a.Source())
		if !s.inSet(&s.g.chosen, s.g.nchosen, o) {
			if s.g.nchosen < maxRem {
				s.g.chosen[s.g.nchosen] = o
				s.g.nchosen++
			} else {
				s.capFull(StallRemember)
			}
		}
		s.g.pickZ++
		if s.nextChooseEach(k) {
			return
		}
		s.continueRes(k, 0)
	case raGenCastFree:
		if a.Kind() == AkDone {
			s.g.nfree = 0
			s.castFreeClosed(k)
			return
		}
		o := Obj(a.Source())
		for i := 0; i < int(s.g.nfree); i++ {
			if s.g.free[i] == o {
				copy(s.g.free[i:], s.g.free[i+1:s.g.nfree])
				s.g.nfree--
				break
			}
		}
		if k.gen.anyHideaway && k.cards[s.objs[o].card].land && !DebugHideNoLand {
			if s.hideLandPark(k, o) {
				return // its as-enters choice is asked first, in exile (kw_hideaway.go)
			}
			s.hideLand(k, o) // a hidden land is played now, not queued (kw_hideaway.go)
			if s.g.nfree == 0 {
				s.castFreeClosed(k)
			}
			return
		}
		if s.g.nfq == 0 {
			s.g.holdPrio = s.prio
		}
		if s.g.nfq < uint8(len(s.g.freeQ)) {
			s.g.freeQ[s.g.nfq] = o
			s.g.nfq++
		} else {
			s.capFull(StallFree)
		}
		// the CastFree instruction still at res.pc carries the
		// Amount$ 1 and ForgetPlayed$ bits. ForgetPlayed$ is not applied at
		// the pick: the cast is only begun later, by the freeQ drain, and a
		// refused or reversed cast never happens (CR 733.1-2) and stays
		// remembered. The freeP marker forgets it once the cast commits
		// (cast.go finishBuild).
		in := &s.prog(k)[s.res.pc]
		if DebugRepeatNoOptional && in.C&128 != 0 {
			s.freeRepeatForce() // mutation only: every copy queued at the first pick
		}
		if in.C&8 != 0 && s.g.nfq != 0 && !DebugM11IgnoreOne {
			// Amount$ 1: stop offering once one card is queued
			s.g.nfree = 0
			s.castFreeClosed(k)
			return
		}
		if in.C&512 != 0 && int(s.g.nfreeAll-s.g.nfree) >= s.value(k, in.A) {
			// a literal Amount$ N: gorge's one ask has Max N, so the N-th
			// pick ends the offer
			s.g.nfree = 0
		}
		if s.g.nfree == 0 {
			s.castFreeClosed(k)
		}
	case raGenEntryOrder:
		s.applyEntryResolve(k, a) // repl_entryorder.go
	case raGenCtrOrder:
		s.applyCtrOrder(k, a) // repl_ctrorder.go
	case raGenReplOrder:
		if s.g.tok.on {
			s.tokAnswer(k, a) // a contested token creation (repl_tokorder.go)
			return
		}
		if s.replOrderPick(a) {
			return // ask again
		}
		if s.g.plusBatchAsked {
			// a pure plus-N batch order ask (trig_batches.go replPlusBatch):
			// the order cannot change the sum, so drop the parked event and
			// re-execute the same op, which now deals every hit
			s.g.ro, s.g.nc = replOrd{}, ncDmg{}
			s.res.ask = raNone
			s.runProg(k)
			return
		}
		s.g.ro.replHdr = replHdr{}
		s.dealNC(k)
		if s.g.dmgSeqOn {
			s.res.ask = raNone
			s.runProg(k) // the same damage op deals its next event
			return
		}
		s.continueRes(k, 0)
	case raGenChooseCard:
		if s.prog(k)[s.res.pc].C == chooseLibUpTo {
			s.chooseLibAnswer(k, a) // zone_optionalmoves.go: asks again or continues
			return
		}
		if a.Kind() == AkDone {
			s.g.nchosen = 0 // chose none (an up-to-one ask)
		} else {
			s.g.chosen[0] = Obj(a.Source())
			s.g.nchosen = 1
			if s.prog(k)[s.res.pc].C == 2 {
				s.remember(Obj(a.Source())) // RememberChosen$
			}
		}
		s.continueRes(k, 0)
	case raGenEnterAsk:
		if s.g.ask[0].kind == akNameLook {
			s.g.ask[0].kind = akNameAny // the look is acknowledged: the name is asked next (repl_choosename.go)
			return
		}
		saved := s.res
		s.res.active = false // the enters program runs with its own source as the context
		s.answerQueuedAsk(k, a)
		s.res = saved
		if s.g.nask != 0 && s.midResAsk(k) {
			return // gorge defers the next election behind this one; a parked tail waits for them all
		}
		if s.eachReturnResume(k) {
			return // zone_eachreturnask.go: the next target's own election
		}
		if s.unparkTail(k) {
			return // a dig's raGenDigArrange
		}
		s.continueRes(k, 0)
	case raGenRetChoice:
		o := s.g.pre.obj
		if s.g.pre.kind == akEntryOrder {
			s.answerReturnOrder(k, a)
			return
		}
		if s.g.pre.kind == akClone {
			s.answerCloneReturn(k, a)
			return
		}
		if s.g.pre.kind == akEnchant && s.g.pre.state == preDigEnchant {
			s.res.ask = raNone
			s.g.pre.val = uint8(s.objs[a.Source()].slot)
			s.g.pre.state = preDigEnchantDone
			if s.rerunMove(k, &s.prog(k)[r.pc]) {
				return // suspended again at another Aura's choice
			}
			s.g.pre = preAsk{}
			s.continueRes(k, 0)
			return
		}
		if s.g.pre.kind == akEnchant {
			paying := s.g.pre.state == preReturnPay
			s.g.pre = preAsk{}
			if s.objs[o].zone == kop.ZoneExile {
				host := s.objs[a.Source()].slot
				from := s.detach(o)
				s.enterOnto(host)
				sl := s.enterLinkRet(k, o, s.objs[o].owner(), from)
				s.attachedTo(k, sl, host)
			}
			if !paying && s.resumeReturns(k) {
				return // the batch's next return asks its host
			}
			if paying {
				s.res.ask = raNone
				s.payDone(k, true) // the payment's sacrifice finishes after the host answer
				return
			}
			s.continueRes(k, 0)
			return
		}
		s.g.pre.val = a.Mode()
		if s.g.pre.kind == akType {
			s.g.pre.val = typeVocabSub(k, int(a.Target())<<8|int(a.Mode()))
		}
		if s.g.pre.state == preHideLand {
			s.hideLandAnswer(k) // kw_hideaway.go
			return
		}
		if s.g.pre.state == preReturnOp {
			s.rerunReturn(k)
			return
		}
		paying := s.g.pre.state == preReturnPay
		s.g.pre.state = 2
		if s.objs[o].zone == kop.ZoneExile {
			from := s.detach(o)
			s.enterLinkRet(k, o, s.objs[o].owner(), from) // the enters program reads the choice
		}
		s.g.pre = preAsk{}
		if !paying && s.resumeReturns(k) {
			return // the batch's next return asks its host
		}
		if paying {
			s.res.ask = raNone
			s.payDone(k, true) // the payment's sacrifice finished: the cost resumes
			return
		}
		s.continueRes(k, 0)
	case raGenTokChoice:
		in := &s.prog(k)[r.pc]
		s.g.pre.val = a.Mode()
		if s.g.pre.kind == akClone {
			s.g.pre.val = cloneAnswer(a) // a copy token's clone election (ctr_entryperkind.go)
		}
		if s.g.pre.kind == akType {
			s.g.pre.val = typeVocabSub(k, int(a.Target())<<8|int(a.Mode()))
		}
		s.g.pre.state = 2
		t := r.lastTok
		s.objs[t].of &^= ofPending
		was := r.e.ctrl
		r.e.ctrl = s.copyCtrl(in)
		var parked bool
		if s.g.sweep.on { // copy_allpark.go: a copy-all sweep's parked copy
			parked = s.sweepAnswer(k, in, t, s.g.sweep.host) // the enters program reads the choice
		} else {
			s.copyTokenEnter(k, in, t, NoSlot, nil) // the enters program reads the choice
			s.g.pre = preAsk{}
			r.ask = raNone
			parked = s.copyTokens(k, in, s.objs[t].card, int(s.g.pickLeft))
			if !parked {
				parked = s.eachResume(k, in) // copy_eachtarget.go: the targets after it
			}
		}
		r.e.ctrl = was
		if parked {
			return
		}
		s.continueRes(k, 0)
	case raGenEnchant:
		in := &s.prog(k)[r.pc]
		switch in.Op {
		case kop.OpReturnEx:
			r.ask = raNone
			s.execReturn(k, in, s.objs[a.Source()].slot)
			s.continueRes(k, 0)
			return
		case kop.OpReturn:
			r.ask = raNone
			s.returnAura(k, s.returningObj(in), s.objs[a.Source()].slot)
			s.continueRes(k, 0)
			return
		case kop.OpChooseMove:
			r.ask = raNone
			s.moveFoundAns(k, s.objs[a.Source()].slot) // zone_handmoveaura.go: enter, attach, remember
			if s.moveAsked() || s.res.left == tailSearchEnter {
				return // a later card parked too; its answer resumes here
			}
			s.continueRes(k, 0)
			return
		case kop.OpSearchEx:
			r.ask = raNone
			s.moveFoundAns(k, s.objs[a.Source()].slot)
			// The move may itself have parked a later card's as-enters ask by
			// setting res.left = tailSearchEnter (moveFoundAns's parkTail): the
			// arm's own parkTail must not overwrite it with tailSearch, or the
			// cards after that card never move.
			if s.moveAsked() || s.res.left == tailSearchEnter {
				return // a later card parked too; its answer resumes here
			}
			if !s.parkTail(k, tailSearch) {
				s.finishSearchTail(k)
				s.continueRes(k, 0)
			}
			return
		}
		t := r.lastTok
		s.objs[t].of &^= ofPending
		was := r.e.ctrl
		r.e.ctrl = s.copyCtrl(in)
		var parked bool
		if s.g.sweep.on { // copy_allpark.go: a copy-all sweep's parked copy
			parked = s.sweepAnswer(k, in, t, s.objs[a.Source()].slot)
		} else {
			s.copyTokenEnter(k, in, t, s.objs[a.Source()].slot, nil)
			r.ask = raNone
			parked = s.copyTokens(k, in, s.objs[t].card, int(s.g.pickLeft))
			if !parked {
				parked = s.eachResume(k, in) // copy_eachtarget.go: the targets after it
			}
		}
		r.e.ctrl = was
		if parked {
			return
		}
		s.continueRes(k, 0)
	case raGenCmdZone:
		s.cmdZoneAnswer(k, a) // zone_cmdzone.go
	case raRecruitDiscard:
		s.recruitPick(k, a) // act_recruit.go
		s.continueRes(k, 0)
	default:
		if !s.applyAskCold1(k, a) {
			panic("kernel: bad resolution answer")
		}
	}
}

// payDone finishes an unless / if-you-do / ward payment.
func (s *State) payDone(k *prep, paid bool) {
	s.noteExileTrigNewOutcome(paid) // the trigger-own ExileAnyGrave tally (trig_mayexile.go)
	f := s.g.pickF
	if f&pfWard != 0 {
		s.res.ask = raNone
		if !paid {
			if idx := s.wardedIndex(&s.res.e); idx >= 0 {
				s.wardCounterAt(k, idx)
			}
		}
		s.finishResolve(k, false)
		return
	}
	if f&pfEachNext != 0 && !paid {
		s.payEachNext(k) // cost_anyplayerpays.go: the next player is asked
		return
	}
	if paid && s.g.pickZ == kop.PayExileGrave {
		if s.prog(k)[s.res.pc].B.N == kop.PayBSubStackSelf {
			s.exileStackSelf(k) // ExileFromStack<1/Card.Self> (cost_exilestack.go)
		}
		s.chainCharge(k) // the mana half of a mana-and-exile cost (cost_exilepool.go)
	}
	skip := uint8(0)
	if (f&pfIfPaid != 0) != paid {
		skip = uint8(f & 0x7ff)
	}
	if paid && s.g.pickZ == kop.PayMana && s.res.e.kind == seTrigger {
		if life := s.prog(k)[s.res.pc].F; life != 0 {
			s.loseLife(k, s.g.pickP, int(life)) // the mana-and-life cost's life (Zoraline)
		}
		s.trigXBind(k) // a PayManaX cost: the announced X is Count$xPaid
	}
	if paid && s.g.pickZ == payKindEvidence && s.res.e.kind == seTrigger {
		s.evidenceXBind(k) // CollectEvidence<X>: the announced X is Count$xPaid
	}
	s.continueRes(k, skip)
}

// payWardN answers a ward's {N}: paid (pay) through the ward mana window,
// whose shortfall counters the warded entry, else countered; the ward then
// finishes resolving. The plain {N} ward and the discard-or-{N} ward's mana
// pick (kw_wardcost.go) share it.
//
//go:noinline
func (s *State) payWardN(k *prep, n uint8, pay bool) {
	r := &s.res
	idx := s.wardedIndex(&r.e)
	// A live tap-another-permanent source is a window source the census above
	// leaves out (availFor counts no tap cost), so opening the window is what
	// gorge does: it taps the plain sources and asks which permanent to tap.
	if c := (kop.ManaCost{Generic: n}); pay && (k.gen == nil || s.affordsFor(k, r.e.trigPl, &c) || s.windowAffords(k, r.e.trigPl, &c) || s.nonSpellRestrAffords(k, r.e.trigPl, &c) || s.windowTapXLive(k, r.e.trigPl).any()) {
		paused, short := s.payWindowAsk(k, r.e.trigPl, &c, winWard)
		if paused {
			return // a tap-another source's pick (mana_windowtapx.go)
		}
		if short && idx >= 0 {
			s.wardCounterAt(k, idx) // the colours taken left the ward unpaid
		}
	} else if idx >= 0 {
		if pay {
			s.tapAllForMana(k, r.e.trigPl) // gorge taps what it has, then can't pay
		}
		s.wardCounterAt(k, idx)
	}
	s.finishResolve(k, false)
}

// resolveWardLife resolves a ward—pay N life trigger (trigAmt = -N).
func (s *State) resolveWardLife(k *prep) {
	e := &s.res.e
	idx := s.wardedIndex(e)
	if idx < 0 {
		s.finishResolve(k, false)
		return
	}
	n := int(-e.trigAmt)
	if int(s.pl[e.trigPl].life) < n {
		s.wardCounterAt(k, idx)
		s.finishResolve(k, false)
		return
	}
	// gorge's effWard counters an unpayable life ward without asking (CR 119.4).
	s.g.pickF = pfWard
	s.g.pickZ = kop.PayLife
	s.g.pickLeft = uint8(n)
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}

// --- each-player sacrifice ---

// startSac poses player p's sacrifice of OpSacrifice in; false = suspended.
// countSacLeft counts p's sacrifice candidates not picked yet.
func (s *State) countSacLeft(k *prep, p uint8, f kop.FilterID) int {
	src := s.resSrcSlot(k)
	n := 0
	for m := s.ctrlM[p].andNot(s.g.sacM); m.any(); m = m.rest() {
		if s.sacMatchGen(k, f, m.first(), p, src) {
			n++
		}
	}
	return n
}

func (s *State) startSac(k *prep, in *kop.Instr, p uint8) bool {
	if k.gen != nil && k.gen.ctrlSBA {
		// gorge settles control after every event (emit.go): an Aura an
		// earlier instruction sacrificed no longer steals the creature this
		// one counts (Vile Mutilator's enchantment, then creature; CR 613.1b)
		s.ctrlSBA(k)
	}
	src := s.resSrcSlot(k)
	for p != 255 {
		n := s.value(k, in.A)
		if n <= 0 {
			// gorge effSacrifice: an Amount$ of 0 or less sacrifices nothing and asks nothing
			p = s.repPop()
			continue
		}
		cnt := s.countSac(k, p, in.F, src)
		if cnt > n {
			s.g.pickP, s.g.pickLeft, s.g.pickZ = p, uint8(n), kop.PaySac
			s.res.ask, s.res.askP = raGenSac, p
			return false
		}
		// no choice: every match, in battlefield order (gorge's eligible
		// list walks the zone)
		var ord [MaxPerm]Slot
		no := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
		for _, sl := range ord[:no] {
			if s.sacMatchGen(k, in.F, sl, p, src) {
				s.addSac(sl)
			}
		}
		p = s.repPop()
	}
	s.flushSac(k)
	return true
}

// flushSac sacrifices the permanents collected for one effect together, once
// every player has chosen (CR 101.4; CR 701.21: Blasphemous Edict's Spinner of
// Souls and High-Society Hunter see every other creature die): player by player
// in the order asked (the active player first for "each player"), each player's
// picks in answer order, a forced batch in battlefield order. What a sacrificed
// source exiled returns after them (CR 610.3d).
func (s *State) flushSac(k *prep) {
	if s.g.nsac == 0 {
		s.g.sacM = slotMask{}
		s.holdClose(k, s.holdOpen(k))
		return
	}
	ord, n := s.g.sacOrd, int(s.g.nsac)
	s.g.nsac = 0
	batch := k.gen != nil && k.gen.anyBatch
	var bOld uint8
	if batch {
		bOld = s.batchOpen() // one sacrifice batch (trig_oneormore.go)
	}
	hr := s.holdOpen(k)
	// The moves before the first parked one (repl_sacbatch.go) go now, as
	// gorge's earlier emits did; the parked ones are sacrificed and move after
	// the answer, in the same batch.
	w, parked := s.sacParkFrom(k, ord[:n])
	var zs [MaxPerm]kop.Zone
	if w > 1 && s.subs[kop.TrigSacrificed].any() {
		s.sacEachDeparture(k, ord[:w], &zs) // one walk per departure (trig_sacqueueorder.go)
	} else {
		for _, sl := range ord[:w] {
			s.fireSac(k, sl)
		}
		for i, sl := range ord[:w] {
			zs[i] = s.dieZone(k, sl)
		}
		for i, sl := range ord[:w] {
			s.fireLeave(k, sl, zs[i])
		}
	}
	for i, sl := range ord[:w] {
		s.removePerm(k, sl, zs[i], false)
	}
	if parked && hr.on {
		// gorge holds an until-leaves return to the end of the whole flush;
		// the parked moves run after this frame closes, so it cannot be told
		// apart from a return between the moves.
		s.capFull(StallReplLeave)
	}
	carried := parked && s.sacParkTail(ord[w:n], batch, bOld)
	s.g.sacM = slotMask{}
	s.holdClose(k, hr)
	if batch && !carried {
		s.batchClose(bOld)
	}
}

// addSac adds slot sl to the effect sacrifice being collected.
func (s *State) addSac(sl Slot) {
	if s.g.sacM.has(sl) {
		return
	}
	if int(s.g.nsac) >= maxSacOrd {
		s.capFull(StallSac)
		return
	}
	s.g.sacM.set(sl)
	s.g.sacOrd[s.g.nsac] = sl
	s.g.nsac++
}

// --- searches ---

func (s *State) startSearch(k *prep, in *kop.Instr) bool {
	if in.Op == kop.OpChooseMove && in.B.N == 1 && s.handExileHostGone() {
		return true // UntilHostLeavesPlay with the host gone: nothing (act_mayask.go)
	}
	p := s.playerRef(k, handMoveOwnerRef(in.Obj))
	if p == 255 {
		if in.Op == kop.OpChooseMove && in.Obj != kop.RefController {
			return true // the target player whose hand it is is gone: nothing moves
		}
		p = s.res.e.ctrl
	}
	s.res.left = 0 // the parked-tail marker (zone_parkedtail.go)
	s.g.pickP = p
	s.g.pickLeft = uint8(s.value(k, in.A))
	s.g.pickZ = uint8(in.C & 7)
	s.g.pickF = uint16(in.C>>3) | uint16(in.B.N)&(kop.SrAttacking|kop.SrDifferentNames|kop.SrNoShuffle|kop.SrRandom|kop.SrGameGrave)
	s.g.nlook = 0
	if altOp(in) {
		s.altBegin() // OriginAlternative$ (zone_originalternative.go)
	}
	if sideOp(in) {
		s.sideClamp(k, in) // zone_sideboardsearch.go: the pool is the library and the sideboard
	} else if s.g.pickF&kop.SrDifferentNames != 0 {
		s.clampDistinctNames(k, in)
	}
	if eachOp(k, in) {
		s.clampEach(k, in) // zone_each.go
	}
	if ask, ok := s.searchConfirmAsk(k, in, p); ok {
		s.res.ask, s.res.askP = raGenConfirm, ask // gorge's Optional$ confirmation first
		return false
	}
	return s.searchPick(k) && !s.moveAsked()
}

// moveAsked reports a found card's move waiting on an answer: a hand move's Aura for
// its host (handAuraEnter), a card entering for its as-enters choice (rerunMove), or
// an Attacking$ entry's CR 508.4 defender (zone_changezoneparams.go).
func (s *State) moveAsked() bool {
	return s.res.ask == raGenEnchant || s.res.ask == raGenRetChoice || s.res.ask == raGenEntryOrder ||
		s.res.ask == raGenEnterDefender || s.res.ask == raGenCmdZone
}

// searchPick poses a search's (or a choose-move's) first pick, or finishes
// it at once when nothing is to be picked.
func (s *State) searchPick(k *prep) bool {
	if s.altPending(k) {
		return false // the zone election is asked first (zone_originalternative.go)
	}
	if s.searchZone(k) == kop.ZoneHand && s.g.pickF&kop.SrMandatory != 0 && s.handTakeAll(k) {
		return true // a required hand move with no choice (gorge handMoveOwnersWalk)
	}
	p := s.g.pickP
	if s.g.pickF&kop.SrRandom != 0 {
		s.randomGravePick(k) // AtRandom$ True, nobody is asked
		return true
	}
	var d Decision
	s.searchCands(k, &d)
	if d.N == 0 && s.prog(k)[s.res.pc].B.N&kop.SrImprint != 0 {
		// gorge effSearchLibrary's fail-to-find drops the resolution's remembered
		// objects (zone_search.go, c.Remembered = nil): Sanar's later colour
		// picks then see an empty set.
		s.g.nrem = 0
	}
	if d.N == 0 || s.g.pickLeft == 0 {
		s.finishSearchNow(k)
		return true
	}
	s.res.ask, s.res.askP = raGenSearch, s.chooserOf(k, p)
	return false
}

// searchZone is the zone the search op in progress looks in.
func (s *State) searchZone(k *prep) kop.Zone {
	in := &s.prog(k)[s.res.pc]
	if in.Op == kop.OpChooseMove {
		if s.g.pickF&kop.SrGraveExile != 0 {
			return kop.ZoneNone // graveexile: the walk in graveExileCands owns the zones
		}
		if in.B.N&kop.SrHandGrave != 0 {
			return kop.ZoneNone // handGraveCands owns the zones
		}
		z := k.t.Filters[in.F].Zone
		if z == kop.ZoneNone {
			// kgen names every other zone; a battlefield filter is zoneless
			// (a hidden battlefield pick: trig_delayedtiming.go battlefieldCands)
			z = kop.ZoneBattlefield
		}
		return z
	}
	if in.B.N&kop.SrAltOrigin != 0 {
		return s.altSearchZone(in) // OriginAlternative$: the elected zones (zone_originalternative.go)
	}
	if in.B.N&(kop.SrSideboard|kop.SrNoLibrary) == kop.SrSideboard|kop.SrNoLibrary {
		return kop.ZoneSideboard // Origin$ Sideboard alone: the library is not searched (zone_sideboardsearch.go)
	}
	return kop.ZoneLibrary
}

func (s *State) searchCands(k *prep, d *Decision) {
	in := &s.prog(k)[s.res.pc]
	if in.Op == kop.OpChooseMove && s.g.pickF&kop.SrGraveExile != 0 {
		s.graveExileCands(k, in, d)
		return
	}
	if in.Op == kop.OpChooseMove && in.B.N&kop.SrHandGrave != 0 {
		s.handGraveCands(k, in, d)
		return
	}
	if altOp(in) {
		s.altOriginCands(k, in, d)
		return
	}
	if sideOp(in) {
		s.sideCands(k, in, d) // zone_sideboardsearch.go
		return
	}
	p := s.g.pickP
	pl := &s.pl[p]
	var list []Obj
	switch s.searchZone(k) {
	case kop.ZoneBattlefield:
		s.battlefieldCands(k, in, d)
		return
	case kop.ZoneLibrary:
		list = pl.lib[pl.libLo:pl.libHi]
	case kop.ZoneGraveyard:
		if in.Op == kop.OpChooseMove && s.g.pickF&(kop.SrUnderYourControl|kop.SrGameGrave) != 0 {
			s.gameGraveCands(k, in, d) // trig_sacqueueorder.go: an ownerless GainControl$ pick, or a game-wide graveyard pick (gorge effHiddenPick gameWide)
			return
		}
		list = pl.grave[:pl.graveN]
	case kop.ZoneHand:
		list = pl.hand[:pl.handN]
	case kop.ZoneExile:
		s.exwPickCands(k, in, d) // zone_exiledwithpick.go: every seat's exile
		return
	default:
		return
	}
	if eachOp(k, in) {
		s.eachCands(k, in, d, list) // zone_each.go
		return
	}
	if s.g.pickF&kop.SrDifferentNames != 0 {
		s.differentNameCands(k, in, d, list)
		return
	}
	if shareLandTypeOp(in) && s.g.nlook > 0 {
		s.shareLandTypeCands(k, in, d, list) // zone_sharelandtype.go
		return
	}
	for _, o := range list {
		if s.picksHas(o) {
			continue
		}
		if s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) && s.pickFits(k, in.F, o, false) {
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
}

func (s *State) picksHas(o Obj) bool {
	for i := 0; i < int(s.g.nlook); i++ {
		if s.resArm().look[i] == o {
			return true
		}
	}
	return false
}

func (s *State) finishSearch(k *prep) {
	s.res.ask = raNone
	if s.entryParkSearch(k) {
		return // a land's CR 616.1 entry-order ask, the card still in the hand (repl_entryorder.go)
	}
	s.finishSearchNow(k)
	if s.moveAsked() {
		return
	}
	if s.entryLatePark() {
		return // the order ask that follows the land's entry (repl_entryorder.go)
	}
	s.continueRes(k, 0)
}

// finishSearchNow moves the found cards and always shuffles a library
// search's library after the moves: a ThenTop card goes to the bottom
// first, then the shuffle, then the card on top (gorge's searchShuffleTail,
// e83159f1a; SrShuffleOptional is no longer read).
func (s *State) finishSearchNow(k *prep) {
	p := s.g.pickP
	fl := s.g.pickF
	if fl&kop.SrForgetOther != 0 {
		// ForgetOtherRemembered$ (graveexile): gorge forgets the other cards
		// before the moves, and also when the pool is empty. The picks are
		// already chosen, so clearing cannot change the candidates.
		s.g.nrem = 0
	}
	lib := s.searchZone(k) == kop.ZoneLibrary && fl&kop.SrGraveExile == 0
	if lib && fl&kop.SrThenTop != 0 {
		// gorge moves the found card first (to the library's bottom), then
		// shuffles, then puts it on top (placeLibraryObjects)
		pl := &s.pl[p]
		for i := 0; i < int(s.g.nlook); i++ {
			o := s.resArm().look[i]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	} else {
		s.moveFound(k)
		if s.moveAsked() || s.res.left == tailSearchEnter {
			return
		}
		if s.parkTail(k, tailSearch) {
			return // a found card's as-enters ask: the shuffle runs once it is answered
		}
	}
	s.finishSearchTail(k)
}

// finishSearchTail is a library search's shuffle and a ThenTop card's move
// to the top, after the found cards' moves (and, when one entered with an
// as-enters choice, after the choice is answered: gorge suspends the search
// at the ask).
func (s *State) finishSearchTail(k *prep) {
	if k.gen != nil && k.gen.anyKwAct && s.searchZone(k) == kop.ZoneLibrary && s.g.pickF&kop.SrGraveExile == 0 {
		if q := s.chooserOf(k, s.g.pickP); q == s.g.pickP {
			s.fireKwAct(k, kop.TrigSearchedLibrary, q, 0) // trig_modes.go
		}
	}
	p := s.g.pickP
	fl := s.g.pickF
	if s.searchZone(k) == kop.ZoneLibrary && fl&kop.SrGraveExile == 0 && fl&kop.SrNoShuffle == 0 {
		s.shuffleLib(p)
	}
	if fl&kop.SrThenTop != 0 {
		s.moveFound(k)
	}
}

// moveFound moves the searched-for cards to the search's destination.
func (s *State) moveFound(k *prep) { s.moveFoundAns(k, NoSlot) }

// moveFoundAns is moveFound; host is the answered enchant ask of the
// one-card hand move onto the battlefield parked at it (handAuraEnter);
// the re-run's first look card is that parked card, and only it consumes
// host.
func (s *State) moveFoundAns(k *prep, host Slot) {
	p := s.g.pickP
	z := kop.Zone(s.g.pickZ)
	fl := s.g.pickF
	if z == kop.ZoneLibrary && s.g.nlook > 1 && (s.searchZone(k) == kop.ZoneHand || fl&kop.SrNoShuffle != 0) {
		s.reverseLook() // act_mayask.go: the first answered ends on top (an unshuffled search too)
	}
	// CR 603.6a: the cards a search puts onto the battlefield enter together, so
	// each one's enters check sees the whole batch (gorge's zone batch): Roiling
	// Canopy's "five other Forests" holds for both of two Forests entering as
	// the sixth. The checks fire when the walk ends or suspends.
	var eb entBatch
	defer s.fireEntries(k, &eb)
	for i := 0; i < int(s.g.nlook); i++ {
		o := s.resArm().look[i]
		if z == kop.ZoneNone {
			if fl&kop.SrRemember != 0 {
				s.remember(o) // a reveal pick's RememberRevealed$
			}
			continue
		}
		if s.objs[o].zone == kop.ZoneBattlefield {
			if s.pickLeaves(k, o, z) { // a hidden pick from the battlefield (trig_delayedtiming.go)
				// a CR 903.9b commander move: the cards from this one on wait on
				// the battlefield for the answer (zone_cmdzone.go)
				n := copy(s.resArm().look[:], s.resArm().look[i:s.g.nlook])
				s.g.nlook = uint8(n)
				return
			}
			continue
		}
		ep := p
		if z == kop.ZoneBattlefield {
			if fl&kop.SrGraveExile != 0 && fl&kop.SrUnderYourControl == 0 {
				ep = s.objs[o].owner() // graveexile: gorge enters a hidden pick under its owner unless GainControl$
			}
			if DebugGyBfOwnerEntry && fl&kop.SrUnderYourControl != 0 {
				ep = s.objs[o].owner() // mutation: a GainControl$ graveyard pick keeps its owner
			}
		}
		if z == kop.ZoneBattlefield && s.handAuraArmed(k) {
			ans := host // the answered enchant ask is for the parked card, the look's first
			if i != 0 {
				ans = NoSlot // every card after it asks for itself again
			}
			var parked, stay bool
			if host, parked, stay = s.handAuraEnter(k, o, ep, ans); parked {
				// the card waits in its zone for the enchant ask; its answer moves the
				// cards from this one on, not the earlier ones that entered
				n := copy(s.resArm().look[:], s.resArm().look[i:s.g.nlook])
				s.g.nlook = uint8(n)
				return
			} else if stay {
				if fl&kop.SrRemember != 0 {
					s.remember(o) // gorge settleHandMove remembers a card whose entry was swallowed too
				}
				continue
			}
		}
		if z == kop.ZoneBattlefield {
			if host == NoSlot && s.cloneReturnPark(k, o, ep) {
				// the card waits in its zone for the answer (rerunMove), which
				// moves the cards from this one on
				n := copy(s.resArm().look[:], s.resArm().look[i:s.g.nlook])
				s.g.nlook = uint8(n)
				return
			}
			if host == NoSlot && s.entryParkFound(k, o, ep) {
				// the CR 616.1 entry-order ask, the card still in its zone; the
				// answer moves the cards from this one on (repl_entryorder.go)
				n := copy(s.resArm().look[:], s.resArm().look[i:s.g.nlook])
				s.g.nlook = uint8(n)
				return
			}
		}
		zz, bb := z, false
		if s.cmdZoneReplace(k, o, &zz, &bb) {
			// a CR 903.9b commander move: the cards from this one on wait in
			// their zone for the answer (zone_cmdzone.go)
			n := copy(s.resArm().look[:], s.resArm().look[i:s.g.nlook])
			s.g.nlook = uint8(n)
			return
		}
		from := s.detach(o)
		if zz == kop.ZoneBattlefield && k.gen != nil && k.gen.anyMayLimit {
			s.countMayPlayLand(k, o, from)
		}
		switch zz {
		case kop.ZoneBattlefield:
			hg := from != kop.ZoneLibrary
			if hg {
				s.handGraveArm(k) // WithCountersType$ on a hand/graveyard pick
			}
			s.enterOnto(host)
			var sl Slot
			if s.g.nlook > 1 && host == NoSlot && s.g.entHost == 0 && !(s.g.pre.ordMid && s.g.pre.obj == o) {
				sl = s.enterHeld(k, &eb, o, ep, fl&kop.SrTapped != 0, from)
				if from == kop.ZoneGraveyard {
					s.fireLeavesGraveyardSelf(k, sl, o) // trig_leftgraveyard.go, as enter does
				}
			} else {
				sl = s.enter(k, o, ep, fl&kop.SrTapped != 0, from)
			}
			if s.g.pre.obj == o {
				s.g.pre = preAsk{}
			}
			if hg {
				s.g.enterCtr.n = 0
			}
			s.attachedTo(k, sl, host) // CR 303.4f: an Aura enters attached
			if fl&kop.SrAttacking != 0 {
				s.enterAttacking(k, sl, ep)
			}
			if fl&kop.SrRemember != 0 {
				s.remember(o) // RememberChanged$ (gorge settleHandMove's eventRemember)
			}
		case kop.ZoneLibrary:
			s.placeFromK(k, o, kop.ZoneLibrary, false, from, false)
			if fl&kop.SrRemember != 0 {
				s.remember(o) // Shuffle$ False's library-to-library search
			}
		default:
			s.placeFromK(k, o, zz, bb, from, false)
			if fl&kop.SrRemember != 0 {
				s.remember(o) // a hand move's RememberChanged$
			}
			if z == kop.ZoneExile && from == kop.ZoneHand && s.prog(k)[s.res.pc].B.N == 1 {
				s.handExileLink(o) // until the source leaves (act_mayask.go)
			}
		}
		s.forgetFound(k, o)  // ForgetChanged$ (SrForgetChanged): the moved card leaves the remembered set
		s.imprintFound(k, o) // Imprint$ (SrImprint): the moved card joins the chosen set
		if z == kop.ZoneBattlefield && s.parkEnterDefender(tailSearchEnter) {
			// the entry posed the CR 508.4 defender ask: the cards after it
			// stay where they are until it is answered, as they do behind an
			// as-enters ask
			s.g.nlook = uint8(copy(s.resArm().look[:], s.resArm().look[i+1:s.g.nlook]))
			return
		}
		if z == kop.ZoneBattlefield && s.entryLateTail(k, tailSearchEnter) {
			s.g.nlook = uint8(copy(s.resArm().look[:], s.resArm().look[i+1:s.g.nlook]))
			return // the re-ask that follows the entry (repl_entryorder.go)
		}
		if z == kop.ZoneBattlefield && i+1 < int(s.g.nlook) && s.parkTail(k, tailSearchEnter) {
			// the entered card queued an as-enters ask (a shock land's pay-life
			// election): gorge suspends the search there, so the cards after it
			// stay where they were until it is answered
			s.g.nlook = uint8(copy(s.resArm().look[:], s.resArm().look[i+1:s.g.nlook]))
			return
		}
		host = NoSlot // an Aura's host is for its own entry; the next card finds its own
	}
	s.g.nlook = 0
}

// --- dig ---

func (s *State) startDig(k *prep, in *kop.Instr) bool {
	q := s.eachPlayers(k, in.Obj)
	if in.Obj == kop.RefEachPlayer {
		// a Dig's Defined$ Player is the seat-order sweep, AliveFrom(0), not
		// AliveFrom(controller): the remember order it builds is the order
		// the chained Play's candidates carry (CastFreeIndex; Etali pair 99)
		q = s.aliveFrom(0)
	}
	if q.n == 0 {
		q = seatOne(s.res.e.ctrl)
	}
	single := q.n == 1
	s.res.left = 0 // the parked-tail marker (zone_parkedtail.go)
	take := int(in.B.N)
	if in.B.K != kop.VConst {
		take = s.value(k, in.B)
		if take < 0 {
			take = 0 // gorge effDig: a negative ChangeNum takes nothing, not the -1 "all" sentinel
		}
	}
	for i := uint8(0); i < q.n; i++ {
		p := q.s[i]
		pl := &s.pl[p]
		n := s.value(k, in.A)
		if n < 0 {
			n = 0 // gorge effDig: a negative DigNum looks at nothing (Jessica Jones with a negative power)
		}
		if n > pl.libLen() {
			n = pl.libLen()
		}
		if n > len(s.resArm().look) {
			s.capFull(StallLook) // loud: gorge digs them all (selfcast)
			n = len(s.resArm().look)
		}
		s.g.nlook = uint8(n)
		s.g.lookSel = 0
		s.g.pickP = p
		for i := 0; i < n; i++ {
			s.resArm().look[i] = pl.lib[int(pl.libLo)+i]
			s.resArm().lookOrig[i] = uint8(i)
		}
		if take == -1 && single && (in.C>>3)&kop.DigOptional != 0 && s.g.nlook > 0 && s.digHasPick(k) {
			// ChangeNum$ All beside Optional$ True (gorge effDig's
			// allOrNothing election): one yes/no, never a subset. Yes takes
			// every matching card; no leaves the looked-at window in place
			// and moves nothing (no rest placement, reveal or capture).
			s.res.ask, s.res.askP = raGenDigAllOpt, s.res.e.ctrl
			return false
		}
		if take == -1 {
			// ChangeNum$ All: every matching card, not every looked-at
			// one (gorge effDig; Bloodline Shaman's chosen-type Dig)
			for i := 0; i < n; i++ {
				o := s.resArm().look[i]
				if s.matchCardGen(k, in.F, s.objs[o].card, s.objs[o].owner(), s.res.e.ctrl, o) {
					s.g.lookSel |= 1 << i
				}
			}
			if s.finishDigAt(k, single, false) && s.res.ask != raNone {
				// a taken card's as-enters ask or a commander's CR 903.9b
				// park stops the each-player loop here; the answer re-runs
				// this player's window (not the next player's)
				return false
			}
			continue
		}
		if take == -2 {
			take = n
		}
		s.g.pickLeft = uint8(take)
		if !single {
			panic("kernel: an each-player dig with a choice")
		}
		if n == 0 || take == 0 || !s.digHasPick(k) || s.digForced(k) {
			return !s.finishDig(k)
		}
		// the controller picks, whoever's library it is (gorge
		// digChooser: the resolving controller, CR 701.29a; Coral
		// Fighters looks at the defending player's library)
		s.res.ask, s.res.askP = raGenDig, s.res.e.ctrl
		return false
	}
	return true
}

// heldPark keeps the outermost frame hr open across a park before a take:
// the entrants so far wait, enters checks held, for the cards entering after
// the answer, and the pass run again from it (finishDigAt sees batchHeld
// still set) fires them all at the dig's end (CR 603.6a). true: the frame is
// left open.
func (s *State) heldPark(hr holdRec) bool {
	if !hr.on || hr.h != 0 {
		return false
	}
	s.g.retTS = hr.ts
	return true
}

func (s *State) digHasPick(k *prep) bool {
	f := s.prog(k)[s.res.pc].F
	for i := 0; i < int(s.g.nlook); i++ {
		o := s.resArm().look[i]
		if s.g.lookSel&(1<<i) == 0 && s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), s.res.e.ctrl, o) && s.pickFits(k, f, o, true) {
			return true
		}
	}
	return false
}

// finishDig moves the picked looked-at cards to the dig's zone and the rest
// to its rest zone.
func (s *State) finishDig(k *prep) bool { return s.finishDigAt(k, true, false) }

// finishDigAt is finishDig; park: a taken card's queued as-enters ask
// suspends the dig, its tail (the rest's ordering and placement, or the
// arrange pose) running when the ask is answered (parkTail). It reports
// whether an ask is posed (raGenDigArrange, or the parked tail's
// raGenEnterAsk).
//
// resume: the run again from a parked tail that kept the entrants' enters
// checks held (tailDigEnterH), so they fire with the cards entering after the
// answer, once the whole dig is done, as gorge's one zone batch does.
func (s *State) finishDigAt(k *prep, park, resume bool) bool {
	in := &s.prog(k)[s.res.pc]
	p := s.g.pickP
	pl := &s.pl[p]
	n := int(s.g.nlook)
	// A commander's CR 903.9b park carries the look index it stopped at: the
	// answer re-runs the take loop from that card, so cards it already took
	// (and a library destination has already placed) are not taken twice
	// (CR 400.7 would make them new objects). moveFoundAns truncates instead;
	// a dig's rest ordering still needs the whole window.
	start := 0
	if pre := &s.g.pre; pre.kind == akCmdZone && (pre.state == preCmdZoneMove || pre.state == preCmdZoneCmd) {
		start = int(pre.val)
	}
	z := kop.Zone(in.C & 7)
	fl := in.C >> 3
	rz := kop.Zone(fl & 7)
	if k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // gorge effDig: one zone batch (trig_oneormore.go)
	}
	var hr holdRec
	deferred := false // a parked tail keeps the entrants' checks held for the answer
	if z == kop.ZoneBattlefield {
		// gorge effDig's zone batch: the taken cards enter together (CR 603.6a)
		if resume || s.g.batchLatch&batchHeld != 0 {
			// resume, or a pass run again from the answer to a park before
			// a take (heldPark), which left the frame open
			hr = holdRec{on: true, ts: s.g.retTS}
		} else {
			hr = s.holdEntries()
		}
		defer func() {
			if !deferred {
				s.holdClose(k, hr)
			}
		}()
	}
	arranges := digArranges(n, s.g.lookSel, rz, fl)
	cmdPick := -1
	if (z == kop.ZoneHand || z == kop.ZoneLibrary) && s.cfg&cfgCommander != 0 {
		for i := 0; i < n; i++ {
			o := s.resArm().look[i]
			if s.g.lookSel&(1<<i) != 0 && s.cmdZoneCommander(o) {
				cmdPick = i
				break
			}
		}
	}
	if arranges || z == kop.ZoneBattlefield || cmdPick >= 0 {
		// the taken cards move one at a time with the rest still on top; one
		// entering the battlefield is asked its as-enters choice in the library
		// (gorge's parked entry), and this runs again from the answer
		for i := start; i < n; i++ {
			o := s.resArm().look[i]
			if s.g.lookSel&(1<<i) == 0 || s.objs[o].zone != kop.ZoneLibrary {
				continue
			}
			if z == kop.ZoneBattlefield && park && s.cloneReturnPark(k, o, s.objs[o].owner()) {
				deferred = s.heldPark(hr)
				return true
			}
			if z == kop.ZoneBattlefield && park && s.entryParkDig(k, o, p) {
				deferred = s.heldPark(hr)
				return true // the CR 616.1 entry-order ask, the card still in the library (repl_entryorder.go)
			}
			if z == kop.ZoneBattlefield && park && s.digAuraPark(k, o, p) {
				deferred = s.heldPark(hr)
				return true // CR 303.4f: the Aura waits in the library for its controller's choice of what it enchants
			}
			if z == kop.ZoneBattlefield && s.digAuraStays(k, o, p) {
				s.g.lookSel &^= 1 << i // CR 303.4g: an Aura with nothing to enchant stays in the library with the rest
				continue
			}
			zz, bb := z, false
			if s.cmdZoneReplace(k, o, &zz, &bb) {
				deferred = s.heldPark(hr)
				// resume the take loop at this card after the answer
				s.g.pre.val = uint8(i)
				return true // a CR 903.9b commander move parked before it happens
			}
			s.detach(o)
			s.digTake(k, o, zz, fl)
			if s.g.pre.obj == o {
				s.g.pre = preAsk{}
			}
			if z == kop.ZoneBattlefield && park && s.entryLateTail(k, tailDigEnter) {
				return true // the re-ask that follows the entry (repl_entryorder.go)
			}
			if z == kop.ZoneBattlefield && park && i+1 < n && s.g.lookSel>>(i+1) != 0 && s.parkTail(k, tailDigEnter) {
				if hr.h == 0 {
					s.res.left, s.g.retTS, deferred = tailDigEnterH, hr.ts, true
				}
				return true
			}
		}
	}
	if arranges {
		// "the rest on the bottom in any order": the rest stay on top while their
		// owner orders them, one pick per card, each going to the bottom in pick
		// order (gorge's dig_arrange, a full permutation)
		if park && s.parkTail(k, tailDigArrange) {
			return true
		}
		s.res.ask, s.res.askP = raGenDigArrange, p
		return true
	}
	if z == kop.ZoneBattlefield {
		// a taken card's queued as-enters ask suspends the dig with the rest
		// still in the library, as gorge's parked effect has them
		if park && s.parkTail(k, tailDig) {
			if hr.h == 0 {
				s.res.left, s.g.retTS, deferred = tailDigH, hr.ts, true
			}
			return true
		}
		s.digRestAside(n)
		s.finishDigTail(k)
		return false
	} else if cmdPick >= 0 {
		// a picked commander was moved one card at a time above; the rest go
		// where the dig's rest zone says; the picks may already sit on top, so
		// the rest come out by identity, not by advancing the window
		s.digRestOut(n)
		s.finishDigTail(k)
		return false
	} else {
		for i := 0; i < n; i++ {
			s.touch(s.resArm().look[i])
			s.objs[s.resArm().look[i]].zone = kop.ZoneNone
		}
		pl.libLo += uint8(n)
		for i := 0; i < n; i++ {
			if s.g.lookSel&(1<<i) != 0 {
				s.digTake(k, s.resArm().look[i], z, fl)
			}
		}
	}
	if park && s.parkTail(k, tailDig) {
		return true
	}
	s.finishDigTail(k)
	return false
}

// digAuraHost is the one permanent a taken Aura o enters enchanting when it
// has exactly one legal host (CR 303.4f); NoSlot for any other card, and for
// two or more hosts, whose controller's choice the dig does not pose yet.
func (s *State) digAuraHost(k *prep, o Obj, p uint8) Slot {
	if s.g.pre.obj == o && s.g.pre.kind == akEnchant && s.g.pre.state == preDigEnchantDone {
		return Slot(s.g.pre.val) // the answered choice among two or more hosts
	}
	c := s.objs[o].card
	if k.gen == nil || k.cards[c].types&kop.TAura == 0 || k.t.Cards[c].Enchant.Kind != kop.TgtObject {
		return NoSlot
	}
	var buf [MaxOptions]uint8
	if s.enchantCands(k, c, p, &buf) != 1 {
		return NoSlot
	}
	return s.objs[buf[0]].slot
}

// digAuraPark poses the enchant election of a taken Aura with two or more
// legal hosts (gorge's auraEntryGate asks before the move folds): the card waits
// in the library with the rest, and the answer runs the dig again from it.
//
//go:noinline
func (s *State) digAuraPark(k *prep, o Obj, p uint8) bool {
	c := s.objs[o].card
	if k.gen == nil || !s.res.active || s.g.pre.state != 0 || s.res.ask != raNone ||
		k.cards[c].types&kop.TAura == 0 || k.t.Cards[c].Enchant.Kind != kop.TgtObject {
		return false
	}
	var buf [MaxOptions]uint8
	if s.enchantCands(k, c, p, &buf) < 2 {
		return false
	}
	s.g.pre = preAsk{obj: o, kind: akEnchant, state: preDigEnchant}
	s.res.ask, s.res.askP = raGenRetChoice, p
	return true
}

// digAuraStays reports whether taken card o is an Aura with no legal host
// (CR 303.4g), which cannot enter.
func (s *State) digAuraStays(k *prep, o Obj, p uint8) bool {
	c := s.objs[o].card
	if k.gen == nil || k.cards[c].types&kop.TAura == 0 || k.t.Cards[c].Enchant.Kind != kop.TgtObject {
		return false
	}
	var buf [MaxOptions]uint8
	return s.enchantCands(k, c, p, &buf) == 0
}

// digRestAside takes the n looked-at cards a battlefield dig did not take
// out of the library window, ahead of finishDigTail placing them.
func (s *State) digRestAside(n int) {
	pl := &s.pl[s.g.pickP]
	for i := 0; i < n; i++ {
		if s.g.lookSel&(1<<i) == 0 {
			s.touch(s.resArm().look[i])
			s.objs[s.resArm().look[i]].zone = kop.ZoneNone
			pl.libLo++
		}
	}
}

// digRestOut takes the looked-at cards a dig did not take out of the library
// by identity, for a dig whose taken cards were already placed (possibly back
// on top of the library).
func (s *State) digRestOut(n int) {
	pl := &s.pl[s.g.pickP]
	for i := 0; i < n; i++ {
		o := s.resArm().look[i]
		if s.g.lookSel&(1<<i) == 0 && s.objs[o].zone == kop.ZoneLibrary {
			s.touch(o)
			pl.libRemove(o)
			s.objs[o].zone = kop.ZoneNone
		}
	}
}

// --- choose each type (Liliana) ---

func (s *State) startChooseEach(k *prep, in *kop.Instr) bool {
	s.g.nchosen = 0
	first := s.eachPlayers(k, in.Obj).at(0)
	if first == 255 {
		return true
	}
	s.g.pickP = first
	s.g.pickZ = 0
	if s.nextChooseEach(k) {
		return false
	}
	return true
}

// nextChooseEach poses the next type with candidates; false when done.
func (s *State) nextChooseEach(k *prep) bool {
	for ; int(s.g.pickZ) < len(chooseEachTypes); s.g.pickZ++ {
		t := chooseEachTypes[s.g.pickZ]
		for m := s.ctrlM[s.g.pickP]; m.any(); m = m.rest() {
			if s.typesGen(k, m.first())&t != 0 {
				s.res.ask, s.res.askP = raGenChooseEach, s.g.pickP
				return true
			}
		}
	}
	return false
}

// cardCands lists (into d, or just counts) the objects matching f in its
// zone (battlefield for ZoneNone).
func (s *State) cardCands(k *prep, f kop.FilterID, p uint8, d *Decision) int {
	n := 0
	z := k.t.Filters[f].Zone
	add := func(o Obj) {
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
	if z == kop.ZoneNone || z == kop.ZoneBattlefield {
		// A creature that crewed or saddled the resolving source this turn
		// (F2CrewedSrc: Fortune, Loyal Steed's "up to one creature that
		// saddled it") is relative to the source, which no other pick reads.
		src := NoSlot
		if chainFlags2(k.t, f)&kop.F2CrewedSrc != 0 {
			src = s.objRef(k, kop.RefSelf) // filter_combatwords.go
		}
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if s.matchPermGen(k, f, sl, p, src, layerFull) {
				add(s.perms[sl].obj)
			}
		}
		return n
	}
	if z == kop.ZoneLibrary {
		return s.libWindowCands(k, f, p, d) // zone_optionalmoves.go
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		pl := &s.pl[pp]
		var list []Obj
		switch z {
		case kop.ZoneExile:
			list = pl.exile[:pl.exileN]
		case kop.ZoneGraveyard:
			list = pl.grave[:pl.graveN]
		case kop.ZoneHand:
			list = pl.hand[:pl.handN]
		}
		for _, o := range list {
			if s.matchCardGen(k, f, s.objs[o].card, pp, p, o) {
				add(o)
			}
		}
	}
	return n
}

// paySubCounterAny settles a resolution PaySubCounter whose kind is
// CtrAll (RemoveAnyCounter<N/Any/CARDNAME>): one kind unasked, several
// kinds asked one unit at a time (gorge trigger_cost_subcounter.go
// subCounterKindAsk).
func (s *State) paySubCounterAny(k *prep, sl Slot) {
	if sl == NoSlot {
		s.payDone(k, false)
		return
	}
	pp := &s.perms[sl]
	if permCtrN(s, pp, int(kop.CtrAll)) < int(s.g.pickLeft) {
		s.payDone(k, false)
		return
	}
	first, n := 0, 0
	for c := 0; c < int(kop.NumCounterKinds3); c++ {
		if kop.IsCounterKind(c) && pp.ctrN(s, c) > 0 {
			if n == 0 {
				first = c
			}
			n++
		}
	}
	if n == 0 {
		s.payDone(k, false)
		return
	}
	if n == 1 {
		s.touchSl(sl)
		pp.seeCtr(s)
		pp.subCtrN(s, first, int(s.g.pickLeft))
		s.g.pickLeft = 0
		s.payDone(k, true)
		return
	}
	s.g.pickZ = kop.PaySubCounter
	s.res.ask = raGenPayPick
}

// applyPaySubCounterPick removes one unit of the chosen kind and asks again
// while several kinds remain.
func (s *State) applyPaySubCounterPick(k *prep, a Action) {
	sl := s.resSrcSlot(k)
	kind := int(a.Mode())
	if sl == NoSlot || s.perms[sl].ctrN(s, kind) == 0 {
		s.payDone(k, false)
		return
	}
	pp := &s.perms[sl]
	s.touchSl(sl)
	pp.seeCtr(s)
	pp.subCtrN(s, kind, 1)
	s.g.pickLeft--
	if s.g.pickLeft == 0 {
		s.payDone(k, true)
		return
	}
	s.paySubCounterAny(k, sl)
}

// resSrcSlot is the resolving entry's source permanent's slot (NoSlot when
// it is not on the battlefield).
func (s *State) resSrcSlot(k *prep) Slot {
	return s.objRef(k, kop.RefSelf)
}

// digTake moves a taken (looked-at, now zoneless) card to the dig's zone.
func (s *State) digTake(k *prep, o Obj, z kop.Zone, fl int16) {
	p := s.g.pickP
	pl := &s.pl[p]
	if fl&kop.DigRemember != 0 {
		s.remember(o)
	}
	if s.digImprintMarked(k) {
		s.imprint(o) // Dig Imprint$ (zone_digimprint.go)
	}
	switch {
	case z == kop.ZoneBattlefield:
		host := s.digAuraHost(k, o, p)
		s.enterOnto(host)
		sl := s.enterMoved(k, o, p, fl&kop.DigTapped != 0, kop.ZoneLibrary)
		s.attachedTo(k, sl, host) // CR 303.4f: an Aura put onto the battlefield enters attached
	case z == kop.ZoneLibrary:
		if p := &s.g.pre; p.kind == akCmdZone && p.obj == o && p.state == preCmdZoneOK {
			*p = preAsk{} // the answered CR 903.9b move lands here, not through placeFromK
		}
		s.objs[o].zone = kop.ZoneLibrary
		s.objs[o].ver++
		if fl&kop.DigTakeToTop != 0 {
			pl.libTop(o)
		} else {
			pl.libBottom(o)
		}
	default:
		s.placeFromK(k, o, z, false, kop.ZoneLibrary, false)
		if fl&kop.DigFaceDown != 0 && z == kop.ZoneExile {
			exiler := s.res.e.ctrl
			if fl&kop.DigExilerOther != 0 {
				exiler = s.targetedExiler(exiler) // DefinedExiler$ Targeted: the targeted player exiled it
			}
			s.faceDownExile(o, s.res.e.obj, exiler) // zone_exilefacedown.go
		}
	}
}

// branchPayable: branch b of the ChooseBranch being resolved does not start
// with an unless / if-you-do cost p can't pay (gorge offers only the
// choices whose UnlessCost is payable: genericChoiceAvailable).
func (s *State) branchPayable(k *prep, b uint8, p uint8) bool {
	prog := s.prog(k)
	if prog[s.res.pc].C == 1 {
		return true // the controller's KModes: gorge offers every choice (kgen genericChoiceYou)
	}
	for i := int(s.res.pc) + 1; i < len(prog); i++ {
		in := &prog[i]
		if in.Op != kop.OpBranch || uint8(in.C) != b {
			continue
		}
		if i+1 >= len(prog) {
			return true
		}
		n := &prog[i+1]
		if n.Op != kop.OpUnlessPaid && n.Op != kop.OpIfPaid && n.Op != kop.OpUnlessSwitched {
			return true
		}
		return s.canPayGen(k, p, unlessPayKind(int(n.C&0xF), n), s.value(k, n.A), n.B.N, n.F)
	}
	return true
}

// pileMove moves a still-revealed library card o of a two-pile split to
// zone z.
func (s *State) pileMove(o Obj, z kop.Zone) {
	if s.objs[o].zone != kop.ZoneLibrary {
		return
	}
	from := s.detach(o)
	s.placeFrom(o, z, false, from, false)
}

// targetedExiler is DefinedExiler$ Targeted: the resolving entry's first
// player target (gorge's Targeted reads the targets' players in order). At
// two seats it is the controller's opponent, the one player it can target.
func (s *State) targetedExiler(ctrl uint8) uint8 {
	if MaxSeats == 2 {
		return NextSeat(ctrl)
	}
	for i := range s.res.e.tgt {
		if t := s.tgtCode(i); isPlayerTarget(t) {
			return CodeSeat(t)
		}
	}
	return NextSeat(ctrl)
}
