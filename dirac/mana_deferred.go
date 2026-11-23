package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Deferred mana sources (Pyromancer's Goggles, kop.Ability.ManaDeferred:
// "{T}: Add {R}. When that mana is spent to cast a red instant or sorcery
// spell, copy that spell ..."). gorge's payment planner defers a mana
// ability carrying TriggersWhenSpent$ (pay.PlanHasSpecialProductionParam), so
// its census never lists it: autopay never taps it (pcard.manaAb stays -1)
// and only a potential play's prefix script, or the manual mana window, does.
// The census has a gap there, relaxable (gorge paymentPlanRelaxedAlternatives
// gives the source a free {R} alternative), so the kernel treats it as a
// scripted-prefix gap of the restricted kind (scriptRestr returns it with
// them: gapMask gfRestrScript|gfDeferred) whose mana floats in the plain pool
// and not in State.rb.
//
// The mana is an artifact's, typed (artTag). A cast that spends the floating
// {R} is paid by the deferred source (gorge ManaSpentSources): the rider reads
// that through the observed-source record of the cast's build (obsSl,
// Tf2ManaFromSelf; mana_spendtrigger.go). genState.defSrc holds the source's
// slot + 1 while its {R} floats; pay compares the pool's {R} across the payment.

// dryDeferPriced is the dry census walk's step over a gap source windowPick
// gap-listed through the deferred arm (windowGap's gfDeferred): the walk
// prices the activation's own production the way gorge's window census prices
// the unit (castWindowUnits prices a deferred ability's fixed one-colour
// production; a Tender's, restricted or Combo Any source's, or an
// AnyDifferent source's gap stays with the caller's probe, as does a chosen,
// dynamic, typed, snow, identity or reflected view). The TriggersWhenSpent
// rider does not change what the window can pay: it fires on spending, after
// the payment's reach is fixed, so the census reads the plain mana alone
// (CR 601.2g). The real walk floats the same mana (floatDeferred below), so
// the census neither understates nor overstates what the window pays.
func (s *State) dryDeferPriced(k *prep, sl Slot, pc *pcard) bool {
	c := s.cardAt(sl)
	if k.gen == nil || !k.gen.has(c, gfDeferred) || k.gen.flags[c]&(gfEachCol|gfRestr) != 0 ||
		k.gen.anyComboAny && k.isComboAny(c) {
		return false
	}
	return pc.manaN > 0 && pc.manaCol != 0 && pc.manaCol != kop.ColAny &&
		pc.manaCol&(pc.manaCol-1) == 0 &&
		!pc.manaChosen && !pc.manaFixedChosen && !pc.manaDyn && !pc.manaOne &&
		!pc.manaTyped && !pc.manaSnow && !pc.manaIdent && !pc.manaRefl && pc.blgFirst == 0
}

// deferCol is the colour (a kop.W..kop.G index) of card c's deferred mana
// ability: Pyromancer's Goggles' {R}, Gilanra's {G}. Every "{R}" above is this.
func (k *prep) deferCol(c CardID) int {
	for i := range k.t.Cards[c].Abilities {
		if a := &k.t.Cards[c].Abilities[i]; a.Kind == kop.AbMana && a.ManaDeferred {
			return bits.TrailingZeros8(uint8(a.ManaColors))
		}
	}
	return kop.R
}

// deferAb is card c's deferred mana ability, nil when it has none.
func (k *prep) deferAb(c CardID) *kop.Ability {
	for i := range k.t.Cards[c].Abilities {
		if a := &k.t.Cards[c].Abilities[i]; a.Kind == kop.AbMana && a.ManaDeferred {
			return a
		}
	}
	return nil
}

// deferLifeOK: the deferred source at sl can pay its life cost ("{T}, Pay 2
// life": Boseiju) for p; a source with no life cost always can. A life cost is
// payable at the life total or less (CR 119.4).
func (s *State) deferLifeOK(k *prep, p uint8, sl Slot) bool {
	a := k.deferAb(s.cardAt(sl))
	return a == nil || a.Cost.Flags&kop.CostPayLife == 0 || int(s.pl[SeatIx(p)].life) >= lifeCostOwed(&a.Cost)
}

// deferLifeSurvives: the deferred source at sl can pay its life cost and leave
// p above 0 life. The scripted-prefix search and the X-ask census use this
// stricter rule (gorge's prefix search drops a branch that ends the game,
// scriptSearch.visit; CR 119.4, 104.3b, as lifeGapSurvives does for a life
// gap), while the mana window keeps deferLifeOK (a payment to 0 is payable).
func (s *State) deferLifeSurvives(k *prep, p uint8, sl Slot) bool {
	a := k.deferAb(s.cardAt(sl))
	return a == nil || a.Cost.Flags&kop.CostPayLife == 0 || int(s.pl[SeatIx(p)].life) > lifeCostOwed(&a.Cost)
}

// floatDeferred adds the deferred source sl's mana to p's pool and remembers
// the source and its colour (tapGap's deferred arm). A second deferred source
// floating beside the first would leave the first's spend unattributed: stall
// loudly.
func (s *State) floatDeferred(k *prep, p uint8, sl Slot) { s.floatDeferredAs(k, p, sl, -1) }

// floatDeferredAs is floatDeferred for colour col (-1: the lowest of deferMask).
func (s *State) floatDeferredAs(k *prep, p uint8, sl Slot, col int) {
	if col < 0 {
		col = bits.TrailingZeros8(s.deferMask(k, sl)) // the lowest colour (the fixed one when the source has just one)
	}
	if col >= kop.NumManaKinds {
		return // an identity source with no commander identity makes no mana (CR 903.4f)
	}
	if d := s.g.defSrc[SeatIx(p)]; d != 0 && d != uint8(sl)+1 && s.pl[SeatIx(p)].pool[s.defColOf(p)] != 0 {
		s.capFull(StallObsSrc)
	}
	s.pl[SeatIx(p)].pool[col]++
	s.artTag(k, p, sl, col)
	s.g.defSrc[SeatIx(p)] = uint8(sl) + 1
	s.setDefCol(p, col)
}

// deferredPool is p's floating deferred mana before a payment, 0 when none,
// the source's slot + 1 and its colour (the payment's pool spend forgets both,
// deferSync, so deferredSpent is handed them).
func (s *State) deferredPool(k *prep, p uint8) (r uint8, src uint8, col int) {
	if k.gen == nil || !k.gen.anyDeferred || s.g.defSrc[SeatIx(p)] == 0 {
		return 0, 0, 0
	}
	col = s.defColOf(p)
	return s.pl[SeatIx(p)].pool[col], s.g.defSrc[SeatIx(p)], col
}

// deferSnap is the deferred colour's floating count before a pool spend (col is
// meaningless when n is 0): deferSync compares it afterwards.
func (s *State) deferSnap(p uint8) (col int, n uint8) {
	if s.g.defSrc[SeatIx(p)] == 0 {
		return 0, 0
	}
	col = s.defColOf(p)
	return col, s.pl[SeatIx(p)].pool[col]
}

// deferSync forgets the deferred source when a pool spend took some of its
// colour (gorge spends the source's batch first, so its unit is gone even when
// same-colour mana from other sources still floats). A creature source such as
// Gilanra sets no poolArt bit, so artSync's gate never sees its spend; every
// live spend of floating mana takes the snapshot: payFromPool,
// payFromPoolAnyColor (and payConvPips under it), payFromPoolRider, filtDo's
// fee and pip, eachProduce's fee. The other pool decrements spend nothing: they
// take back mana a source just added (a restricted or parked unit, a fixed
// extra, a witness tap's), return units rbTake lent to their batches, or run on
// a dry probe whose pool is restored (windowDryFilter, windowFiltFirstFallback).
func (s *State) deferSync(p uint8, col int, n uint8) {
	if n != 0 && s.pl[SeatIx(p)].pool[col] < n {
		s.g.defSrc[SeatIx(p)] = 0
		s.setDefCol(p, 0)
	}
}

// deferredSpent records the deferred source on the cast being built when the
// payment spent some of its mana (r0 was the pool's before, of colour col: read
// before the payment, which clears the record).
//
//go:noinline
func (s *State) deferredSpent(k *prep, p uint8, r0, src uint8, col int) {
	if s.pl[SeatIx(p)].pool[col] >= r0 {
		return
	}
	// gorge spends the source's batch first, so its unit is gone now even when
	// same-colour mana from other sources still floats (Gilanra's {G} beside
	// Forests'): a later payment of that colour is not the source's
	s.g.defSrc[SeatIx(p)] = 0
	s.setDefCol(p, 0)
	if !s.buildLive() || s.bld().e.kind != seSpell {
		return
	}
	sl := Slot(src - 1)
	if s.bld().obsSl != 0 && s.bld().obsSl != sl+1 {
		s.capFull(StallObsSrc)
		return
	}
	s.bld().obsSl = sl + 1
	s.deferredNoCounter(k, sl)
}

// scriptDeferred: p's activatable deferred sources, a slot mask (a subset of
// scriptRestr's).
func (s *State) scriptDeferred(k *prep, p uint8) slotMask {
	if k.gen == nil || !k.gen.anyDeferred {
		return slotMask{}
	}
	return s.gapMask(k, p, gfDeferred)
}

// addDeferredUnits counts p's activatable deferred sources (Gilanra's {G}) as
// available units of av: the planner's census omits them, yet the CR 601.2g
// mana window can tap them, and gorge's announceFeasible, which prices a
// hybrid or Phyrexian pip against the sources the window can activate, counts
// their mana (CR 605.3a).
func (s *State) addDeferredUnits(k *prep, p uint8, av *avail) {
	for b := s.scriptDeferred(k, p); b.any(); b = b.rest() {
		av.addUnits(s.deferMask(k, b.first()), 1)
	}
}

// deferRider: a is the TriggersWhenSpent$ rider of the deferred source at sl,
// which onCast's listener walk skips. gorge queues the rider apart from the
// ordinary cast triggers (cast_commit.go fireManaSpentTriggers), after them and
// casualty, before cascade, so its place in a same-controller trigger-order
// ask is last.
func (s *State) deferRider(k *prep, sl Slot, a *kop.Ability) bool {
	return k.gen.anyDeferred && a.Trig.Flags2&kop.Tf2ManaFromSelf != 0 && k.gen.has(s.cardAt(sl), gfDeferred)
}

// fireDeferredRider queues the rider of the deferred source the cast's payment
// spent (build.obsSl), when it is still on the battlefield; castTrigGen gates
// the spell's colour and type and the payment (castProvOK).
//
//go:noinline
func (s *State) fireDeferredRider(k *prep, e *stackEntry) {
	if !s.buildLive() || s.bld().obsSl == 0 {
		return
	}
	sl := Slot(s.bld().obsSl - 1)
	if !s.occ.has(sl) || !k.gen.has(s.cardAt(sl), gfDeferred) {
		return
	}
	s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigSpellCast && s.deferRider(k, sl, a) }, func(l trigListener, a *kop.Ability) {
		s.castTrigGen(k, l, a, e)
	})
}

// defColOf is the colour index of the deferred mana floating in p's pool.
func (s *State) defColOf(p uint8) int {
	p = SeatIx(p)
	return int(s.g.defCol[p/2]>>(4*(p%2))) & 15
}

func (s *State) setDefCol(p uint8, col int) {
	p = SeatIx(p)
	sh := 4 * (p % 2)
	s.g.defCol[p/2] = s.g.defCol[p/2]&^(15<<sh) | uint8(col)<<sh
}
