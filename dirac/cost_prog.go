package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The cost-program walker. An ability whose Ability.CostProg is n pays
// Table.CostProgs[n] atom by atom (kop.CostAtom), each atom through the
// single-atom payer the flag-driven path (payAbilityCosts) already uses, and
// in gorge's payment order whatever order the program lists them in
// (engine/rules/cast_commit.go payCast's ability arm, cast_asks.go).
//
// What the program does not do: the mana. The build's mana stage pays it, and
// canActivate prices it from Cost.Mana, so the Mana atom only marks that the
// cost has a mana part. The pick stage still asks the cards and permanents a
// Discard, TapType, Sac-another or graveyard-exile atom takes (its asks read
// Cost's flags); the walker pays the picks the stage collected, one kind at a
// time. A table with no CostProgs never reaches any of this: useCostProg stays
// unset and no ability carries a CostProg.

// useCostProg: the table has cost programs.
var useCostProg = newGenUse(func(t *kop.Table) bool { return len(t.CostProgs) != 0 })

// costProgTrace, when set (tests only), sees each atom the walker pays.
var costProgTrace func(kop.CostAtomKind)

// costAtomRank is a kind's place in gorge's payment order, 1-based; 0 is a
// kind the walker never pays. The order is payCast's ability arm: the tapped
// permanents (paid at their ask, before the payment), the mana, the life, {T},
// the discards, the exiles, the mill, the draws, the return, the reveal, {Q},
// the counters, the exert, and the sacrifice last (a sacrificed source must
// still be there for everything before it).
var costAtomRank = [256]uint8{
	kop.CostAtomTapType:       1,
	kop.CostAtomMana:          2,
	kop.CostAtomPayLife:       3,
	kop.CostAtomTap:           4,
	kop.CostAtomDiscard:       5,
	kop.CostAtomExileFromZone: 6,
	kop.CostAtomMill:          7,
	kop.CostAtomDraw:          8,
	kop.CostAtomReturn:        9,
	kop.CostAtomReveal:        10,
	kop.CostAtomUntap:         11,
	kop.CostAtomSubCounter:    12,
	kop.CostAtomAddCounter:    13,
	kop.CostAtomExert:         14,
	kop.CostAtomSac:           15,
}

const costAtomMaxRank = 15

// costProgOrderInto appends to buf the indexes of prog's atoms in payment
// order (a stable sort by rank; an atom with no rank is left out).
func costProgOrderInto(buf []int, prog []kop.CostAtom) []int {
	for r := uint8(1); r <= costAtomMaxRank; r++ {
		for i := range prog {
			if costAtomRank[prog[i].K] == r {
				buf = append(buf, i)
			}
		}
	}
	return buf
}

// costProgOrder is costProgOrderInto on a fresh slice.
func costProgOrder(prog []kop.CostAtom) []int { return costProgOrderInto(nil, prog) }

// costProgOf is the program of ability a, or nil.
func costProgOf(k *prep, a *kop.Ability) []kop.CostAtom {
	if a.CostProg == 0 || int(a.CostProg) >= len(k.t.CostProgs) {
		return nil
	}
	return k.t.CostProgs[a.CostProg]
}

// payCostProg pays ability a's program for the source at sl (NoSlot: the
// source is not on the battlefield). A source atom after one that moved the
// source away (Return, Exile, Sac) does nothing, as payAbilityCosts leaves the
// rest of the cost unpaid once the source has left.
func (s *State) payCostProg(k *prep, e *stackEntry, sl Slot, a *kop.Ability) {
	prog := costProgOf(k, a)
	var buf [16]int
	var src Obj = NoObj
	if sl != NoSlot {
		s.touchSl(sl)
		src = s.perms[sl].obj
	}
	for _, i := range costProgOrderInto(buf[:0], prog) {
		at := &prog[i]
		if costProgTrace != nil {
			costProgTrace(at.K)
		}
		s.payCostAtom(k, e, sl, src, at)
	}
}

// srcLive: the source is still the permanent at sl.
func (s *State) srcLive(sl Slot, src Obj) bool {
	return sl != NoSlot && src != NoObj && s.objs[src].zone == kop.ZoneBattlefield && s.objs[src].slot == sl
}

// payCostAtom pays one atom.
func (s *State) payCostAtom(k *prep, e *stackEntry, sl Slot, src Obj, at *kop.CostAtom) {
	n := s.evalV(k, at.N, evalCtx{e: e, src: src, srcSl: sl, ctrl: e.ctrl})
	live := s.srcLive(sl, src)
	switch at.K {
	case kop.CostAtomMana, kop.CostAtomReveal:
		// the build's mana stage; a self-reveal moves nothing (cast_asks.go revealCostAsk)
	case kop.CostAtomTap:
		if live {
			s.tapSelfCost(k, sl)
		}
	case kop.CostAtomUntap:
		if live {
			s.untapCost(k, sl)
		}
	case kop.CostAtomSac:
		if at.F != 0 {
			s.payPickKind(k, pkSac) // paid later by paySacsGen on a generated table
		} else if live {
			s.sacSelfCost(k, sl)
		}
	case kop.CostAtomExileFromZone:
		if at.F == 0 && (at.Z == kop.ZoneBattlefield || at.Z == kop.ZoneNone) {
			if live {
				s.exileSelfCost(k, sl)
			}
		} else {
			s.payExilePicks()
		}
	case kop.CostAtomDiscard:
		s.payPickKind(k, pkDiscard)
	case kop.CostAtomTapType:
		s.payPickKind(k, pkTapX)
	case kop.CostAtomPayLife:
		s.loseLife(k, e.ctrl, n)
	case kop.CostAtomSubCounter:
		if live {
			s.perms[sl].seeCtr(s)
			s.subCtrFire(k, sl, int(at.Ctr), n) // trig_counterremoved.go
		}
	case kop.CostAtomAddCounter:
		if live {
			s.addCounterCost(k, sl, int(at.Ctr), n, e.ctrl)
		}
	case kop.CostAtomMill:
		s.millCostPay(k, e.ctrl, n)
	case kop.CostAtomDraw:
		s.payDraw(e.ctrl, n)
	case kop.CostAtomReturn:
		if live && !(k.gen != nil && s.deferSacGen(k, sl)) {
			s.leave(k, sl, kop.ZoneHand, false)
		}
	case kop.CostAtomExert:
		if live {
			s.exertCost(sl)
		}
	}
}

// canPayCostProg reports whether player p can pay ability a's program for the
// source at sl: every atom is payable, each by the gate the flag-driven offer
// (canActivate) applies to the same part. An atom kind with no kernel payer
// (Behold, ExiledMoveToGrave) is never payable.
func (s *State) canPayCostProg(k *prep, p uint8, sl Slot, a *kop.Ability) bool {
	prog := costProgOf(k, a)
	src := s.perms[sl].obj
	tapped := false
	for i := range prog {
		tapped = tapped || prog[i].K == kop.CostAtomTap
	}
	for i := range prog {
		at := &prog[i]
		n := s.evalV(k, at.N, evalCtx{src: src, srcSl: sl, ctrl: p})
		if !s.canPayAtom(k, p, sl, src, at, n, tapped, NoObj, true) {
			return false
		}
	}
	return true
}

// offerCostProg is canPayCostProg for a spell's cast: the source is a card
// (cast from hand, or another zone), not a permanent, so an atom that works on
// a permanent source is unpayable and a discard leaves the cast card out. The
// cast offer keeps the AkCast mode-bit scheme; this is the predicate its
// additional-cost gate reads for a card whose spell ability carries a program.
func (s *State) offerCostProg(k *prep, p uint8, src Obj, a *kop.Ability) bool {
	prog := costProgOf(k, a)
	for i := range prog {
		at := &prog[i]
		n := s.evalV(k, at.N, evalCtx{src: src, srcSl: NoSlot, ctrl: p})
		if !s.canPayAtom(k, p, NoSlot, src, at, n, false, src, false) {
			return false
		}
	}
	return true
}

// canPayAtom: one atom is payable. onBF: the source is the permanent at sl.
// exclude is the cast card a discard leaves out.
func (s *State) canPayAtom(k *prep, p uint8, sl Slot, src Obj, at *kop.CostAtom, n int, tapsSource bool, exclude Obj, onBF bool) bool {
	switch at.K {
	case kop.CostAtomMana, kop.CostAtomDraw, kop.CostAtomMill:
		return true // the mana is priced from Cost.Mana; a draw or a mill from an empty library is legal
	case kop.CostAtomTap:
		return onBF && s.perms[sl].flags&pfTapped == 0 && !s.activationSick(k, sl)
	case kop.CostAtomUntap:
		return onBF && s.untapCostOK(k, sl)
	case kop.CostAtomSac:
		if at.F == 0 {
			return onBF && !s.sacBlocked(k, sl)
		}
		return s.countCostSac(k, p, &kop.Cost{Filter: at.F}, sl) >= n
	case kop.CostAtomExileFromZone:
		switch {
		case at.F == 0 && (at.Z == kop.ZoneBattlefield || at.Z == kop.ZoneNone):
			return onBF
		case at.Z == kop.ZoneHand:
			return int(s.pl[p].handN) >= n
		}
		return s.costObjPayable(k, p, &kop.Cost{Flags: kop.CostExileGrave, N: uint8(n)}, src, exclude)
	case kop.CostAtomDiscard:
		return s.discardPayable(k, p, &kop.Cost{Flags: kop.CostDiscard, N: uint8(n), Filter: at.F}, exclude)
	case kop.CostAtomTapType:
		have := s.countUntapped(k, p, at.F, sl)
		if tapsSource && s.perms[sl].flags&pfTapped == 0 && s.matchPerm(k, at.F, sl, s.perms[sl].ctrl, sl) {
			have-- // the source's own {T} is not one of the creatures
		}
		return have >= n
	case kop.CostAtomPayLife:
		return int(s.pl[p].life) >= n
	case kop.CostAtomSubCounter:
		return onBF && s.perms[sl].ctrN(s, int(at.Ctr)) >= n
	case kop.CostAtomAddCounter, kop.CostAtomExert, kop.CostAtomReturn:
		return onBF
	case kop.CostAtomReveal:
		return s.costTokenPartsPayable(k, p, src, &kop.Cost{RevealSelf: true})
	}
	return false // Behold, ExiledMoveToGrave: no kernel payer yet
}

// tapSelfCost pays {T}: the source taps. Only a tap that happens fires the
// Taps triggers and marks (payAbilityCosts' own {T}).
func (s *State) tapSelfCost(k *prep, sl Slot) {
	was := s.perms[sl].flags & pfTapped
	s.perms[sl].flags |= pfTapped
	if was == 0 {
		if k.gen != nil && s.subs[kop.TrigTaps].any() {
			s.fireTaps(k, sl)
		}
		s.markTapped(k, sl) // only a tap that happens marks
	}
}

// exertCost pays Exert<1/CARDNAME> (CR 702.100a): the one-shot no-untap mark,
// consumed at the controller's next untap step (turn.go's pfSkipUntap).
func (s *State) exertCost(sl Slot) {
	s.touchSl(sl)
	s.perms[sl].flags |= pfSkipUntap
}

// addCounterCost puts n counters of kind c on the source at sl as a cost
// (CR 609.1: a cost's counters are no effect -- Doubling Season's EffectOnly$
// doubler never doubles them, whatever the stack holds (gorge; Forge
// CostPutCounter effect=false, XMage PayLoyaltyCost); every other AddCounter
// replacement does, the payer putting it: replCostCtr, repl_addcounter.go).
func (s *State) addCounterCost(k *prep, sl Slot, c, n int, payer uint8) {
	placed := s.placeCounters(k, sl, c, n, payer, ctrPutCost)
	if placed > 0 && k.anyCtrAdded {
		s.ctrNoteBy(k, sl, c, payer) // a cost: gorge pay_engine publishes the payer
	}
	if placed > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		// "whenever you put one or more counters on" fires for the cost's
		// placement too: the [+1] of a loyalty ability (CR 606.4, 122.6)
		s.g.lastCK = kop.CounterKind(c)
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(placed))
	}
}

// sacSelfCost pays sacrifice-this. On a generated table it waits for the mana
// (deferSacGen, paySacsGen): a source that could tap for mana pays it first.
func (s *State) sacSelfCost(k *prep, sl Slot) {
	if k.gen != nil {
		s.bld().e.li = s.snapLKI(s.perms[sl].obj) // last known information (RefSacrificed)
		if s.deferSacGen(k, sl) {
			return // paySacsGen, after the mana
		}
	}
	s.fireSac(k, sl)
	if !s.parkCost(k, s.perms[sl].obj, parkLeave, 0) {
		s.leave(k, sl, kop.ZoneGraveyard, false)
	}
}
