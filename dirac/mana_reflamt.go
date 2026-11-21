package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A reflected Produce mana ability with an Amount$ (Incubation Druid's "{T}:
// Add one mana of any type that a land you control could produce. If it has a
// +1/+1 counter on it, add three mana of that type instead"). gorge's payment
// planner defers it (pay.PaymentPlanReflectedShape rejects mr.Amount.Present),
// so its census leaves the source a gap (potential_plan.go
// paymentPlanCensusOf: the blank-Produced$ stand-in is tier-deferred, the
// ability uncovered), non-relaxable (paymentPlanRelaxedAlternatives reads a
// Mana API only), and its potential fold prices the indeterminate amount
// unbounded (potential.go addIndeterminatePotentialMana). A play the census
// cannot pay is unsupported/census_incomplete and gorge's one prefix search
// activates the source through the manual window's activate options, the
// resolution evaluated for real: the amount from the Amount$ expression, the
// colour from one real ask (one mana's colour names all of it; a single
// candidate colours asks nothing). The kernel mirrors the Combo Any source's
// shape (mana_comboany.go): a census gap of its own (censusGapMana), never a
// planner unit, modelled with the Combo Any plumbing — the same
// scriptOffers/scriptPrefix routes, the scriptDFS chain and the payment
// windows — with the per-source answer semantics branched on isReflAmt: the
// ask is the reflected candidates (manaReflectedOrder, the play's announced
// colours first, gorge's scriptOrderAnswers), one node only when more than
// one candidate, and the whole evaluated amount floats in the pick's colour.
// A filter, a tap-another source, a granted mana ability or a Lion's Eye
// Diamond in the table is refused at its start with the Combo Any source's
// (StallComboAnyMix: prepComboAnyMix reads the same anyComboAny gate).

// useReflAmt: some card's census gap is a reflected Amount$ source (a use
// bit, not a genPrep field: the fixed State layout is pinned).
var useReflAmt = newGenUse(tableHasReflAmt)

func tableHasReflAmt(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if reflAmtManaAb(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// reflAmtManaAb reports whether a is a reflected Produce mana ability with a
// non-constant Amount$ (Incubation Druid): plain {T}, the reflected read over
// ManaAmong, one mana of any colour scaled by the Amount$ expression
// (AfManaDynamic; kop.ManaV). gorge's planner defers exactly it
// (pay.PaymentPlanReflectedShape rejects mr.Amount.Present).
func reflAmtManaAb(a *kop.Ability) bool {
	return a.Kind == kop.AbMana &&
		a.AFlags&(kop.AfManaReflect|kop.AfManaChoice|kop.AfManaDynamic|kop.AfLatent) == kop.AfManaReflect|kop.AfManaChoice|kop.AfManaDynamic &&
		a.ManaColors == kop.ColAny && a.ManaAmong != 0 && a.ManaProduce && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 &&
		!a.ManaDeferred && !a.ManaEach && !a.ManaIdentity && !a.ManaIdentityPotential && a.ManaRider == 0 &&
		a.SplitGate == 0 && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// isReflAmt reports whether card c's census gap (gfGapOther) is its reflected
// Amount$ ability.
func (k *prep) isReflAmt(c CardID) bool {
	if k.gen == nil || !k.gen.uses.has(useReflAmt) || !k.gen.has(c, gfGapOther) {
		return false
	}
	j := k.gen.gapOther(c)
	return j >= 0 && reflAmtManaAb(&k.t.Cards[c].Abilities[j])
}

// producesC reports whether cd's printed mana abilities make {C} (gorge
// effects.producibleSymbols' WUBRGC runes: a literal Produced$ C; a blank or
// "Any" production is the five colours, never colourless). The kernel's
// lowered colours carry the produced symbols' mask exactly (build_ab.go
// mana(): a literal "{C}" is kop.ColC, a blank or Any is kop.ColAny), so the
// printed abilities' masks say it.
func producesC(cd *kop.CardDef) bool {
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.ManaColors&kop.ColC != 0 {
			return true
		}
	}
	return false
}

// reflAmtPotential folds the reflected Amount$ source's bound at the
// per-tap pass (availPotentialFor's pc.manaAb < 0 arm) and reports whether it
// took the card. gorge's bound (rules potential.go) prices the ability — a
// blank Produced$ with an indeterminate Amount$ — unbounded in the colourless
// slot ONLY (addPotentialMana normalizes the blank Produced$ to "C" and
// addIndeterminatePotentialMana folds the indeterminate amount unbounded
// there; addPotentialReflected's shape refuses the Amount$, so the reflected
// arm never runs): the coloured pips a play needs stay bound by the lands, so
// the druid's own Adapt {3}{G}{G} beside one Forest is not offered. The
// planner census lists no unit (pay.PaymentPlanReflectedShape defers the
// amount), so the census pass records nothing and the over-bound fold keeps
// {C} — never the whole-pool open the Combo Any source's fold is.
func (s *State) reflAmtPotential(k *prep, sl Slot, pc *pcard, done *[MaxPerm]uint16, fold func(Slot, uint8, uint8)) bool {
	c := s.cardAt(sl)
	if !k.isReflAmt(c) {
		return false
	}
	cd := &k.t.Cards[c]
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; j < 16 && a.Kind == kop.AbMana && a.AFlags&kop.AfManaReflect != 0 {
			done[sl] |= 1 << j
		}
	}
	if s.canTapForManaPC(k, sl, pc) {
		fold(sl, uint8(kop.ColC), potentialOpenUnits)
	}
	return true
}

// reflAmtCols is the colour candidates the reflected Amount$ ability of sl
// offers (gorge effects.ManaReflectedCandidates over the Produce read): the
// union of the matching permanents' cards' producible colours, colourless
// included when a card's printed mana abilities make {C} (Wastes beside the
// lands; gorge's manaReflectedOrder lists C). The amount is deferred, so no
// census alternative exists; the manual ask is the candidates' own read.
func (s *State) reflAmtCols(k *prep, sl Slot) uint8 {
	c := s.cardAt(sl)
	f := k.t.Cards[c].Abilities[k.gen.gapOther(c)].ManaAmong
	p := s.perms[sl].ctrl
	var m uint8
	for b := s.ctrlM[SeatIx(p)].or(s.oppsM(p)); b.any(); b = b.rest() {
		o := b.first()
		if !s.matchPerm(k, f, o, p, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(o)]
		m |= uint8(cd.ProduceCol)
		if producesC(cd) {
			m |= uint8(kop.ColC)
		}
	}
	return m
}

// reflAmtN is the mana the reflected Amount$ ability of sl makes now (gorge
// resolves the Amount$ at the activation: effManaReflected's numText).
func (s *State) reflAmtN(k *prep, sl Slot) int {
	return s.caAmount(k, sl)
}

// reflAmtDo activates reflected Amount$ source sl for p with answer col (a
// colour index, C included): the tap, then the whole evaluated amount of that
// colour (gorge's resolution: one colour ask names all the mana).
func (s *State) reflAmtDo(k *prep, p uint8, sl Slot, col int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	n := s.reflAmtN(k, sl)
	if n > 0 {
		s.poolAdd(p, col, n)
		s.artTag(k, p, sl, col)
	}
	s.tapExtraStall(k, sl)
}

// reflAmtOrder is the ask's answers in the search's order: the candidates in
// engine order (gorge manaReflectedOrder's W U B R G C), the play's announced
// colours moved to the front (scriptOrderAnswers). out must hold 6; the
// returned slice is the answers.
func (s *State) reflAmtOrder(k *prep, sl Slot, a Action, out []uint8) []uint8 {
	m := s.reflAmtCols(k, sl)
	n := 0
	for m != 0 {
		out[n] = uint8(bits.TrailingZeros8(m))
		n++
		m &= m - 1
	}
	needFirst(out[:n], s.playNeed(k, a))
	return out[:n]
}

// reflAmtAnswers is the colour ask's answer count (one, the deterministic
// resolution, when a single candidate names no ask).
func (s *State) reflAmtAnswers(k *prep, sl Slot) uint8 {
	return uint8(max(bits.OnesCount8(s.reflAmtCols(k, sl)), 1))
}

// reflAmtChainTap taps reflected Amount$ source sl for the scriptDFS chain's
// answer c of play a: the c'th colour of the ask in scriptOrderAnswers order.
func (s *State) reflAmtChainTap(k *prep, p uint8, a Action, sl Slot, c int) {
	var order [6]uint8
	cols := s.reflAmtOrder(k, sl, a, order[:])
	if len(cols) == 0 {
		// nothing to reflect: the tap, no mana (windowReflAmtTap's empty
		// branch; the answer count floors at one, reflAmtAnswers)
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		s.tapExtraStall(k, sl)
		return
	}
	if c >= len(cols) {
		c = 0
	}
	s.reflAmtDo(k, p, sl, int(cols[c]))
}

// windowGapTap taps the pick's own census-gap source: a reflected Amount$ one
// (windowReflAmtTap, below) or a Combo Any one (windowComboTap, mana_comboany.go).
// It reports whether the card is one of them; windowTapWalk keeps hunting
// otherwise, as gorge's manual window lists every untapped source.
func (s *State) windowGapTap(k *prep, p uint8, sl Slot, c CardID, cost, useful *kop.ManaCost, clamp, noOwed bool) bool {
	switch {
	case k.isReflAmt(c):
		s.windowReflAmtTap(k, p, sl, cost, useful, clamp, noOwed)
	case k.isComboAny(c):
		s.windowComboTap(k, p, sl, cost, useful, clamp, noOwed)
	default:
		return false
	}
	return true
}

// windowReflAmtTap activates reflected Amount$ source sl in a payment
// window: the tap, then the whole evaluated amount in one colour — the one
// the cost still owes (windowOwedCost), else the lowest the clamp keeps, else
// the lowest candidate. A single candidate asks nothing (gorge's
// deterministic resolution).
func (s *State) windowReflAmtTap(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool) {
	m := s.reflAmtCols(k, sl)
	if m == 0 {
		// nothing to reflect: the activation still resolves — the tap, no mana
		// (gorge's Note)
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		s.tapExtraStall(k, sl)
		return
	}
	col := bits.TrailingZeros8(m)
	if !noOwed {
		var owed kop.ManaCost
		s.windowOwedCost(k, p, cost, &owed)
		for c := 0; c < kop.NumManaKinds; c++ {
			if m&(1<<c) != 0 && owed.Pips[c] > 0 {
				col = c
				break
			}
		}
	} else if clamp {
		if want, decided := s.windowUseful(k, p, useful); decided {
			if wm := m & want; wm != 0 {
				col = bits.TrailingZeros8(wm)
			}
		}
	}
	s.reflAmtDo(k, p, sl, col)
}

// reflBoundPlay: play a is one gorge's PotentialMana bound pays beside a live
// reflected Amount$ source (caM, the census gaps of a Combo Any kind) though
// the gaps walk's census does not: the gaps walk counts the source's mana as
// {C} alone (availCA), but another source's open production (Humble
// Naturalist's restricted one mana of any colour, admitted for a creature
// spell and priced unbounded by the bound) lifts a coloured pip the {C} could
// not. gorge's unrelaxable census then searches the play, and the search finds
// it when the source's colour answer funds the pips (Druid's three {B} and the
// Naturalist's one beside Faceless Butcher's {2}{B}{B}).
func (s *State) reflBoundPlay(k *prep, p uint8, caM slotMask, a Action) bool {
	for b := caM; b.any(); b = b.rest() {
		if k.isReflAmt(s.cardAt(b.first())) {
			return s.sacPotPays(k, p, a)
		}
	}
	return false
}

// reflBoundOrOwnTapPlay: play a is a candidate by reflBoundPlay or by the own-tap
// walk (ownTapWalkPlay), tried in that order.
func (s *State) reflBoundOrOwnTapPlay(k *prep, p uint8, caM slotMask, a Action, ownWalk *Decision, ownBuilt *bool) bool {
	return s.reflBoundPlay(k, p, caM, a) || s.ownTapWalkPlay(k, p, a, ownWalk, ownBuilt)
}
