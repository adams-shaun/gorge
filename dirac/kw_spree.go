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

// modalFloorFree is modalFloorZero for the board p holds: a mode that adds
// nothing must also be one the cast can announce, so its targets must have a
// candidate (gorge potentialModalCastFreeMode reads modeTargetsAvailable on
// each free mode, X pending). Cloud's Limit Break with no tapped creature has
// no legal Cross-Slash ({0}); Blade Beam ({1}) still casts with no targets,
// but a cast whose cheapest legal mode costs extra is priced at its printed
// cost alone and gets no prefix search through a storage battery's X.
func (s *State) modalFloorFree(k *prep, p uint8, c CardID) bool {
	if !modalFloorZero(k, c) {
		return false
	}
	cd := &k.t.Cards[c]
	ab := &cd.Abilities[spellAb(k, c)]
	n := int(ab.NModes)
	if n > len(cd.ModeCost) {
		n = len(cd.ModeCost)
	}
	for m := 0; m < n; m++ {
		if mc := cd.ModeCost[m]; mc.MV() == 0 && mc.X == 0 && s.abilityTargetsFeasible(k, ab, p, NoSlot, 1<<m) {
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
	d := s.payDescNow(NoObj) // pdSpell with the entry's origin (mana_restrictedsource.go)
	s.availPotentialFor(k, e.ctrl, &av, &d, false)
	if k.tapMana {
		// gorge's PotentialMana counts a tap-another or sacrifice-another
		// source (Krark-Clan Ironworks' {C}{C}) the CR 601.2g window could
		// activate; the potential walk above leaves the sacrifice one a
		// census gap.
		s.availTapXUnwalked(k, e.ctrl, &av)
	}
	return av.affords(&c, pipMaskOf(&c), 0)
}

// availTapXUnwalked is availTapX less the sources availPotentialFor already
// counted: its fixpoint admits a tap-another source (Urza, Lord High
// Artificer's artifact tap) through potentialGapCost's TapXMana arm, so
// adding that unit again prices one mana gorge's PotentialMana does not have
// and offers a Spree mode it withholds (CR 601.2b, 702.172a).
func (s *State) availTapXUnwalked(k *prep, p uint8, av *avail) {
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		a := k.tapXAbOf(s.cardAt(b.first()))
		if k.gen != nil && kop.TapXMana(a) {
			continue // a unit of the potential walk's fixpoint already
		}
		addTapXUnits(av, a)
	}
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
// census omits (mana_deferred.go) and the window offers. A counter-cost
// source (availGaps: Mercadian Bazaar's storage counters, a Wall of Roots'
// counter) is one the window offers too, so it counts at its counters: a
// Tiered Blade Beam ({1}{W}+{1}) beside one floating R and W is paid by the
// Bazaar's X of 1 (CR 601.2g).
//
//go:noinline
func (s *State) availSpreeWindow(k *prep, e *stackEntry, av *avail) {
	x := s.g.availX
	if k.gen != nil {
		s.g.availX |= availGaps
	}
	if k.painMana {
		s.g.availX |= availWinCol
	}
	s.availForBuilt(k, e, av)
	s.g.availX = x
	for b := s.scriptDeferred(k, e.ctrl); b.any(); b = b.rest() {
		av.addUnits(s.deferMask(k, b.first()), 1)
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
		if s.restrAvail(k, e.ctrl, e.card, s.restrOriginOfEntry(e), &av) {
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
// cost still opens the window (restrManaUntapped). The abilities'
// payability gate is read: a tap-another one is a source only while its
// tap-another part has candidates (tapXActivatable; gorge ManaTapsPayable),
// and an ability whose cost needs no tap is a source however tapped its
// permanent is (gorge mana_available.go rejects only mf.Tap && o.Tapped),
// so the sacrifice-only, life-only-pain and no-{T} tap-another arms run
// ahead of the tapped skip. A counter-cost source that can be activated now
// (scriptCtrIdle, a storage battery holding no counters included: gorge
// prices its X at 0, Icatian Store) is a source as well; the mask carries
// its own {T} gate.
//
//go:noinline
func (s *State) hasUntappedMana(k *prep, p uint8) bool {
	ctr := s.scriptCtrIdle(k, p, true) // an empty storage battery is a source too: gorge prices it at X=0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		// the no-tap arms first (see above): a sacrifice-only source
		// (CR 602.2b), a life-only pain one, a tap-another one whose cost
		// taps another permanent
		if k.sacOnlyMana && s.canSacOnlyNowAt(k, sl) ||
			k.painMana && s.canLifeOnlyMana(k, sl, SeatIx(p)) || s.tapXActivatable(k, p, sl) || ctr.has(sl) || s.depleteLive(k, p, sl) {
			return true
		}
		if s.perms[sl].flags&pfTapped != 0 {
			continue
		}
		if s.canTapForMana(k, sl) || s.restrManaUntapped(k, sl) || s.eachColLive(k, p, sl) {
			return true
		}
	}
	return s.handManaAny(k, p) || s.graveManaAny(k, p) // a hand guide, a graveyard source (mana_handguide.go, mana_gravemana.go)
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
	payCost := cost
	if payCost.Hyb[0] == kop.HybTwobrid {
		// The cast planner resolves a twobrid cost's pips before pricing it
		// (planTwobridCost). This witness prices the cast before its
		// announcements, and the hybrid witness refuses the unresolved
		// marker; no face the census affords is no witness.
		var av avail
		s.availFor(k, p, &av)
		payCost = av.hybWitnessTb(payCost, 0, false)
		if payCost.Hyb[0] == kop.HybTwobrid {
			return false
		}
	}
	t := s.fork()
	n0 := t.npend
	var pk planner
	pk.try = true
	// the witness pays for this spell (gorge plans the cast option): a restricted
	// source that cannot pay a spell (a Powerstone's {C}) is no unit of it
	t.tapForPK(k, p, &payCost, 0, o, false, false, &pk)
	consumes := pk.found && plannerConsumesSac(&pk)
	reaches := false
	// a witness whose sacrifices stack more triggers than the wait budget lets
	// resolve never reaches the cast (witnessOverWaits)
	if consumes && !s.witnessOverWaits(k, p, c, o, int(t.npend)-int(n0)) {
		ab := spellAb(k, c)
		if ab == 255 || !cd.HasModeCost {
			// no target to lose, or an unmoded spell: reached when its targets
			// all remain after the payment and the floated pool pays the cost
			// as it is then priced: a count reduction over the sacrificed
			// permanents (Valkyrie Aerial Unit's affinity for a Treasure) has
			// dropped, so the lowering cannot reach the cast whose total
			// CR 601.2f fixed before that sacrifice (gorge witnessReachesMode
			// lists the cast only when the pool pays it). A card with no
			// AbSpell is not automatically targetless: an aura's Enchant spec
			// is its spell target (tspec), and the payment can consume
			// exactly that creature (Brilliant Wings over the Eldrazi Spawn
			// whose sac pays its {1}: gorge's replay then lists no cast, the
			// targets are gone; tail-20 pair 5).
			// judged as the spell it is (spellTargetsFeasible probes it as one): an
			// ability probe bars a target only an ability cannot take (Shanna's
			// "can't be the target of abilities your opponents control" under
			// Titanic Brawl, CR 115.2), and the cast would read as unreachable
			feas := t.spellTargetsFeasible(k, c, p) // an aura's Enchant spec is its spell target (tspec); a card with none keeps true
			reaches = feas && t.FeatPoolPays(p, o, 0)
			if reaches && t.witnessEndsGameLost(k, p, c, n0) {
				reaches = false // the sacrifices' triggers end the game against p before the cast (Pain Distributor's damage per Treasure)
			}
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

// witnessEndsGameLost: the witness fork t (taken before the cast's pre-cast
// triggers resolve) queued sacrifice triggers from n0 that, resolved as gorge's
// lowering lets them resolve, end the game with p out of it. gorge's
// witnessReachesMode replays the witness on a clone and leaves at once on a
// finished game unless p won (winOK), so the witness does not reach the play
// and targetPaidNeedsPlannedPayment plans the cast at the printed cost. An
// instant-speed cast is offered with the triggers still on the stack, below it.
func (t *State) witnessEndsGameLost(k *prep, p uint8, c CardID, n0 uint8) bool {
	instant := k.cards[c].instant || t.castFlash(k, p, c)
	if instant || t.npend <= n0 {
		return false
	}
	t.resolveLowerCounterTriggers(k, n0, false)
	return t.over && t.pl[SeatIx(p)].lost
}
