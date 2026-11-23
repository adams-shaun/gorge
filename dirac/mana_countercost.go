package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Counter-cost mana (Wall of Roots: "Put a -0/-1 counter on Wall of
// Roots: Add {G}. Activate only once each turn."). gorge's planner census
// cannot execute an AddCounter cost (pay.PlanLastResortCostOK), so the
// source is a census gap with a fixed relaxed alternative (potential_plan.go
// paymentPlanRelaxProof: "Wall of Roots' counter"): a play the lands cannot
// pay but the relaxed census can is searched for a prefix that activates
// the gap first (potentialPrefixScript), the sources then paying the rest.
// The kernel models it as one more script gap kind beside the each-colour,
// restricted and sacrifice-only ones (script_gen.go): its mana in the gaps
// walk (availCtr), and at the play the chain's activations in battlefield
// order, a source with a plain {T} first ability taken through both branches
// its activation ask lists (the {T} tap before the counter cost), an
// any-colour ability (Vivid Grove's second) branched on its colour
// ask (ctrScript), until the walk lists the play. Each activation puts a
// -0/-1 counter (kop.CtrM0M1, read by ptGen) and spends the once-each-turn
// use (perm.actUsed). Everything is behind prep.ctrMana.
//
// The all-at-once removal shape (kop.CtrEachManaAb: Ramos, Dragon Engine's
// "Remove five +1/+1 counters from Ramos: Add {W}{W}{U}{U}{B}{B}{R}{R}{G}{G}.
// Activate only once each turn.") is the same gap: gorge relaxes it to one
// free alternative holding the whole production. Its pcard keeps the whole
// colour mask (no prepEachMana split: it is no tap source), the gaps walk
// adds every colour (not one of them), the activation removes the counters
// as the activation path's CostRemoveCtr does, and the gap is offered only
// while the source holds them (gorge manaAbilityPayablePool).

// stepCtr is the gi of a prefix step that taps a counter-cost gap; the
// step's col is the any-colour ask's answer plus one (0: a fixed colour or
// every colour at once asks nothing).
const stepCtr int16 = -9

// ctrManaAb reports whether mana ability a costs only counters put on its
// source, once each turn, with a fixed production.
func ctrManaAb(a *kop.Ability) bool { return kop.CtrManaAb(a) }

// deadCtrAb reports whether mana ability a is the dead counter-cost shape
// (Component Pouch's "{T}, Remove a component counter: Add two mana of
// different colors", kop.CtrDeadManaAb): gorge's census records the source
// as an unrelaxable, unbounded gap while the cost is payable, and the
// activation's resolution adds no mana (engine/effects/mana_effect.go's
// fail-closed rune gate). The kernel models it as a dead chain member of
// the prefix search: no planner unit, no window pick, no units in any gaps
// walk, and its single chain branch pays the cost and taps without mana.
func deadCtrAb(a *kop.Ability) bool { return kop.CtrDeadManaAb(a) }

// ctrGapAb reports whether mana ability a is one of the counter- or
// sacrifice-counter-cost shapes the kernel prices as a counter-cost script
// gap (mana_countercost.go, mana_sacctr.go). prep marks the card's gap from
// this one predicate so the census and the script walk cannot diverge.
func ctrGapAb(a *kop.Ability) bool {
	return ctrManaAb(a) || kop.StoreManaAb(a) || kop.CtrDeadManaAb(a) || sacCtrManaAb(a)
}

// deadCtrMask is p's activatable dead counter-cost sources among scriptCtr.
// While one is activatable gorge's census is incomplete and unrelaxable
// (potential_plan.go paymentPlanRelaxedAlternatives enumerates no colour for
// its production), so the prefix search runs on the any-mana walk and every
// insufficient play takes a script slot (potentialPlans counts one whatever
// the search finds).
func (s *State) deadCtrMask(k *prep, p uint8) slotMask {
	var m slotMask
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if a := k.ctrAbOf(s.cardAt(sl)); deadCtrAb(a) {
			m = m.with(sl)
		}
	}
	return m
}

// ctrAbOf is card c's counter-cost mana ability (prep.ctrAb): its first mana
// ability (Wall of Roots, Ramos), or a later {T} one beside a plain {T} first
// (Weather Maker), nil none.
func (k *prep) ctrAbOf(c CardID) *kop.Ability {
	if int(c) >= len(k.ctrAb) || k.ctrAb[c] < 0 {
		return nil
	}
	return &k.t.Cards[c].Abilities[k.ctrAb[c]]
}

// ctrSecond: card c's counter-cost ability is a later one beside a plain {T}
// first (Weather Maker), so activating the source asks which ability. A
// storage battery whose first ability is the counter one (Bottomless Vault)
// has no plain first ability to answer with (pcard.manaAb is -1: a dynamic
// amount is no planner unit), so it is not a second.
func (k *prep) ctrSecond(c CardID) bool {
	return int(c) < len(k.ctrAb) && k.ctrAb[c] >= 0 && k.cards[c].manaAb >= 0 && k.ctrAb[c] != k.cards[c].manaAb
}

// scriptCtr: p's counter-cost sources that can be activated now (a Wall of
// Roots has no {T}: summoning sickness does not matter, CR 302.6; a source
// whose cost taps it must be untapped and, a creature, not sick), a slot mask.
//
//go:noinline
func (s *State) scriptCtr(k *prep, p uint8) slotMask { return s.scriptCtrIdle(k, p, false) }

// scriptCtrIdle is scriptCtr; idle also lists a storage battery holding no
// counters (its X is 0: the {T} alone, no mana), the shape gorge's activation
// ask still offers on a grant holder (grantStoreIdleGaps).
//
//go:noinline
func (s *State) scriptCtrIdle(k *prep, p uint8, idle bool) slotMask {
	var m slotMask
	if !k.ctrMana || DebugNoCtrMana {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.pickedForMana(s.perms[sl].obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
		c := s.cardAt(sl)
		a := k.ctrAbOf(c)
		if a == nil || DebugNoDeadCtr && deadCtrAb(a) || s.perms[sl].actUsed&(1<<uint(k.ctrAb[c])) != 0 || s.genLost(k, sl) || k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		if a.Cost.Flags&kop.CostRemoveCtr != 0 && s.perms[sl].ctrN(s, int(a.Cost.Ctr)) < int(a.Cost.N) {
			continue // too few counters to remove (legal.go actCostOK's CostRemoveCtr)
		}
		if deadCtrAb(a) {
			// A dead gap's own {T}: canTapForMana reads no mana unit here (the
			// production resolves to nothing), so the activation's tap
			// eligibility is read directly (storeGap's checks).
			pm := &s.perms[sl]
			if a.Cost.Flags&kop.CostTap != 0 && (pm.flags&pfTapped != 0 || pm.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.activationSick(k, sl)) {
				continue
			}
			m = m.with(sl)
			continue
		}
		if sacCtrManaAb(a) {
			if !s.sacCtrUsable(k, sl, a) {
				continue // its live counters, its {T} and its sacrifice (mana_sacctr.go)
			}
		} else if kop.StoreManaAb(a) {
			// A storage battery: the announced X is the counters it holds, so
			// with none there is no activation. Its {T} is read here, not
			// through canTapForManaPC (whose plain unit prices one mana, not X).
			if s.perms[sl].ctrN(s, int(a.Cost.Ctr)) <= 0 && !idle {
				continue
			}
			if a.Cost.Flags&kop.CostTap != 0 && (s.perms[sl].flags&pfTapped != 0 || s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.activationSick(k, sl)) {
				continue
			}
		} else if a.Cost.Flags&kop.CostTap != 0 && !s.canTapForMana(k, sl) {
			continue // its {T}: the source is untapped and not summoning sick (the second ability beside a plain {T} first)
		}
		m = m.with(sl)
	}
	return m
}

// ctrBesideGrant reports whether p has an activatable counter-cost gap while
// a permanent of p's holds a live granted mana ability: the grant search is
// then gorge's one prefix search over both, and runs before the counter-cost
// script (a Hickory Woodlot beside Jaheira's tokens: gorge taps the granted
// gap first, battlefield order, CR 601.2g-h).
func (s *State) ctrBesideGrant(k *prep, p uint8) bool {
	m := s.scriptCtr(k, p)
	if !m.any() || !s.liveGrantGap(k, p) {
		return false
	}
	for b := m; b.any(); b = b.rest() {
		a := k.ctrAbOf(s.cardAt(b.first()))
		if !deadCtrAb(a) && (!kop.StoreManaAb(a) || k.ctrSecond(s.cardAt(b.first()))) {
			return true // the grant search lists the plain and counter-scaled sacrifice shapes (candidates)
		}
	}
	return false
}

// grantBesideTapXGaps reports whether gorge's one search beside the grants
// prices play a on a board whose tap-another script (scriptOffers) found
// nothing: a tap-another source (a second ability, a one- or two-permanent
// cost: Seton, Krosan Protector's "Tap an untapped Druid" beside a Food granted
// "{T}, Sacrifice: Add one mana of any color") or a counter-cost gap among the
// grants (mana_granttapx.go, ctrBesideGrant), the grant search listing both.
// The second return is the failed search's latched stall (grantActScript's),
// StallNone none: the caller keeps it only when no route priced the play.
func (s *State) grantBesideTapXGaps(k *prep, a Action, p uint8) (bool, uint8) {
	if !(s.grantTapXGaps(k, p).any() || s.ctrBesideGrant(k, p)) || !s.liveGrantGap(k, p) {
		return false, 0
	}
	return s.grantActScript(k, a, p)
}

// availCtr adds p's counter-cost sources' mana to the gaps walk.
//
//go:noinline
func (s *State) availCtr(k *prep, p uint8, av *avail) {
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		a := k.ctrAbOf(c)
		if deadCtrAb(a) {
			continue // a dead gap: gorge's census lists it, but it adds no mana (mana_countercost.go ctrDeadGap)
		}
		if sacCtrManaAb(a) {
			if n := s.sacCtrUnits(sl, a); n > 0 {
				av.addUnits(uint8(a.ManaColors)&0x3f, uint8(n)) // its live counters (mana_sacctr.go)
				if ctrAnyColour(a) && n > 1 {
					s.addOneColour(av, uint8(n)) // X of any ONE colour (CR 106.4): the Hall bound must not read them as independent
				}
			}
			continue
		}
		if s.g.availX&availNoStore != 0 && unlessUnpriced(a) {
			continue // an unless cost's window prices no battery nor any-colour counter source (mana_unlessnostore.go)
		}
		if kop.StoreManaAb(a) {
			n := s.perms[sl].ctrN(s, int(a.Cost.Ctr))
			if n > 255 {
				s.capFull(StallDynMana)
				n = 255
			}
			if n > 0 {
				av.addUnits(uint8(a.ManaColors)&0x3f, uint8(n))
			}
			continue
		}
		if a.Cost.Flags&kop.CostTap != 0 && k.ctrSecond(c) {
			// A later {T} ability beside a plain {T} first (Weather Maker's
			// {C}{C} beside its any-colour tap): both are one tap, and the
			// plain walk already counts the first's unit, which pays any
			// generic unit of the second's. Only the units beyond it are new
			// (CR 605.3b: one activation per untapped source). A {C} pip
			// the plain unit cannot pay is not priced by this.
			cols, n := tapXUnit(a)
			if cols == 1<<kop.C {
				av.addUnits(cols, n-1)
				continue
			}
		}
		addCtrUnits(av, a)
	}
}

// addCtrUnits adds counter-cost ability a's mana to av: every colour at once
// for the all-at-once shape (Ramos), else one unit of its fixed colour. A
// dead gap (Component Pouch) adds nothing.
func addCtrUnits(av *avail, a *kop.Ability) {
	if deadCtrAb(a) {
		return
	}
	cols, n := tapXUnit(a)
	if a.ManaEach {
		for c := 0; c < kop.NumManaKinds; c++ {
			if cols&(1<<c) != 0 {
				av.addUnits(1<<c, n) // every colour at once, not a choice of one
			}
		}
		return
	}
	av.addUnits(cols, n)
}

// windowCtrOffer: counter-cost source o is one gorge's ward or unless window
// lists and can take now (it asks no colour), and its colour adds something the
// clamp keeps. A dead gap (Component Pouch) is never picked: the planner
// defers it (pay.PaymentPlanShapeTierOf, a SubCounter cost), so gorge's bot
// answers the window without it, though its activation is offered and the
// prefix script's steps activate it (ctrDeadGap).
//
// windowCtrGapTap pays a counter-cost source the window picked in a gap.
// The once-each-turn Produced$ Any shape takes the plain {T} first mana
// ability: gorge's activation ask lists it first (owedAbilitiesFirst prices
// no colour in a counter-cost shape: AnnouncedAbilityColours is nil for
// one), so the window's answer takes it and the counters and the
// once-each-turn use stay (ctrChainTap's first branch); otherwise the
// counter, the use, the mana. A census walk's (an unless / ward window's
// probe) counter-fee shape the unless census prices (ctrCensusAb, the pick
// ctrCensusOffer made) takes the counter activation itself, the plain first
// ability's tap pricing the census's other alternative only. Reports whether
// the walk continues past a handled gap.
//
//go:noinline
func (s *State) windowCtrGapTap(k *prep, p uint8, sl Slot, census bool) bool {
	if !k.ctrMana || !s.scriptCtr(k, p).has(sl) || sacCtrManaAb(k.ctrAbOf(s.cardAt(sl))) {
		return false
	}
	if census && ctrCensusAb(k.ctrAbOf(s.cardAt(sl))) {
		s.ctrGap(k, p, sl, -1) // the census's fee shape: the counters, the use, the mana (ctrCensusOffer's pick)
		return true
	}
	if k.ctrSecond(s.cardAt(sl)) {
		s.ctrAb1(k, p, sl, -1)
	} else {
		s.ctrGap(k, p, sl, -1) // the counter, the use, the mana (mana_countercost.go)
	}
	return true
}

func (s *State) windowCtrOffer(k *prep, p uint8, o Slot, decided bool, want uint8) bool {
	if !k.ctrMana || !s.scriptCtr(k, p).has(o) {
		return false
	}
	a := k.ctrAbOf(s.cardAt(o))
	if deadCtrAb(a) || sacCtrManaAb(a) {
		return false // (a counter-scaled sacrifice source is no window pick: mana_sacctr.go)
	}
	return !decided || uint8(a.ManaColors)&want != 0
}

// unlessStoreUnpriced: slot o is a storage battery (Mercadian Bazaar's "{T},
// Remove any number of storage counters: Add {R} for each") and the walk is an
// unless window's census. Its announced X is no fixed amount, so gorge's
// unlessWindowUnits lists no unit for it and the unless offer cannot count its
// counters: with a Swamp beside a Bazaar holding counters, "sacrifice unless
// you pay {2}" posed decline only in gorge, while the walk here reached {2}
// through the battery and offered pay too. A ward's window keeps the offer.
func (s *State) unlessStoreUnpriced(k *prep, o Slot) bool {
	if s.g.turnF&turnUnlessWin == 0 {
		return false
	}
	a := k.ctrAbOf(s.cardAt(o))
	return a != nil && unlessUnpriced(a) // a == nil: a non-ctr gap mixed into censusPayGaps' otherGaps mask (Remote Farm, Vivid Grove)
}

// ctrCensusAb reports whether counter-cost ability a is one gorge's unless
// census prices (rules paid_mana_census.go unlessWindowUnits' SubCounter arm,
// pay.ManaSubCounterOnlyCost): a bare {T} plus a fixed, source-anchored
// counter removal on a fixed production of one colour that asks no colour
// (Weather Maker's "{T}, Remove two charge counters: Add {C}{C}", CR 608.2g).
// The added-counter shape (Wall of Roots) prices no counter the census counts;
// a storage battery's announced X is no fixed amount; the dead shape
// (Component Pouch) resolves to no mana; an any-colour production asks a
// colour (Vivid Grove's second ability), as the census withholds the plain
// one. A depletion rider's own shape (Remote Farm, ManaRiderDeplete) is
// riderCensusFree's, the rider arm's; this is the non-rider one.
func ctrCensusAb(a *kop.Ability) bool {
	return a != nil && !deadCtrAb(a) && !kop.StoreManaAb(a) && !ctrAnyColour(a) &&
		kop.CtrTapSecondManaAb(a) && a.ManaRider == kop.ManaRiderNone
}

// unlessCtrGaps reports whether p has an activatable counter-cost source the
// unless census prices (ctrCensusAb): windowAffordsTake's gate takes the walk
// for it (scriptCtr has already read the counters the fee removes and the
// source's tap), and windowPick answers the probe with the counter activation
// (ctrCensusOffer), so the unless / ward offer counts the source exactly when
// gorge's census lists it.
func (s *State) unlessCtrGaps(k *prep, p uint8) bool {
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		if ctrCensusAb(k.ctrAbOf(s.cardAt(b.first()))) {
			return true
		}
	}
	return false
}

// ctrCensusOffer: slot o is p's activatable counter-cost source the unless
// census prices (ctrCensusAb). The probe's windowPick answers it as a gap and
// windowCtrGapTap's census arm takes the counter activation itself (ctrGap):
// the plain first ability's tap prices the census's other alternative, never
// the fee shape's, so the probe would under-price the fee and offer decline
// only where gorge offers pay.
func (s *State) ctrCensusOffer(k *prep, p uint8, o Slot) bool {
	return k.ctrMana && s.scriptCtr(k, p).has(o) && ctrCensusAb(k.ctrAbOf(s.cardAt(o)))
}

// ctrGap activates counter-cost source sl for p: its counters, its use this
// turn, its mana into the pool (gorge's activation: the cost's
// CounterChange, then the mana). An any-colour ability (Vivid Grove's second
// one) asks its colour: col is the answer, the lowest colour when negative.
//
//go:noinline
func (s *State) ctrGap(k *prep, p uint8, sl Slot, col int) {
	s.ctrGapDo(k, p, sl, col, true)
}

// ctrGapDo is ctrGap; fire false is the grant search's fork-side branch
// (ctrTry): like the other branches' forks (eachTry) it taps plainly and
// leaves the tap's trigger passes to grantApply, so the search does not see
// a trigger of its own step pending (waitsForStack would dead-end the
// branch). The dead and storage shapes are no
// grant-search candidates (candidates, mana_grantedoffers.go); the
// counter-scaled sacrifice shape is one, and always fires.
func (s *State) ctrGapDo(k *prep, p uint8, sl Slot, col int, fire bool) {
	c := s.cardAt(sl)
	a := k.ctrAbOf(c)
	if kop.StoreManaAb(a) {
		s.storeGap(k, p, sl, a) // the storage battery's announced X (mana_countercost.go)
		return
	}
	if sacCtrManaAb(a) {
		s.sacCtrGap(k, p, sl, a, col) // mana_sacctr.go: it always fires, the search's step too
		return
	}
	if deadCtrAb(a) {
		s.ctrDeadGap(k, p, sl, a)
		return
	}
	pm := &s.perms[sl]
	if a.Cost.Flags&kop.CostRemoveCtr != 0 {
		pm.seeCtr(s) // the activation path's CostRemoveCtr (mana_window.go)
		if fire {
			s.subCtrFire(k, sl, int(a.Cost.Ctr), int(a.Cost.N))
		} else {
			// the search's fork-side probe leaves the trigger passes to grantApply
			pm.subCtrN(s, int(a.Cost.Ctr), int(a.Cost.N))
		}
	} else {
		n := s.placeCounters(k, sl, int(a.Cost.Ctr), int(a.Cost.N), p, ctrPutCost) // a cost: only the non-EffectOnly$ replacements
		if n > 0 && k.anyCtrAdded {
			s.ctrNoteBy(k, sl, int(a.Cost.Ctr), p) // a cost: gorge pay_engine publishes the payer
		}
	}
	if a.Cost.Flags&kop.CostOncePerTurn != 0 {
		pm.actUsed |= 1 << uint(k.ctrAb[c])
	}
	taps := a.Cost.Flags&kop.CostTap != 0
	if taps {
		pm.flags |= pfTapped // the second ability's {T} (Weather Maker)
		if fire {
			s.manaTapped(k, sl)
		}
	}
	s.touchSl(sl)
	cols, n := tapXUnit(a)
	if ctrAnyColour(a) {
		if col < 0 {
			col = bits.TrailingZeros8(cols)
		}
		cols = 1 << uint(col)
	}
	for c := 0; c < kop.NumManaKinds; c++ {
		if cols&(1<<c) != 0 {
			s.poolAdd(SeatIx(p), c, int(n))
			if taps {
				s.artTag(k, p, sl, c)
			}
		}
	}
	if taps {
		s.tapExtraStall(k, sl)
	}
}

// ctrDeadGap activates a dead counter-cost source (Component Pouch): the
// cost's counters removed, the {T} through the activation path, and NO mana
// -- gorge's census records the source as an unrelaxable, unbounded gap
// (potential_plan.go paymentPlanRelaxedAlternatives enumerates no colour for
// the production) and its resolution fail-closes on the unresolvable
// Produced$ with a Note and no mana (engine/effects/mana_effect.go's rune
// gate), so the step asks nothing and adds nothing. The kernel has no note
// channel here: the same shape outside the modelled gap is refused by kgen
// with the dead census reason (kgen build.go), and inside it the silent
// no-mana step is the fail-closed counterpart of gorge's note.
//
//go:noinline
func (s *State) ctrDeadGap(k *prep, p uint8, sl Slot, a *kop.Ability) {
	pm := &s.perms[sl]
	pm.seeCtr(s) // the activation path's CostRemoveCtr (mana_window.go)
	s.subCtrFire(k, sl, int(a.Cost.Ctr), int(a.Cost.N))
	if a.Cost.Flags&kop.CostTap != 0 {
		pm.flags |= pfTapped // its {T}
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
	if a.Cost.Flags&kop.CostTap != 0 {
		s.tapExtraStall(k, sl)
	}
}

// ctrAnyColour: counter-cost ability a makes one mana of a colour its
// activation asks (Vivid Grove's "{T}, remove a charge counter: add one mana
// of any color"), not a fixed colour or every colour at once.
func ctrAnyColour(a *kop.Ability) bool {
	return !a.ManaEach && a.ManaColors&(a.ManaColors-1) != 0
}

// ctrAnsCol is the colour counter-cost source sl's mana ability makes for the
// prefix search's answer ans of play a (ctrScript's path): an any-colour
// ability's ask is answered in scriptOrderAnswers' order, the play's
// announced colours first (tapXSearch.cols), 0 the first one; -1 for a fixed
// colour ability (Wall of Roots, Ramos), whose step asks nothing.
func (s *State) ctrAnsCol(k *prep, a Action, sl Slot, ans int) int {
	ab := k.ctrAbOf(s.cardAt(sl))
	if ab == nil || !ctrAnyColour(ab) {
		return -1
	}
	return int(s.ctrColOrder(k, a)[ans])
}

// ctrBranch activates the counter-cost candidate i (slot sl) by each of the
// answers its activation poses (ctrChainN): a plain {T} first ability's
// (Vivid Grove's {T}: Add {G}) when the source can still tap, then the counter
// ability's, an any-colour one by each colour in ctrNeedOrder's order, the
// play's announced colours first (the mirror of gorge's scriptOrderAnswers,
// CR 601.2g-h: the search asks each activation's ability and colour). A
// counter-cost ability whose counters, use or {T} are gone on copy t has no
// answer (scriptCtr's checks).
func (g *grantSrch) ctrBranch(t *State, i int, sl Slot) bool {
	c := t.cardAt(sl)
	a := g.k.ctrAbOf(c)
	if a == nil {
		return false
	}
	off := g.k.ctrFirstN(c)
	tap := t.canTapForMana(g.k, sl) // its {T}: untapped and, a creature, not sick
	ctrOK := t.perms[sl].actUsed&(1<<uint(g.k.ctrAb[c])) == 0 &&
		(a.Cost.Flags&kop.CostRemoveCtr == 0 || t.perms[sl].ctrN(t, int(a.Cost.Ctr)) >= int(a.Cost.N)) &&
		(a.Cost.Flags&kop.CostTap == 0 || tap)
	// The subsume run's colour asks keep only the colours the pool still
	// lacks (subsumeCols: gorge's subsumeAnswers on the ask's answers, the
	// pool read before the step's mana floats in, none kept no answer): the
	// plain first ability's five-colour ask (off > 1) and the counter
	// ability's any-colour one. An ask of one answer asks nothing.
	ord := ctrNeedOrder(g.need)
	ask1, ask2 := g.subsume && off > 1, g.subsume && ctrAnyColour(a)
	for ans := 0; ans < g.k.ctrChainN(c); ans++ {
		if ans < off && !tap || ans >= off && !ctrOK {
			continue
		}
		if ans < off && ask1 && !g.subsumeKeeps(t, ord[ans]) ||
			ans >= off && ask2 && !g.subsumeKeeps(t, ord[ans-off]) {
			continue // a covered colour pays nothing more, so the line through it is dominated
		}
		if g.ctrTry(t, i, sl, ans) {
			return true
		}
	}
	return false
}

// ctrTry takes candidate i's chain answer ans on a copy of t and continues the
// search there.
func (g *grantSrch) ctrTry(t *State, i int, sl Slot, ans int) bool {
	if g.np >= len(g.path) {
		return false
	}
	c := t.fork()
	c.ctrStepDo(g.k, g.p, sl, g.need, ans, false)
	g.path[g.np] = grantStep{i: uint8(i), col: uint8(ans + 1), gi: stepCtr, sl: sl}
	g.np++
	ok := g.next(c, sl)
	if !ok {
		g.np--
	}
	t.unfork()
	return ok
}

// ctrStepDo applies chain answer ans of counter-cost source sl as the grant
// search's step (ctrChainTap's counter-cost arm, the colour order read off the
// play's need): the plain first ability's tap, or the counters, use and mana
// and the state-based check a -0/-1 counter may kill with. fire false is the
// search's fork-side form (ctrGapDo).
func (s *State) ctrStepDo(k *prep, p uint8, sl Slot, need [kop.NumManaKinds]bool, ans int, fire bool) {
	c := s.cardAt(sl)
	off := k.ctrFirstN(c)
	col := -1 // a fixed colour asks nothing
	if ans < off {
		if off > 1 {
			col = int(ctrNeedOrder(need)[ans])
		}
		s.ctrAb1Do(k, p, sl, col, fire)
		return
	}
	if ab := k.ctrAbOf(c); ab != nil && ctrAnyColour(ab) {
		col = int(ctrNeedOrder(need)[ans-off])
	}
	s.ctrGapDo(k, p, sl, col, fire)
	s.sba(k)
}

// ctrColOrder is the colour ask's answers for play a in the prefix search's
// order: the play's announced colours first (tapXSearch.cols, needFirst).
func (s *State) ctrColOrder(k *prep, a Action) [5]uint8 {
	return ctrNeedOrder(s.playNeed(k, a))
}

// ctrNeedOrder is the colour ask's answers for a play needing the colours need:
// those first, then the rest in colour order (needFirst).
func ctrNeedOrder(need [kop.NumManaKinds]bool) [5]uint8 {
	var order [5]uint8
	for c := range order {
		order[c] = uint8(c)
	}
	needFirst(order[:], need)
	return order
}

// ctrFirstN is the answers a counter-cost source's plain {T} first mana
// ability poses in the chain (ctrChainTap): none for a card without one, one
// for a fixed colour, five (the colour ask, ctrColOrder's order) for an
// any-colour one (Weather Maker's "{T}: Add one mana of any color", the
// tapXPlainDo reading of manaCol).
func (k *prep) ctrFirstN(c CardID) int {
	if !k.ctrSecond(c) {
		return 0
	}
	if mc := k.cards[c].manaCol; mc&(mc-1) != 0 && !DebugCtrFirstAllColours {
		return 5
	}
	return 1
}

// ctrChainN is the answers a counter-cost gap of card c offers in the chain:
// the plain first ability's (ctrFirstN) then the counter ability's, an
// any-colour one's five colour answers (ctrAnsCol).
func (k *prep) ctrChainN(c CardID) int {
	n := k.ctrFirstN(c)
	if ab := k.ctrAbOf(c); ab != nil {
		n++
		if ctrAnyColour(ab) {
			n += 4
		}
	}
	return n
}

// ctrChainTap applies the prefix search's chain step for gap sl, answer ans
// of play a (ctrScript's node build, scriptPrefix's replay): a counter-cost
// source with a plain {T} first mana ability answers 0 with that ability's
// tap (gorge's activation ask lists it first: Activate Vivid Creek for mana,
// Add U), later answers its counters, use and mana (ctrGap; the SBA a -0/-1
// counter may kill with), any other gap its tap (tapGap).
func (s *State) ctrChainTap(k *prep, p uint8, a Action, sl Slot, ctr, restr slotMask, ans int, held *[kop.NumManaKinds]uint8) {
	if ctr.has(sl) {
		off := k.ctrFirstN(s.cardAt(sl)) // the ask's plain {T} first ability answers come first
		if ans < off {
			col := -1 // a fixed colour asks nothing
			if off > 1 {
				col = int(s.ctrColOrder(k, a)[ans])
			}
			s.ctrAb1(k, p, sl, col)
			return
		}
		s.ctrGap(k, p, sl, s.ctrAnsCol(k, a, sl, ans-off))
		s.sba(k) // the script returns to priority after the step: a Wall of Roots out of toughness dies before the next one
		return
	}
	if restr.has(sl) && k.gen.anyDeferred && k.deferIdent(s.cardAt(sl)) {
		s.deferChainTap(k, p, a, sl, ans) // a deferred identity source's colour ask (mana_deferident.go)
		return
	}
	s.tapGap(k, p, sl, restr, held)
}

// ctrAb1 applies the chain's ab1 branch for counter-cost source sl: the
// card's plain {T} first mana ability (ctrSecond), tapped, its mana (one of
// colour col when it asks, an any-colour one: Weather Maker; col -1 for a
// fixed colour) into the pool — the counters and the once-each-turn use stay (gorge's
// activation: the {T} cost, then the mana).
//
//go:noinline
func (s *State) ctrAb1(k *prep, p uint8, sl Slot, col int) {
	s.ctrAb1Do(k, p, sl, col, true)
}

// ctrAb1Do is ctrAb1; fire false is the grant search's fork-side branch,
// which leaves the tap's trigger passes to grantApply (ctrGapDo).
func (s *State) ctrAb1Do(k *prep, p uint8, sl Slot, col int, fire bool) {
	pm := &s.perms[sl]
	pm.flags |= pfTapped
	if fire {
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
	c := s.cardAt(sl)
	cols, n := tapXUnit(&k.t.Cards[c].Abilities[k.cards[c].manaAb])
	if col >= 0 {
		cols = 1 << uint(col)
	}
	for cc := 0; cc < kop.NumManaKinds; cc++ {
		if cols&(1<<cc) != 0 {
			s.poolAdd(SeatIx(p), cc, int(n))
			s.artTag(k, p, sl, cc)
		}
	}
	s.tapExtraStall(k, sl)
}

// ctrScript is gorge's prefix search (scriptSearch.visit) for play a over the
// chain order: p's gaps tapped in battlefield order, a counter-cost source
// with a plain {T} first mana ability branched on the two branches its
// activation ask lists (the {T} tap first, then the counter-cost one), an
// any-colour counter ability branched on its five colour answers (ctrAnsCol's
// order, every answer an activation poses branched on, in engine order),
// depth first; at each node after a tap the walk (restrOptions) is asked for
// a. It returns the number of gaps the found script taps (path holds their
// answers, 0 where the gap asks nothing) and whether a was offered at that
// node, dead there as scriptDeadEnd reads it (restricted mana floats and the
// plain walk does not price the play). Each node is rebuilt on a fresh fork
// (the scriptDFS walk), so the caller's state is untouched.
func (s *State) ctrScript(k *prep, a Action, p uint8, order []Slot, ctr, restr slotMask, path []uint8, noGrant bool) (int, bool, bool) {
	var d Decision
	depth := 0 // gaps tapped at this node: order[:depth], answers path[:depth]
	nodes := 0
	stall := uint8(0) // the first stall a node's fork latched (t.g.stalled), StallNone none
	for {
		if depth > 0 {
			if nodes++; nodes > potentialScriptNodes { // gorge's search budget: no script past it
				if stall != 0 {
					// the drop is final (scriptDeadEnd's callers drop the
					// play) and one node's fork ran into a rule gap the kernel
					// stalls by name: declare it, not "no payment" (tapXScript's
					// stall; a found path is not this, its replay latches on s)
					s.capFull(stall)
				}
				return 0, false, true
			}
			t := s.fork()
			t.g.noGrant = noGrant // (scriptDeadEnd's restrictedGrantLive count; the root's is unset)
			var held [kop.NumManaKinds]uint8
			for j := 0; j < depth; j++ {
				t.ctrChainTap(k, p, a, order[j], ctr, restr, int(path[j]), &held)
			}
			if stall == 0 {
				stall = t.g.stalled // the chain taps may have latched a stall
			}
			t.restrOptions(k, &d, p)
			if hasOption(&d, a) {
				dead := held != ([kop.NumManaKinds]uint8{}) && !t.scriptCompletes(k, p, a)
				s.unfork()
				return depth, true, dead
			}
			if stall == 0 {
				stall = t.g.stalled // the node's own options build may have latched one
			}
			s.unfork()
		}
		if depth < len(order) { // the node's first child
			path[depth] = 0
			depth++
			continue
		}
		for { // a leaf: the deepest gap with an answer left, else done
			if depth == 0 {
				if stall != 0 {
					// every line failed and one ran into a rule gap the kernel
					// stalls by name: declare it, not "no payment" (tapXScript's
					// stall; a found path is not this, its replay latches on s)
					s.capFull(stall)
				}
				return 0, false, true
			}
			j := depth - 1
			c := s.cardAt(order[j])
			nch := max(k.ctrChainN(c), 1)
			if restr.has(order[j]) && k.gen.anyDeferred && k.deferIdent(c) {
				nch = int(s.deferAnswers(k, order[j])) // the identity's colour ask (mana_deferident.go)
			}
			if int(path[j])+1 < nch {
				path[j]++
				break
			}
			depth--
		}
	}
}

// sbaCtrTough is CR 704.5f for a creature whose toughness only counters of
// kind ck lowered (-0/-1, or Contagion's -2/-1): the SBA's no-damage shortcut
// reads the printed toughness, so the counters are read here, once per pass.
//
//go:noinline
func (s *State) sbaCtrTough(k *prep, ck kop.CounterKind, die, gy *slotMask) {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if die.has(sl) || s.perms[sl].ctrN(s, int(ck)) == 0 || !s.isCreature(k, sl) {
			continue
		}
		if _, t := s.pt(k, sl); t <= 0 {
			*die = die.with(sl)
			*gy = gy.with(sl)
		}
	}
}

// DebugCtrFirstAllColours reads an any-colour first mana ability in the
// counter-cost chain as one answer floating every colour (the old hole; the
// weather-maker mutation test proves the lockstep needs the colour ask).
var DebugCtrFirstAllColours bool

// DebugNoCtrMana hides the counter-cost gaps (the wallroots mutation test
// proves the pool's plays need them).
var DebugNoCtrMana bool

// DebugNoDeadCtr hides the dead counter-cost gaps (Component Pouch): the
// dead-gap chain mutation test proves the lockstep reaches them.
var DebugNoDeadCtr bool

// storeGap activates a single-colour storage-battery mana ability (Mage-Ring
// Network and the single-colour storage lands): its {T} if any marked, every
// counter of the kind removed, and that many mana of the ability's one
// colour. A generic fee, a colour combination or an any-colour production is
// not modelled (kgen refuses those shapes with their reasons). CR 107.3,
// 118.3; gorge's non-interactive ManaCostSubCounterStage takes X at the bound
// (all the counters).
func (s *State) storeGap(k *prep, p uint8, sl Slot, a *kop.Ability) {
	s.storeGapX(k, p, sl, a, 255)
}

// storeGapX is storeGap announcing X = x counters at most (the counters it
// holds when fewer): the prefix search's X ask offers every X from the bound
// down to 0, and X = 0 pays the {T} alone (CR 107.3, 602.2b).
func (s *State) storeGapX(k *prep, p uint8, sl Slot, a *kop.Ability, x int) {
	pm := &s.perms[sl]
	n := pm.ctrN(s, int(a.Cost.Ctr))
	if n <= 0 {
		s.storeIdleTap(k, sl, a)
		return
	}
	if n > 255 {
		s.capFull(StallDynMana)
		n = 255
	}
	if x < n {
		n = x
	}
	if n <= 0 {
		s.storeIdleTap(k, sl, a)
		return
	}
	taps := a.Cost.Flags&kop.CostTap != 0
	if taps {
		if pm.flags&pfTapped != 0 {
			return
		}
		pm.flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
	pm.seeCtr(s)
	s.subCtrFire(k, sl, int(a.Cost.Ctr), n)
	col := bits.TrailingZeros8(uint8(a.ManaColors) & 0x3f)
	s.poolAdd(SeatIx(p), col, n)
	if taps {
		s.artTag(k, p, sl, col)
		s.tapExtraStall(k, sl)
	}
}

// ctrGapsOutsideGrantSearch reports whether p has an activatable counter-cost
// gap the grant search does not already cover: the grant search lists only
// grantStoreGaps (a storage battery with no plain first mana ability), gorge's
// one prefix search lists every gap, so a play the search's budget gave up on
// is dropped for those, not left to the counter-gap search that cannot price
// the grants. A ctrSecond battery (Mage-Ring Network) is priced only by the
// counter-gap search, so it still counts as outside.
func (s *State) ctrGapsOutsideGrantSearch(k *prep, p uint8) bool {
	return s.scriptCtr(k, p).andNot(s.grantStoreGaps(k, p)).any()
}
