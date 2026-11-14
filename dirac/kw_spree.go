package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Spree (CR 702.172a): a modal spell whose chosen modes each add
// an additional cost (CardDef.ModeCost). gorge rules/spree.go and
// cast_asks.go: the cast is offered on its printed cost; at the mode ask
// a mode whose own cost (the base plus that mode's, judged against every
// untapped source and the pool) cannot be paid is not offered; the chosen
// modes' costs join the total (buildCost0); a combination of individually
// payable modes that cannot be paid together is reversed at payment
// (CR 733.1, finishBuild). gorge's payment planner declines the face
// ("shape:modal_cost", featview.go plannedCastCost).

// modalFloorZero reports a Spree or Tiered card with a mode that adds
// nothing, so a cast's cheapest legal total is its printed cost (CR
// 702.172a/702.183a). Only then may a granted-source prefix search fund the cast: a
// cast whose cheapest mode still costs mana needs more than the printed cost,
// and a prefix that floats only the printed total would offer a cast no mode
// can pay.
func modalFloorZero(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	if !cd.HasModeCost {
		return false
	}
	ab := spellAb(k, c)
	if ab == 255 {
		return false
	}
	n := int(cd.Abilities[ab].NModes)
	if n > len(cd.ModeCost) {
		n = len(cd.ModeCost)
	}
	for _, mc := range cd.ModeCost[:n] {
		if mc.MV() == 0 && mc.X == 0 {
			return true
		}
	}
	return false
}

// spreeCost adds the chosen modes' costs to c.
func spreeCost(cd *kop.CardDef, mode uint8, c kop.ManaCost) kop.ManaCost {
	if DebugTieredNoModeCost {
		return c // modal_tiered.go: TestFintieredMutation
	}
	for m := 0; m < len(cd.ModeCost); m++ {
		if mode&(1<<m) != 0 {
			c = addCost(c, cd.ModeCost[m])
		}
	}
	if cd.HasEscalate {
		c = escalateCost(cd, mode, c) // kw_escalate.go
	}
	return c
}

// spreeModeAffordable: the entry being built can pay its cost with only
// mode m chosen (gorge modeCostFeasible: base plus that mode's cost, against
// its PotentialMana bound).
//
//go:noinline
func (s *State) spreeModeAffordable(k *prep, m int) bool {
	e := &s.bld().e
	if k.t.Cards[e.card].HasEscalate {
		return s.escalateModeAffordable(k, m) // kw_escalate.go
	}
	saved := e.mode
	e.mode = e.mode&^0x0f | 1<<m
	c := s.buildCost(k)
	e.mode = saved
	var av avail
	s.availPotential(k, e.ctrl, &av)
	return av.affords(&c, pipMaskOf(&c), 0)
}

// spreeAffordable: the chosen modes' total cost can be paid, by the sources
// as they stand or with some of the controller's filters activated in the
// CR 601.2g window (faceReachable): a Conduit Pylons' {1}, {T} any-colour
// ability makes the second {B} the base cost and a mode's cost owe.
//
//go:noinline
func (s *State) spreeAffordable(k *prep) bool {
	ctrl := s.bld().e.ctrl
	if s.castTapX(k) && s.windowTapXLive(k, ctrl).any() {
		// a tap-another source (Gene Pollinator) is live in the CR 601.2g
		// window gorge walks for a Spree face (the payment planner declines
		// it: "shape:modal_cost", kw_spree.go): the window asks each of its
		// taps and finishes from the pool, and payCastWindow reverses the
		// cast only if the cost is still short at the window's end (CR
		// 733.1). availFor counts tap-another units only in a script availX
		// mode, so pricing here without them reverses a cast whose window
		// could still fund it (mana_window.go castTapX).
		return true
	}
	c := s.buildCost(k)
	var av avail
	s.availSpreeWindow(k, &s.bld().e, &av)
	return s.faceReachable(k, ctrl, &av, &c, 0)
}

// availSpreeWindow is availForBuilt for the CR 601.2g window gorge's Spree
// census walks (affordableTargetCandidates), which offers every source the
// player could tap: a painful source counts whatever the life, its damage
// being an effect and not a cost (CR 106.12, 605.3: Cabal Pit at 1 life, as
// availCastWindowFor's announce gate counts it), and a deferred
// TriggersWhenSpent$ source (Gilanra) counts its mana, which the planner's
// census omits (mana_deferred.go) and the window offers.
//
//go:noinline
func (s *State) availSpreeWindow(k *prep, e *stackEntry, av *avail) {
	x := s.g.availX
	if k.painMana {
		s.g.availX |= availWinCol
	}
	s.availForBuilt(k, e, av)
	s.g.availX = x
	for b := s.scriptDeferred(k, e.ctrl); b.any(); b = b.rest() {
		av.addUnits(1<<k.deferCol(s.cardAt(b.first())), 1)
	}
}

// spreeCastWindowAbort pays the stranded Spree cost through the window and
// reverses the cast, leaving the window's taps and floating mana in place.
// A Spree cast whose chosen modes cannot be paid together reaches gorge's
// CR 601.2g mana window (the payment planner declines the face: "shape:
// modal_cost"), which offers every useful source; the player taps them, the
// mana floats, the cost is still unpayable and payCast aborts (CR 733.1,
// cast_commit.go). The per-slot target ask never reaches this point: its
// affordableTargetCandidates census filters the candidates through the same
// window and aborts a required target before any tap (kernel cast.go's
// bsTargets arm). The combined charm target ask has no census
// (cast_targets.go askCharmModeTargets): when an untapped source is still
// there the cast passes gorge's pre-target gate and pays through the
// window; with none it is reversed at that gate (charmTargetAskDead). The
// window half is finishBuild's, which is payCast and must run the window
// before reversing, as loweredWindowOK does
// (mana_pain.go). The restricted mana a source held out of
// the payment is returned to the pool, as payCastWindow does.
//
//go:noinline
func (s *State) spreeCastWindowAbort(k *prep) {
	e := &s.bld().e
	anyType := e.flags&sfAnyMana != 0
	anyColor := s.bld().bfl&bflAnyColor != 0
	cost := s.buildCost(k)
	// The window's colour ask carries no ManaPayment (see manaStep's Spree
	// arm): its owed colours are the announced hybrid/Phyrexian faces only,
	// so with none an any-colour source answers its lowest colour.
	_, held := s.windowTapUseful(k, e.ctrl, &cost, &cost, !anyType && !anyColor, anyColor, s.bld().pipN == 0, false)
	for c := range held {
		s.pl[e.ctrl].pool[c] += held[c]
	}
	s.reverseCast(k) // kw_spree.go
}

// charmTargetAskDead mirrors the gate gorge's target ask runs before it
// poses any target (cast_targets.go: a proposal whose resolved mana cost the
// floating pool cannot pay, with no untapped mana-ability source the CR
// 601.2g window could activate, can never complete; CR 733.1 reverses it).
// It applies to the combined charm target ask too -- askCharmModeTargets has
// no affordability census of its own, but this gate runs ahead of it. A
// cast whose cost a target-dependent reduction could still pay is left
// alive (gorge targetDependentCostMayPay).
//
//go:noinline
func (s *State) charmTargetAskDead(k *prep) bool {
	e := &s.bld().e
	if e.kind != seSpell {
		return false
	}
	cost := s.buildCost(k)
	if s.tapPaysEntry(k, e) {
		// gor's gate prices paymentMana, which folds the announced
		// Convoke/Improvise/Harmonize picks in; the pool pays only the remainder
		cost = s.convokeApplyPicks(k, xFolded(cost, int(e.x)))
	}
	anyColor := s.bld().bfl&bflAnyColor != 0
	if s.windowPoolPays(e.ctrl, &cost, anyColor) {
		return false
	}
	if s.rbAny(SeatIx(e.ctrl)) {
		// the floating restricted mana this cast may spend is in gorge's
		// payment pool (its AvailableFor holds the batches the spell's
		// restriction admits, CR 106.6), which this gate's payable check
		// reads like any other pool unit (legal.go castAffordsR)
		var av avail
		pl := &s.pl[SeatIx(e.ctrl)]
		for c := 0; c < kop.NumManaKinds; c++ {
			if pl.pool[c] > 0 {
				av.addUnits(1<<c, pl.pool[c])
			}
		}
		if s.restrAvail(k, e.ctrl, e.card, restrOriginOfEntry(e), &av) {
			if anyColor && av.affordsAnyColor(&cost, 0) || !anyColor && av.affords(&cost, pipMaskOf(&cost), 0) {
				return false
			}
		}
	}
	if tgtDepReduce(&k.t.Cards[e.card]) {
		return false // a matching target can still reduce the cost
	}
	return !s.hasUntappedMana(k, e.ctrl)
}

// hasUntappedMana reports whether p controls an untapped mana-ability
// source (gorge hasUntappedManaSource over pay.ManaSourceIDs). The
// restriction is unread: gorge's untapped-source gate reads every
// available mana ability, so a restricted source that will not pay the
// cost still opens the window (restrManaUntapped).
//
//go:noinline
func (s *State) hasUntappedMana(k *prep, p uint8) bool {
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped != 0 {
			continue
		}
		if s.canTapForMana(k, sl) || s.restrManaUntapped(k, sl) || k.sacOnlyMana && s.canSacOnlyNowAt(k, sl) {
			return true
		}
	}
	return false
}

// targetPaidPlanned: gorge's cast planner takes a Spree or Tiered face, an
// additional or optional cost, a convoke-like credit, a mana-spent reader or a
// target-dependent price when the cast is payable only by paying with a permanent its own announcements
// need (rules payment_plan.go targetPaidNeedsPlannedPayment). The witness at
// the printed cost sacrifices a source, no mode still has a target and a
// price the pool can pay once it has run (the lowering would float that mana
// before the cast, potentialWitnessReaches), and no witness that keeps its
// sources pays the printed cost (potentialWitnessKeeping). The cast is then
// priced at the printed cost, and the chosen modes' costs, which change it,
// take the manual CR 601.2g window with the targets already chosen.
//
//go:noinline
func (s *State) targetPaidPlanned(k *prep, p uint8, o Obj) bool {
	if !k.sacMana || !s.anySacSource(k, p) {
		return false
	}
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	cost := s.featCost(k, SeatIx(p), c, o, k.cards[c].cost)
	if cd.HasAltAdd {
		// gorge's witness floats the spell's mana with the first either-or
		// part that has mana (potentialAltAddPlan), so the lowering reaches
		// the cast with that mana pooled too (CR 601.2f)
		cost = addCost(cost, s.altAddManaPart(k, c))
	}
	t := s.fork()
	var pk planner
	pk.try = true
	t.tapForPK(k, p, &cost, 0, NoObj, false, false, &pk)
	consumes := pk.found && plannerConsumesSac(&pk)
	reaches := false
	if consumes {
		ab := spellAb(k, c)
		if ab == 255 || !cd.HasModeCost {
			// no target to lose, or an unmoded spell: reached when its targets
			// all remain after the payment and the floated pool pays the cost
			// as it is then priced: a count reduction over the sacrificed
			// permanents (Valkyrie Aerial Unit's affinity for a Treasure) has
			// dropped, so the lowering cannot reach the cast whose total
			// CR 601.2f fixed before that sacrifice (gorge witnessReachesMode
			// lists the cast only when the pool pays it)
			reaches = (ab == 255 || t.abilityTargetsFeasible(k, &cd.Abilities[ab], p, NoSlot, 0xff)) && t.FeatPoolPays(p, o, 0)
		}
		for m := 0; ab != 255 && cd.HasModeCost && m < int(cd.Abilities[ab].NModes); m++ {
			if t.abilityTargetsFeasible(k, &cd.Abilities[ab], p, NoSlot, 1<<m) {
				reaches = true
				break
			}
		}
	}
	s.unfork()
	if !consumes || reaches {
		return false
	}
	held := s.sacSourcesHeld(k, p)
	var av avail
	s.availFor(k, p, &av)
	s.ctrlM[p] = s.ctrlM[p].or(held)
	return !av.affords(&cost, pipMaskOf(&cost), 0)
}
