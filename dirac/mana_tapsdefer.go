package dirac

import (
	"math/bits"
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Taps watchers and the payment plan. gorge classifies every source a planned
// tap would touch (pay.PaymentPlanAbilityTier): a source whose {T} fires a
// "whenever a creature you control becomes tapped" watcher with an unvetted
// body (Judge of Currents' gain life, Magda's Treasure, Quest for Renewal's
// counter) is TierDeferred -- the census never lists it, so autopay never
// taps it (manual only) -- and one whose watcher deals vetted literal damage
// to the payer is a last-resort unit priced by that damage (Pain). Vetted
// damage that cannot reach the payer leaves the source an ordinary unit.
// kgen stamps the class on the watcher (kop.Trigger.TapPlan, closed world:
// lower_tapsclass.go); this file reads it per source.

// DebugNoTapsDefer drops the classification (every source an ordinary unit).
// The lockstep's mutation test flips it to prove the pool exercises it.
var DebugNoTapsDefer bool

// DebugNoTapsDeferYield drops the several-mana shapes from the gap
// (tapsDeferYield): a multi-mana or dynamic host keeps the ordinary census
// unit, as before the gap modelled its production. The lockstep's mutation
// test flips it to prove the recordings exercise it.
var DebugNoTapsDeferYield bool

// tapsWatchClass classifies a tap of sl for mana against the battlefield's
// lowered Taps watchers: painN is the damage the tap deals its payer (the
// Pain-class watchers' amounts, plus a deferred TapsForMana watcher's
// disclosed literal damage, kgen tapsDeferPain), deferSrc whether a
// Defer-class watcher matches. p is the payer. Tables without the family
// walk nothing.
func (s *State) tapsWatchClass(k *prep, p uint8, sl Slot) (painN int, deferSrc bool) {
	painN, deferSrc, _ = s.tapsWatchClassForeign(k, p, sl)
	return painN, deferSrc
}

// tapsWatchClassForeign is tapsWatchClass that also reports whether every
// Defer-class watcher matching the tap is a printed trigger another player
// controls whose build poses a target or mode ask (Relic Bind's mode on an
// enchanted Treasure), the shape gorge's prefix search cannot get past
// (mana_foreignask.go). A granted watcher never counts as one.
func (s *State) tapsWatchClassForeign(k *prep, p uint8, sl Slot) (painN int, deferSrc, foreignOnly bool) {
	if k.gen == nil || !(k.gen.anyTapsDefer || k.gen.anyTapsPain) || DebugNoTapsDefer {
		return 0, false, false
	}
	if s.reflTapsUnwatched(k, sl) {
		return 0, false, false
	}
	foreignOnly = true
	for m := s.subs[kop.TrigTaps]; m.any(); m = m.rest() {
		w := m.first()
		if !s.occ.has(w) || k.gen.flags[s.cardAt(w)]&(gfTapsDefer|gfTapsPain) == 0 || s.genLost(k, w) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(w)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigTaps || a.AFlags&kop.AfLatent != 0 ||
				a.Trig.TapPlan != kop.TapPlanDefer && a.Trig.TapPlan != kop.TapPlanPain ||
				a.Trig.Zone != kop.ZoneNone && a.Trig.Zone != kop.ZoneBattlefield {
				continue
			}
			// ValidPlayer$ reads the TAPPER (gorge tapsMatches/TapActor), and a
			// mana tap is the payer's; the listener's controller is only the
			// reference for "you"/"opponent" (tapsWhoseOK).
			if !whoseOK(a.Trig.Whose, s.perms[w].ctrl, p) {
				continue
			}
			if a.Trig.Flags5&kop.Tf5TapperNonActive != 0 && p == s.active {
				continue // Activator$ Player.NonActive: the payer taps on their own turn
			}
			if a.Trig.Flags&kop.TfSelf != 0 {
				// the source's own Pain-class line (City of Brass) is priced
				// with its printed mana ability (prepSelfTapsPain), and a
				// self line classes no other source
				continue
			}
			if !s.matchPermGen(k, a.Trig.Filter, sl, s.perms[w].ctrl, w, layerFull) {
				continue
			}
			// The gates gorge's real matcher applies (triggers_gen.go: FirstTime$
			// reads mechTappedTurn, YourTurn$ the turn's active player): a spent
			// or wrong-turn watcher matches no tap, so the source is an
			// ordinary planner unit (Shuri's crewed Quinjet beside a spent
			// Captain America).
			if a.Trig.Flags2&kop.Tf2FirstTime != 0 && s.mSl(k, mechTappedTurn, sl) {
				continue // an earlier tap this turn: FirstTime$ is false
			}
			if a.Trig.Flags2&kop.Tf2YourTurn != 0 && s.active != s.perms[w].ctrl {
				continue // not the watcher's controller's turn
			}
			// The class-only gated Taps line (Rikala, Homarid King): the Island
			// defers only while Trigger.Cond holds (gorge's real matcher,
			// paymentPlanTapObservers' triggerMatches over the IsPresent gate).
			if a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: s.perms[w].obj, srcSl: w, ctrl: s.perms[w].ctrl}) {
				continue
			}
			if a.Trig.TapPlan == kop.TapPlanDefer {
				deferSrc = true
				// A deferred TapsForMana watcher whose body still names the
				// tapper as literal damage (Manabarbs, kgen tapsDeferPain):
				// the planner never taps the source, but the tap's damage is
				// disclosed for the potential census's lethal guard.
				painN += int(a.Trig.TapPainN)
				if s.perms[w].ctrl == p || a.NTargets == 0 && a.NModes == 0 {
					foreignOnly = false
				}
			} else {
				painN += int(a.Trig.TapPainN)
			}
		}
	}
	if k.gen.grantTrigEv&(1<<kop.TrigTaps) != 0 {
		// A Taps trigger a layer-6 AddTrigger$ static grants (Tale of Katara and
		// Toph's "creatures you control have 'whenever this creature becomes
		// tapped for the first time...'") lives on the granting card; its
		// listener is each granted host (grantTrigHosts, grantedAbilitiesN).
		// The printed walk above reads only the watchers' own subscriptions and
		// skips TfSelf lines, so a granted watcher's self line (the recipient's
		// own tap) never classes the host: the planner tapped it inside a play's
		// payment, where gorge's planner defers it (pay.PaymentPlanAbilityTier
		// reads the granted watcher over the recipient) and its prefix search
		// taps the source before the play, the trigger flushing onto the stack
		// beneath the play (CR 117.5; repro projection_stack_kind_hall).
		wasDefer := deferSrc
		s.grantedTapsClass(k, sl, p, &painN, &deferSrc)
		if deferSrc && !wasDefer {
			foreignOnly = false
		}
	}
	if painN > 255 {
		painN = 255
	}
	return painN, deferSrc, foreignOnly && deferSrc
}

// tapsDeferDead is p's mana sources that the planner defers and whose every
// deferring watcher is a foreign build ask (tapsWatchClassForeign): gorge's
// census never lists such a source, and its search, which gap-activates it,
// ends at the opponent's ask, so no payment or prefix uses it and the plain
// availability leaves it out. A Treasure under an opponent's Relic Bind has
// no {T}-only gap of its own for the script to model (scriptTapsDefer), so
// without this it kept its census unit and the offer priced a cast gorge's
// autopay surface withholds (census_incomplete). So does a source whose
// production the gap does not model -- a Talisman's two abilities under the
// same Relic Bind -- and the activation it alone funded, such as an Equip
// {2} paid by that Talisman and a land.
func (s *State) tapsDeferDead(k *prep, p uint8) slotMask {
	var m slotMask
	if k.gen == nil || !k.gen.anyTapsDefer || !s.subs[kop.TrigTaps].any() {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if pc := s.manaPCGen(k, sl); !pc.manaSac && pc.manaAb < 0 {
			continue // no mana ability of its own: nothing to leave out
		}
		if _, d, f := s.tapsWatchClassForeign(k, p, sl); d && f {
			m = m.with(sl)
		}
	}
	return m
}

// DebugNoReflTapsUnwatched drops reflTapsUnwatched (a reflected source's tap
// is classed by the watchers like any other); tests only.
var DebugNoReflTapsUnwatched bool

// reflTapsUnwatched reports whether sl's live mana view is a lone AB$
// ManaReflected ability (pcard.manaRefl: Quirion Explorer, Exotic Orchard,
// Mox Amber, Chrome Mox), whose tap gorge's planner never runs past the Taps
// watchers. Its interference walk (rules paymentPlanSourceInterference) visits
// one production at a time from pay.PaymentPlanProductions, and a reflected
// ability has no Produced$, so that list is empty and the source stays a
// TierNormal census unit whatever watches its tap. Quirion Explorer under an
// opponent's Spirit Shackle ("whenever enchanted creature becomes tapped")
// therefore prices Dragon-Scarred Bear on gorge's autopay surface. A
// land-type setter's view clears manaRefl (static_landtypesetter.go): the
// intrinsic ability it leaves has a production, so its watchers still apply.
func (s *State) reflTapsUnwatched(k *prep, sl Slot) bool {
	return !DebugNoReflTapsUnwatched && k.gen.has(s.cardAt(sl), gfManaRefl) && s.manaPCGen(k, sl).manaRefl
}

// grantedTapsClass classes the granted Taps watchers over sl (tapsWatchClass's
// granted arm): every Taps instance a static in force grants sl, as the
// runtime fire walk sees them (triggers_gen.go slotTriggers' granted arm). It
// reads only, and honours DebugNoTapsDefer through tapsWatchClass's gate.
func (s *State) grantedTapsClass(k *prep, sl Slot, p uint8, painN *int, deferSrc *bool) {
	s.grantedAbilitiesN(k, sl, func(c CardID, ab uint8, n int) {
		if k.gen.flags[c]&(gfTapsDefer|gfTapsPain) == 0 {
			return
		}
		a := &k.t.Cards[c].Abilities[ab]
		// a granted trigger is AfLatent by construction (kgen grantedTrigger);
		// slotTriggers' granted arm has no latent gate
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigTaps ||
			a.Trig.Zone != kop.ZoneNone && a.Trig.Zone != kop.ZoneBattlefield {
			return
		}
		l := trigListener{obj: s.perms[sl].obj, sl: sl, ctrl: s.perms[sl].ctrl, card: c, ab: ab, host: s.g.grantSrc}
		for ; n > 0; n-- {
			if !s.grantedTapsMatch(k, l, a, p) {
				continue
			}
			if a.Trig.TapPlan == kop.TapPlanDefer {
				*deferSrc = true
			} else {
				*painN += int(a.Trig.TapPainN)
			}
		}
	})
}

// grantedTapsMatch applies the gates the printed arm mirrors of the runtime
// fire walk (objTrigGen's TrigTaps arm), with the granted listener's
// referents: the recipient is the tapped slot, its controller "you".
func (s *State) grantedTapsMatch(k *prep, l trigListener, a *kop.Ability, p uint8) bool {
	if !s.objEventMatch(k, l, a, l.sl) {
		return false
	}
	if !whoseOK(a.Trig.Whose, l.ctrl, p) {
		return false
	}
	if a.Trig.Flags5&kop.Tf5TapperNonActive != 0 && p == s.active {
		return false
	}
	if a.Trig.Flags2&kop.Tf2FirstTime != 0 && s.mSl(k, mechTappedTurn, l.sl) {
		return false
	}
	if a.Trig.Flags2&kop.Tf2YourTurn != 0 && s.active != l.ctrl {
		return false
	}
	if a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: l.obj, srcSl: l.sl, ctrl: l.ctrl}) {
		return false
	}
	return true
}

// tapsDeferred reports whether sl is a source gorge's planner never taps
// (tapsWatchClass's Defer class).
func (s *State) tapsDeferred(k *prep, p uint8, sl Slot) bool {
	if k.gen == nil || !k.gen.anyTapsDefer || !s.subs[kop.TrigTaps].any() {
		return false
	}
	_, d := s.tapsWatchClass(k, p, sl)
	return d
}

// tapsCensusPain is the foreign tap damage the potential census's per-tap
// view carries for sl (legal.go potCensus.tapPain): a Pain-class watcher's
// amount, or a deferred TapsForMana watcher's disclosed literal payer damage
// (Manabarbs). The over-bound walk still taps the source; only the units'
// lethal guard (potUnitsAvail) consumes this read.
func (s *State) tapsCensusPain(k *prep, p uint8, sl Slot) uint8 {
	if k.gen == nil || !(k.gen.anyTapsDefer || k.gen.anyTapsPain) || !s.subs[kop.TrigTaps].any() {
		return 0
	}
	pain, _ := s.tapsWatchClass(k, p, sl)
	return uint8(min(max(pain, 0), 255))
}

// tapsDeferLethalMask is the slots of m for which tapsDeferLethal holds.
func (s *State) tapsDeferLethalMask(k *prep, p uint8, m slotMask) slotMask {
	var out slotMask
	for ; m.any(); m = m.rest() {
		if sl := m.first(); s.tapsDeferLethal(k, p, sl) {
			out = out.with(sl)
		}
	}
	return out
}

// tapsDeferLethal reports whether a Taps-deferred source's tap deals its payer
// the damage its watcher discloses (tapsCensusPain: Manabarbs' literal 1) and
// that damage alone reaches the payer's life: gorge's planner cuts every plan
// whose summed pain reaches it (payment_plan.go's lethal guard), so the
// source is no unit of an offer's census. A source the prefix search models
// as a gap never reaches here (availFor's deferGap skip); the one kept as an
// ordinary unit (a two-ability land such as Unknown Shores) needs the guard.
func (s *State) tapsDeferLethal(k *prep, p uint8, sl Slot) bool {
	if !s.tapsDeferred(k, p, sl) {
		return false
	}
	pain := int(s.tapsCensusPain(k, p, sl))
	return pain > 0 && pain >= s.painPlanLife(p)
}

// playLifeNotSurvived reports whether play a is an activation whose own fixed
// PayLife<N> reaches p's life as it stands after a script prefix: gorge's
// harness replays the prefix and re-prices the play, and the planner's lethal
// guard (pain >= life) prices no activation that would pay the whole life
// total, so the line the search found is a dead end (CR 119.4, 601.2g;
// dr-mbc The Cauldron of Eternity beside Burning Earth at 3 life).
func (s *State) playLifeNotSurvived(k *prep, p uint8, a Action) bool {
	if a.Kind() != AkActivate {
		return false
	}
	o := Obj(a.Source())
	if o == 0 || int(o) >= len(s.objs) {
		return false
	}
	c := s.objs[o].card
	ai := int(a.Mode())
	if int(c) >= len(k.t.Cards) || ai >= len(k.t.Cards[c].Abilities) {
		return false
	}
	ab := &k.t.Cards[c].Abilities[ai]
	if ab.Kind == kop.AbMana || ab.Cost.Flags&kop.CostPayLife == 0 {
		return false
	}
	owed := lifeCostOwed(&ab.Cost)
	return owed > 0 && int(s.pl[SeatIx(p)].life) <= owed
}

// availLeavesOut reports whether availFor left the planner-deferred source at
// sl out of the availability its walk is building: a deferred source is no
// census unit (gorge's census never lists it) and only the any-mana and gaps
// modes (availScript) put its relaxed unit back. A cost that taps the source
// then has no mana of its own to take out, and removing its printed mana would
// take a floating unit of the pool instead: Cabal Pit's {B},{T}, sacrifice
// beside Price of Glory, paid by the {B} a scripted prefix tapped from a
// Swamp into the pool (CR 602.2b). The test is availFor's skip.
func (s *State) availLeavesOut(k *prep, sl Slot) bool {
	if !s.deferLive(k) || s.g.availX&(availWalk|availHyp|availGaps|availAny) != 0 {
		return false
	}
	p := s.perms[sl].ctrl
	return s.scriptTapsDefer(k, p).has(sl) && s.planDeferred(k, p, sl) && !s.blgIntrinsicUnit(k, sl)
}

// deferSelfTapped is the slot an activated play's cost taps on its own
// source (NoSlot otherwise): the play itself spends that {T}, so no script
// taps the source to fund its own play (deferGapsForPlay) and no search node
// hides it (hideDeferred) -- its tap's mana is out of that payment
// (availLeavesOut, CR 602.2b), and gorge's planner plans the pass-2 units
// without the source's own one. Tapping the Pit first (battlefield order)
// would leave the {T} its own cost spends spent and the play unlisted at
// every depth (Cabal Pit's {B},{T}, sacrifice beside Price of Glory).
func (s *State) deferSelfTapped(k *prep, a Action) Slot {
	if a.Kind() != AkActivate {
		return NoSlot
	}
	o := Obj(a.Source())
	if o == NoObj {
		return NoSlot
	}
	ab := a.Mode()
	if ab&0x80 != 0 {
		// a granted or granted-exile ability (legal_gen.go, cast_exilegrant.go):
		// its cost's tap is the host's, the grant search's play shape, not a
		// printed permanent's own activation
		return NoSlot
	}
	if ab >= abCascade || int(ab) >= len(k.t.Cards[s.objs[o].card].Abilities) {
		return NoSlot
	}
	if k.t.Cards[s.objs[o].card].Abilities[ab].Cost.Flags&kop.CostTap == 0 {
		return NoSlot
	}
	return s.objs[o].slot
}

// deferGapsForPlay drops the play's own tap-cost source from the deferred
// gaps (deferSelfTapped). A non-activation, or an activation whose cost does
// not tap, keeps every gap: the source's mana ability may still fund it.
func (s *State) deferGapsForPlay(k *prep, a Action, m slotMask) slotMask {
	if sl := s.deferSelfTapped(k, a); sl != NoSlot {
		m = m.without(sl)
	}
	return m
}

// scriptTapsDefer is p's activatable Taps-deferred mana sources, a slot
// mask: the census gap a live Defer-class watcher makes of a source's tap.
// Which sources defer is a runtime fact of the watchers alive, so no card
// flag reaches it (contrast gapMask's gfEachCol and friends); the walk is
// gapMask's, keyed on tapsDeferred. Only a tap is the source's mana
// activation for it -- gorge's tier classes planned taps
// (pay.PaymentPlanAbilityTier via paymentPlanSourceInterference) -- and the
// census gap the prefix search branches on (potentialPrefixScript) is a
// source whose missed ability is a plain one-mana-of-any-colour tap
// (Firdoch Core), whose relaxed alternatives (paymentPlanRelaxedAlternatives)
// are one mana per colour, or a plain one-mana tap of one fixed colour
// (Llanowar Elves under Cryoshatter): its one relaxed alternative, no colour
// ask (deferGapAnswers is 1). Left out, that source kept its census unit and
// the offer priced a plain cast with it, which gorge's census never lists and
// its autopay surface never scripts. Any other production stays unmodelled.
func (s *State) scriptTapsDefer(k *prep, p uint8) slotMask {
	if k.gen == nil || !(k.gen.anyTapsDefer || k.gen.anyManaRider) {
		return slotMask{}
	}
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.pickedForMana(s.perms[sl].obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
		c := s.cardAt(sl)
		if a, ok := s.riderAbilityOf(k, sl); ok {
			// A bounded rider source: the planner defers its first ability
			// (riderDeferred) or, with a plain first ability, its later
			// rider (Grove of the Burnwillows). Both are prefix gaps; the
			// activation pays the ability's own cost (mana_rider.go). A
			// granted mana ability beside the rider does not exclude the
			// gap: the grant stays the availability unit availGrant counts
			// (ownManaUsable is false for a rider-first source, so the
			// grant is all its census keeps), while gorge's census lists
			// neither its rider's production nor the grant and its one
			// prefix search activates both (scriptSearch.visit), so the
			// rider is the prefix gap the script taps beside it and the
			// source stays here, the grant search reading it as a gap too
			// (tapsDeferGrantGaps). Gorge's plan prices a play with both:
			// Saprazzan Skerry's "{T}, Remove a depletion counter: Add
			// {U}{U}" or Sandstone Needle's Add {R}{R} beside Rain of
			// Filth's granted sacrifice on the same land. The rider being
			// live is what the activatable test holds; a source whose
			// abilities are lost has no live rider (genLost in
			// riderActivatable) and a lost one is no gap.
			if k.gen.has(c, gfRestr|gfAnyR|gfEachCol|gfFilt|gfDeferred) ||
				!s.riderActivatable(k, p, sl, a) {
				continue
			}
			m = m.with(sl)
			continue
		}
		if !s.planDeferred(k, p, sl) {
			continue
		}
		if s.askColDeferred(k, sl) {
			if s.askColUsable(k, sl) {
				m = m.with(sl) // a colour-asking source (mana_askcolour.go): the prefix search poses its ask
			}
			continue
		}
		if s.splitDeferred(k, sl) {
			if s.splitUsable(k, sl) {
				m = m.with(sl) // a Condition-split source (mana_condsplit.go): its relaxed unit and answers are the live gate's
			}
			continue
		}
		if !s.manaGranted(k, sl) && (s.restrTapsGap(k, p, sl) || s.restrCensusTapsGap(k, p, sl)) {
			// A restricted source whose single restricted ability one fixed
			// colour makes (Hydraulic Helper): the same gap, its mana floating
			// restricted (tapDeferFloat's restrTaps arm, floatRestrAb), when
			// the source can be activated at all. A restricted ability beside
			// a census one (Primal Beyond's {C} beside its any-colour
			// Elemental ability) is the same gap for the play the restricted
			// ability pays; its tap floats the restricted mana too.
			if s.tapsGapActivatable(k, sl) {
				m = m.with(sl)
			}
			continue
		}
		if s.manaGranted(k, sl) || k.gen.has(c, gfRestr|gfAnyR|gfEachCol|gfFilt|gfDeferred) && !s.landSetLost(k, sl) {
			// Not the plain unrestricted any-colour shape the census gap is:
			// a restricted source's mana must float restricted (tapDeferFloat
			// adds plain pool mana), a granted one is grantCap/grantPrefix's,
			// an each-colour or filter one has its own arm, and a deferred one
			// is tapGap's gfDeferred arm. Each keeps the census unit the
			// availFor skip would otherwise drop (the skip is keyed on this
			// mask). A land a type setter stripped (landSetLost) has none of
			// those printed abilities left (CR 305.7: Unknown Shores' {1},{T}
			// filter under Contaminated Ground): only its intrinsic mana
			// remains, which landSetGap below reads.
			continue
		}
		// a rider source whose view is the granted types' intrinsic unit
		// (blgView) qualifies here on its card's own facts: the gap is its
		// deferred rider, not the intrinsic unit (blgGapPC)
		pc := s.blgGapPC(k, sl)
		if pc.manaAb < 0 || !pc.manaTap || !tapsDeferColour(kop.ColorMask(s.srcMaskAt(pc, sl))) ||
			pc.manaFixedChosen || pc.manaOne || !tapsDeferYield(pc, s.srcMaskAt(pc, sl)) {
			continue
		}
		if s.landSetGap(k, sl) {
			// A land a type setter made an Island (Spreading Seas on a Forest):
			// only its intrinsic ability is left, which the planner defers
			// like any Island's under Rikala and gorge's prefix search taps as
			// a census gap. It has no card ability to read; the view's tap is
			// the activation (tapDeferFloat).
			m = m.with(sl)
			continue
		}
		if !s.landSetIntrinsic(k, c, pc) && (int(pc.manaAb) >= len(k.t.Cards[c].Abilities) || k.t.Cards[c].Abilities[pc.manaAb].ManaRestrict != 0) {
			continue // the ability is restricted (Pillar of the Paruns): the gap floats plain mana
		}
		if pc.painCol != 0 && int(pc.painN) >= s.painPlanLife(p) {
			// a tap whose pain alone reaches the payer's life is no gap the
			// script branches on: the payment it funds would end the payer
			// mid-play, so no script pays with it (painUnit's guard, pay.
			// PlanLastResortChoices' one-step guard) -- gorge's search only
			// walks the branch to watch it end the payer and backtracks.
			// Cabal Pit's {B} beside Price of Glory at 1 life.
			continue
		}
		if !s.tapsGapReady(k, sl) || s.genLost(k, sl) && !s.landSetLost(k, sl) {
			continue // a land under a land-type setter still taps for its intrinsic mana (CR 305.7)
		}
		m = m.with(sl)
	}
	return m
}

// tapsDeferYield reports whether pc's tap is a shape the Taps-deferred gap
// models: one mana of the colours mask names (any colour, a Combo or one fixed
// colour), or several mana of ONE fixed colour -- a literal Amount$ above one
// (Llanowar Tribe's {G}{G}{G}) or a dynamic one the view evaluates now
// (Priest of Titania's {G} per Elf). gorge's relaxation of such a source is
// one alternative of the whole production and its prefix search takes it with
// no colour ask, so the gap floats the full yield (tapDeferFloat); several
// mana of a choice of colours poses an ask per mana and stays unmodelled.
func tapsDeferYield(pc *pcard, mask uint8) bool {
	if pc.manaN == 1 && !pc.manaDyn {
		return true
	}
	return !DebugNoTapsDeferYield && pc.manaN >= 1 && pc.manaN == srcYield(pc) && !pc.manaMulti && bits.OnesCount8(mask) == 1
}

// landSetGap: sl lost its own abilities to land-type setters alone
// (landSetLost) and can still be tapped for its intrinsic mana now: untapped,
// not summoning-sick, under no CantBeActivated lock.
func (s *State) landSetGap(k *prep, sl Slot) bool {
	return k.gen.landSet && s.landSetLost(k, sl) && s.perms[sl].flags&pfTapped == 0 && !s.activationSick(k, sl) &&
		!(k.gen.anyCantAct && s.actForbidden(k, sl, true))
}

// tapsGapActivatable: the Taps-deferred gap at sl can be tapped for mana now:
// untapped, not summoning-sick, still its lowered card, and under no
// CantBeActivated lock (cast_cantactivate.go). Every script gap arm that taps
// its source asks it.
func (s *State) tapsGapActivatable(k *prep, sl Slot) bool {
	return s.tapsGapReady(k, sl) && !s.genLost(k, sl)
}

// tapsGapReady is tapsGapActivatable without the lost-abilities test: the
// source is untapped, not summoning-sick and under no CantBeActivated lock.
func (s *State) tapsGapReady(k *prep, sl Slot) bool {
	return s.perms[sl].flags&pfTapped == 0 && !s.activationSick(k, sl) &&
		!(k.gen.anyCantAct && s.actForbidden(k, sl, true))
}

// tapsDeferColour is the production scriptTapsDefer models: a choice among
// one or more of WUBRG (any colour, a Combo such as Razortide Bridge's W or
// U, or one fixed colour), or {C} (Crumbling Vestige under Brink of
// Disaster). gorge's relaxed alternatives are one mana per colour the
// production names, so the gap's answers are those colours
// (deferGapAnswers), and a source whose production was left out kept its
// census unit in availFor, offering a cast gorge's census never prices. A
// chosen-colour source (Cliffgate's R or the chosen colour) names its printed
// colours beside the chosen one, so any colours of WUBRG: gorge's mana ask
// lists each.
func tapsDeferColour(c kop.ColorMask) bool {
	return c == kop.ColC || c != 0 && c&^kop.ColAny == 0
}

// chosenAskOrder writes, as WUBRG indexes, the colours of the chosen-colour
// source at sl in the order gorge's mana ask lists them: the printed colours
// in their printed order (manaColourOrder), then the chosen colour last
// (pay.SubstituteChosenProduced rewrites the value's last token, dropping a
// repeat of a printed colour). Cliffgate with white chosen asks R then W,
// though the engine order is W then R. It returns how many it wrote.
func (s *State) chosenAskOrder(k *prep, sl Slot, pc *pcard, out *[5]uint8) int {
	ab := k.t.Cards[s.cardAt(sl)].Abilities[pc.manaAb] // a stack copy: only ManaColors changes
	printed := srcMask(pc)
	ab.ManaColors = kop.ColorMask(printed)
	n := manaColourOrder(&ab, out)
	if ch := s.perms[sl].chosen; ch < 5 && printed&(1<<ch) == 0 {
		out[n] = uint8(ch)
		n++
	}
	return n
}

// restrTapsGapShape: the source at sl is a restricted planner source whose
// only mana production is one restricted ability of one fixed colour
// (Hydraulic Helper's artifact-spells {U}, a Powerstone token's {C}): the Taps-deferred gap the script
// route funds, floating its mana restricted (floatRestrAb). A source with a
// choice between restricted abilities (Steelswarm Operator) is no such gap.
func (s *State) restrTapsGapShape(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	if k.gen == nil || k.gen.rs == nil || k.cards[c].manaAb >= 0 || !k.gen.has(c, gfRestrSrc) {
		return false
	}
	n := 0
	var rec *restrRec
	for i := range k.gen.rs.recs {
		if k.gen.rs.recs[i].card == c {
			n++
			rec = &k.gen.rs.recs[i]
		}
	}
	if n != 1 || rec.kind != rkExt {
		return false
	}
	a := &k.t.Cards[c].Abilities[rec.ab]
	return a.Kind == kop.AbMana && !a.ManaEach &&
		a.AFlags&(kop.AfManaChoice|kop.AfManaDynamic|kop.AfManaChosen) == 0 &&
		oneColour(a.ManaColors&colOrC) &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// restrTapsGap: the source at sl is Taps-deferred (a live Defer-class watcher
// with the gates gorge's matcher applies, tapsWatchClass) and of the script
// route's restricted shape.
func (s *State) restrTapsGap(k *prep, p uint8, sl Slot) bool {
	return s.tapsDeferred(k, p, sl) && s.restrTapsGapShape(k, sl)
}

// restrCensusTapsGapShape: the source at sl has a census mana ability beside
// one restricted mana ability of its own (Primal Beyond's {C} beside its
// any-colour ability restricted to Elemental spells and abilities). Unlike
// restrTapsGapShape's no-census source, the census unit is already the
// source's ordinary planner unit; the gap is the restricted ability a play
// the census cannot pay needs, tapped and floated restricted (tapDeferFloat).
func (s *State) restrCensusTapsGapShape(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	if k.gen == nil || k.gen.rs == nil || k.cards[c].manaAb < 0 || !k.gen.has(c, gfRestrSrc) ||
		s.splitDeferred(k, sl) {
		return false
	}
	if _, ok := s.riderAbilityOf(k, sl); ok {
		return false // the rider gap's units are riderGap's (addTapsDeferUnits, tapDeferFloat)
	}
	n := 0
	var rec *restrRec
	for i := range k.gen.rs.recs {
		if k.gen.rs.recs[i].card == c {
			n++
			rec = &k.gen.rs.recs[i]
		}
	}
	if n != 1 || rec.kind != rkWiden {
		return false
	}
	a := &k.t.Cards[c].Abilities[rec.ab]
	return a.Kind == kop.AbMana && !a.ManaEach && !a.ManaDeferred &&
		a.AFlags&(kop.AfManaDynamic|kop.AfManaChosen) == 0 &&
		a.ManaColors&kop.ColAny != 0 && a.ManaColors&^kop.ColAny == 0 && a.ManaN >= 1 &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// restrCensusTapsGap: the source at sl is planner-deferred and of
// restrCensusTapsGapShape.
func (s *State) restrCensusTapsGap(k *prep, p uint8, sl Slot) bool {
	return s.tapsDeferred(k, p, sl) && s.restrCensusTapsGapShape(k, sl)
}

// addTapsDeferUnits adds the census unit of the Taps-deferred gap at sl to a:
// its own mana and the extra a StatTapExtra carrier brings with the tap
// (Badgermole Cub; gorge's relaxed alternative carries it too,
// potential_plan.go potentialTapExtra). A restricted gap's unit is its
// restricted ability's production, priced beside the restriction as any gap's
// is (the play pays it from the floating batches). A source the extra cannot
// be priced on stalls (srcExtra).
func (s *State) addTapsDeferUnits(k *prep, sl Slot, av *avail) {
	if s.askColDeferred(k, sl) {
		s.askColUnits(k, sl, av) // mana_askcolour.go
		return
	}
	if s.restrCensusTapsGapShape(k, sl) {
		// One tap is one alternative of either ability (CR 605.1a): the
		// census unit beside the restricted colours, gorge's one relaxed
		// alternative per mana ability, priced as a single unit so one tap
		// is never spent twice. The census answer still prices a generic or
		// {C} play the restricted batch cannot pay (Orcish Captain's {1}
		// flip), the restricted colours an Elemental play's coloured pips
		// (Flamekin Brawler's {R}).
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.restrRecOf(s.cardAt(sl)).ab]
		pc := s.manaPCGen(k, sl)
		av.addUnits(s.srcMaskAt(pc, sl)|uint8(ab.ManaColors&kop.ColAny),
			max(s.srcYieldAt(k, sl, pc), max(ab.ManaN, 1)))
		return
	}
	if s.restrTapsGapShape(k, sl) {
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.restrRecOf(s.cardAt(sl)).ab]
		av.addUnits(uint8(ab.ManaColors&colOrC), max(ab.ManaN, 1))
		return
	}
	if a, ok := s.riderAbilityOf(k, sl); ok {
		// The rider's own {T} is one tap for mana like any other (CR 605.3a):
		// the reflected carrier's extra is one more of the pick (srcYieldAt),
		// a fixed carrier's joins the unit as its own (srcExtra, manaRiderFloat
		// floats both; gorge's planner folds them into the tap too,
		// payment_plan_interference.go paymentPlanTapObservers).
		pc := s.riderPC(k, sl, a)
		n := max(a.ManaN, 1) + s.srcReflectPick(k, sl, pc)
		if k.gen.anyTapExtra && s.srcEachPick(k, sl, pc) > 0 {
			// the pick floats unasked (nothing poses its colour ask yet);
			// srcExtra reads the production alone, and a payment priced on it
			// would drop the pick silently (mana_eachpick_stall_test.go)
			s.capFull(StallTapExtra)
			return
		}
		if k.gen.anyTapExtra {
			xv := s.srcExtra(k, sl, pc)
			for c := range xv {
				if xv[c] > 0 {
					av.addUnits(1<<c, xv[c])
				}
			}
		}
		if a.ManaEach {
			for col := 0; col < kop.NumManaKinds; col++ {
				if uint8(a.ManaColors)&(1<<uint(col)) != 0 {
					av.addUnits(1<<uint(col), n)
				}
			}
			return
		}
		av.addUnits(uint8(a.ManaColors), n)
		return
	}
	pc := s.blgGapPC(k, sl) // an intrinsic-unit rider source's gap unit is its rider's (blgGapPC)
	if s.splitDeferred(k, sl) {
		pc = s.splitCensusPC(k, sl) // the root's production plus the arm's while its gate holds (mana_condsplit.go)
		if av.pipWin {
			pc = s.splitWindowPC(k, sl) // a pip ask prices the window's census (announceFeasible), the arm's alone while its gate holds
		}
	}
	mask := s.srcMaskAt(pc, sl)
	if col, ok := tapsDeferPainCol(k, pc, mask); ok {
		mask |= 1 << col // the paid-life ability is one more relaxed alternative of the source
	}
	if k.gen.anyTapExtra && s.srcEachPick(k, sl, pc) > 0 {
		// the pick floats unasked (nothing poses its colour ask yet);
		// srcExtra reads the production alone, and a payment priced on it
		// would drop the pick silently (mana_eachpick_stall_test.go)
		s.capFull(StallTapExtra)
		return
	}
	av.addUnits(mask, s.srcYieldAt(k, sl, pc))
	if k.gen.anyTapExtra {
		xv := s.srcExtra(k, sl, pc)
		for c := range xv {
			if xv[c] > 0 {
				av.addUnits(1<<c, xv[c])
			}
		}
	}
}

// tapsDeferGapOwn reports whether the Taps-deferred source sl's census gap is
// a production of its own that availForMode's loop does not count (a
// restricted or rider ability's), against the ordinary tap it counts once.
// The trigger X announcement's capped census (availForCapped) counts the
// ordinary tap in the loop, so adding addTapsDeferUnits' as well would count
// a Condition-split source's mana twice (gorge's census lists it once).
func (s *State) tapsDeferGapOwn(k *prep, sl Slot) bool {
	if s.restrCensusTapsGapShape(k, sl) || s.restrTapsGapShape(k, sl) {
		return true
	}
	_, rider := s.riderAbilityOf(k, sl)
	return rider
}

// deferGapAnswers is how many colour answers a Taps-deferred gap's
// activation poses: one per colour its production names (gorge's search
// branches on the mana ask's options, manaChoiceColours W U B R G), plus
// the plain first ability as answer 0 when the rider is a later ability
// (Grove of the Burnwillows; riderPlainFirst).
func (s *State) deferGapAnswers(k *prep, sl Slot) uint8 {
	if s.askColDeferred(k, sl) {
		return s.askColDeferAnswers(k, sl) // mana_askcolour.go
	}
	if s.restrCensusTapsGapShape(k, sl) {
		// the census float first (answer 0, the plain first ability's place,
		// riderPlainFirst), then one answer per colour of the restricted
		// ability: the activation ask's options are both abilities'
		// productions (restrCensusDeferGap).
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.restrRecOf(s.cardAt(sl)).ab]
		return 1 + uint8(bits.OnesCount8(uint8(ab.ManaColors&kop.ColAny)))
	}
	if s.restrTapsGapShape(k, sl) {
		return 1 // one fixed colour: no colour ask
	}
	if a, ok := s.riderAbilityOf(k, sl); ok {
		if a.ManaEach {
			return 1 // every colour at once: no colour ask
		}
		n := uint8(bits.OnesCount8(uint8(a.ManaColors)))
		if s.riderPlainFirst(k, sl) {
			n++
		}
		return n
	}
	pc := s.blgGapPC(k, sl) // an intrinsic-unit rider source's answers are its rider's colours (blgGapPC)
	mask := s.srcMaskAt(pc, sl)
	n := uint8(bits.OnesCount8(mask))
	if _, ok := tapsDeferPainCol(k, pc, mask); ok && int(s.pl[SeatIx(s.perms[sl].ctrl)].life) >= int(pc.painN) {
		n++ // the paid-life ability's colour is the last answer (tapDeferFloat)
	}
	return n
}

// tapsDeferPainCol is the colour of a life-pain land's paid-life ability
// (Ramunap Ruins' "{T}, Pay 1 life: Add {R}" beside its {C}) when it is a
// planner-deferred gap's extra answer: one WUBRG colour the normal
// alternative (mask) does not make. gorge's relaxation of a deferred source
// (paymentPlanRelaxedAlternatives) is one free alternative per mana ability,
// and its prefix search activates each ability's option in turn, the plain
// one first, so the kernel's gap answers are the normal colours then this one
// (CR 605.3b; Price of Glory beside Ruins and an {R} play).
func tapsDeferPainCol(k *prep, pc *pcard, mask uint8) (int, bool) {
	if !k.painMana || pc.painOnly || !pc.painLife || pc.painN == 0 {
		return 0, false
	}
	c := uint8(pc.painCol)
	if c == 0 || c&^0x1f != 0 || bits.OnesCount8(c) != 1 || c&mask != 0 {
		return 0, false
	}
	return bits.TrailingZeros8(c), true
}

// tapDeferPlain floats a rider source's plain first ability at sl, the
// answer-0 activation of a gap whose rider comes later (riderPlainFirst):
// one fixed colour into the plain pool, exactly tapDeferFloat's non-rider
// arm.
func (s *State) tapDeferPlain(k *prep, p uint8, sl Slot, a Action) {
	s.fillTapAnyCol(k, p, sl, a)
	pc := s.manaPCGen(k, sl)
	col := bits.TrailingZeros8(uint8(pc.manaCol))
	s.pl[p].pool[col]++
	s.addExtra(k, p, sl, pc) // the tap's StatTapExtra mana floats beside it (addTapsDeferUnits)
	s.artTag(k, p, sl, col)
	if k.painMana && pc.painCol != 0 && pc.painOnly {
		s.painDamage(k, sl, p, int(pc.painN))
	}
}

// restrCensusDeferGap activates a restrCensusTapsGapShape source at sl for
// the script search's answer c: the source's one activation ask offers the
// census ability's production (answer 0, the plain first ability's place,
// riderPlainFirst) before the restricted ability's colours, and gorge's
// scriptSearch stably reorders those answers by the play's needed pips
// (scriptOrderAnswers). c indexes that reordered list; the census answer
// floats its plain {C} (tapDeferPlain), a colour answer floats the restricted
// mana (floatRestrAb) the payment admits only on an Elemental play (CR
// 605.1a).
func (s *State) restrCensusDeferGap(k *prep, p uint8, a Action, sl Slot, c int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if c == 0 {
		s.tapDeferPlain(k, p, sl, a)
		return
	}
	ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.restrRecOf(s.cardAt(sl)).ab]
	mask := uint8(ab.ManaColors & kop.ColAny)
	var order [kop.NumManaKinds]uint8
	no := 0
	for col := uint8(0); col < kop.NumManaKinds; col++ {
		if mask&(1<<col) != 0 {
			order[no] = col
			no++
		}
	}
	needFirst(order[:no], s.playNeed(k, a))
	if c-1 >= no {
		return // unreachable: deferGapAnswers bounds c (one per mask colour)
	}
	if !s.floatRestrAb(k, p, sl, ab, int(order[c-1]), max(ab.ManaN, 1)) {
		s.capFull(StallRestrSrc) // every batch is taken: never a silent spill
	}
}

// tapDeferFloat floats the Taps-deferred source at sl's mana in the
// answer's colour: tapGap's Taps-defer arm, plain pool mana (tapGapChoice
// and the prefix player tap it first, so manaTapped has queued the
// watcher's trigger for flushPrefixTriggers to place before the play).
func (s *State) tapDeferFloat(k *prep, p uint8, sl Slot, c int, play Action) {
	s.fillTapAnyCol(k, p, sl, play)
	if s.askColDeferred(k, sl) {
		s.askColDeferFloat(k, p, sl, c) // mana_askcolour.go
		return
	}
	if a, ok := s.riderAbilityOf(k, sl); ok {
		s.manaRiderFloat(k, p, sl, a, c)
		return
	}
	if s.restrTapsGapShape(k, sl) {
		// the restricted gap's mana floats restricted (tapGap's restr arm):
		// the play pays it from the floating batches the payment admits
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.restrRecOf(s.cardAt(sl)).ab]
		if !s.floatRestrAb(k, p, sl, ab, bits.TrailingZeros8(uint8(ab.ManaColors)), max(ab.ManaN, 1)) {
			s.capFull(StallRestrSrc) // every batch is taken: never a silent spill
		}
		return
	}
	pc := s.blgGapPC(k, sl)     // an intrinsic-unit rider source floats its rider's mana (blgGapPC)
	mask := s.srcMaskAt(pc, sl) // a Combo Chosen source (Cliffgate) reads its chosen colour beside the printed one
	if pcol, ok := tapsDeferPainCol(k, pc, mask); ok && c == bits.OnesCount8(mask) {
		// the paid-life ability's answer: its colour and its life (painDamage reads painLife)
		s.pl[p].pool[pcol]++
		s.addExtraCol(k, p, sl, pc, pcol) // the tap's StatTapExtra mana floats beside it, of the pick this tap made (CR 106.12a)
		s.artTag(k, p, sl, pcol)
		s.painDamage(k, sl, p, int(pc.painN))
		return
	}
	for i := 0; i < c; i++ {
		mask &= mask - 1
	}
	col := bits.TrailingZeros8(mask)
	s.pl[p].pool[col] += max(pc.manaN, 1) // a several-mana fixed-colour tap floats all of it (tapsDeferYield)
	// the tap's StatTapExtra mana floats beside it, the pick-priced reflection
	// one more of the pick this float names (CR 106.12a; addExtra stalls on it,
	// and a dropped play this census prices was the offer divergence: Mana
	// Flare over a lucky Gemstone Caverns funding Evincar's Justice with a
	// Plains, pool dr-mbc-abbb pair 45)
	s.addExtraCol(k, p, sl, pc, col)
	s.artTag(k, p, sl, col)
	if k.painMana && pc.painCol != 0 && pc.painOnly {
		s.painDamage(k, sl, p, int(pc.painN)) // a pain-only source's rider resolves with its mana (Elves of Deep Shadow)
	}
}

// deferSeenKey is gorge's scriptStateKey over the deferred gaps' search: the
// tapped gaps (a set, not their order), the floating pool, the restricted
// mana's partition and the stack the taps queued. gorge's Pool counts each
// restricted unit with its colour, while State.rb holds that unit beside the
// raw pool (floatRestrAb leaves pool untouched), so the key reads poolProj
// beside the batches. Hashing the raw pool alone made two answers that differ
// only in their restricted colour one state and dropped the second's subtree
// (Primal Beyond's five-colour restricted ability; restrCensusDeferGap).
type deferSeenKey struct {
	taps [(MaxPerm + 63) / 64]uint64
	pool [kop.NumManaKinds]uint8
	rb   [maxRB]rbatch
	pend uint8
}

// deferSeen is a search's visited set: the map it replaces, its keys scanned
// in insertion order. It is a fixed array so the search allocates nothing;
// seenSet records at most one key per node and the walk stops past its node
// budget (potentialScriptNodes), so the array holds every key a search can
// reach.
type deferSeen struct {
	n    int
	keys [potentialScriptNodes]deferSeenKey
}

// seenSet records the state of a search node and reports whether an earlier
// node had already reached it. Full reports not-a-duplicate: the search is
// already past its node budget there, and the budget cuts the walk anyway.
func (d *deferSeen) seenSet(used []int8, pool [kop.NumManaKinds]uint8, rb [maxRB]rbatch, pend uint8) bool {
	key := deferSeenKey{pool: pool, rb: rb, pend: pend}
	for _, u := range used {
		key.taps[u>>6] |= 1 << (uint(u) & 63)
	}
	for i := 0; i < d.n; i++ {
		if d.keys[i] == key {
			return true
		}
	}
	if d.n == len(d.keys) {
		return false
	}
	d.keys[d.n] = key
	d.n++
	return false
}

// deferInUsed reports whether gap i is already tapped on the search's path.
func deferInUsed(used []int8, i int) bool {
	for _, u := range used {
		if int(u) == i {
			return true
		}
	}
	return false
}

// deferDuplicate reports whether the replayed node is a state gorge's visited
// set (scriptStateKey) already reached: the same tapped set, pool (restricted
// units included, as gorge's Pool counts them), restricted batches and pending
// stack in another tap order. A post node is the pass that resolves the stack,
// not a tap state gorge's key marks, so it is never a duplicate. t is the
// node's fork.
func deferDuplicate(replay, post bool, seen *deferSeen, t *State, p uint8, used []int8) bool {
	return replay && !post && seen.seenSet(used, t.poolProj(p), t.rb[SeatIx(p)], t.npend)
}

// deferAskNodes charges the activation's colour ask as gorge's search prices
// it (scriptSearch.visit): an ask with more than one answer is a clone of its
// own beside the one per answer, so the walk pays one extra node for it.
func deferAskNodes(n int) int {
	if n > 1 {
		return 1
	}
	return 0
}

// deferReplay taps the path's gaps on the node fork t, in tap order, with the
// answer each was chosen at. It reports whether every tap was payable: a
// life-only gap the path's earlier activation made unpayable ends the replay.
func deferReplay(t *State, k *prep, p uint8, a Action, order []Slot, used []int8, answers []uint8, tapsDef slotMask, spill *[kop.NumManaKinds]uint8) bool {
	for u := range used {
		if !t.scriptGapTap(k, p, a, order[used[u]], slotMask{}, slotMask{}, slotMask{}, tapsDef, slotMask{}, int(answers[u]), spill) {
			return false
		}
	}
	return true
}

// deferChainCharge returns the clones gorge's pass chain and the answers under
// its intermediate resolutions add to the post node walked on fork t (the node
// itself already counted): one per queued trigger resolved short of the last,
// and, under each of those, every remaining gap's activation ask and each of
// its answers, the children gorge's scriptSearch.visit branches. The child term
// is a floor on the walk's clones: the ask node per multi-answer gap plus one
// per answer, while pricing the whole subtree at its worst over-withholds plays
// the walk still finds (Charix, Grasping Dunes). The chain term is the
// queued triggers' count, cut where a resolution kills the payer
// (deferChainLen, mana_deferchain.go), so a trigger that does not damage the
// payer is not undercharged. The bool reports whether
// nodes plus the charge stays inside gorge's potentialScriptNodes.
func (s *State) deferChainCharge(k *prep, t *State, nodes int, order []Slot, used []int8, replay, post, resolvePlay, twinCut bool) (int, bool) {
	if !replay || !post || !resolvePlay || t.npend == 0 {
		return 0, true
	}
	sub := 0
	var tw twinWalk // the unused gaps already charged, for gorge's twin cut
	for i := 0; i < len(order); i++ {
		if !deferInUsed(used, i) {
			if twinCut && tw.skip(s, order, i) {
				continue // the twin cut skips it beside an earlier twin (scriptTwin)
			}
			n := int(s.deferGapAnswers(k, order[i]))
			sub += n + deferAskNodes(n)
		}
	}
	chain := s.deferChainLen(k, t)
	charge := chain - 1 + (chain-1)*sub
	return charge, nodes+charge <= potentialScriptNodes
}

// deferGapSearch is the Taps-deferred gaps' search on a board whose only
// census gaps are those sources: gorge's one prefix search (potential_script.go
// scriptSearch.visit), whose visit explores every subset of the gaps -- a
// node's children are the remaining gaps in option order, the pass that lets
// the queued triggers resolve after them (mana_scriptpost.go) -- where
// scriptDFS walks the ordered prefixes alone. A cost whose pips only a later
// gap pay, and an earlier gap whose prefix kills the payer (Burning Earth's
// damage to the tapper beside Dragon-Cursed Halls and Starlit Sanctum: the
// Sanctum-then-Halls prefix deals 2 at 2 life and ends the search's clone,
// while the Halls alone floats the {B} Planar Despair's {3}{B}{B} needs), is
// that later sibling, and the script is the taps and answers along the first
// line that offers the play, gorge's visit order (CR 601.2b with 605.4a).
//
// cp != nil is the pre-push payment search (payCastPrefixRest): the node stop
// is gorge's sr.priced check -- the planner prices the planned cast's payment
// from the node's floats -- not the offer. gorge's offer of a planned cast is
// insensitive to its payment (FeatPlannedCast), so the offer stop would take
// the first line's taps whatever they float.
// TapsDeferGapStats counts the subsets searches (deferGapSearch) the kernel
// ran: the pool tests read it so a board that never reaches the subsets
// search cannot pass vacuously.
var TapsDeferGapStats struct {
	Searches atomic.Int64
}

// deferGapScratch is the subsets search's recursion state, one per search in
// the caller's frame: the walk function writes through its pointer and never
// stores it, so the arrays stay on the stack and the search allocates nothing
// (the sweep's map and slices were four heap objects a decision).
type deferGapScratch struct {
	cut     [MaxPerm]bool // gap i: every answer so far ended the line at another player's ask
	removed [MaxPerm]bool // the gaps gorge's search goes on without (scriptDFS cutAll)
	seen    deferSeen     // gorge's visited set (scriptStateKey)
	used    [MaxPerm]int8 // the tapped gaps, in tap order (nu their count)
	answers [MaxPerm]uint8
	nu      int
	nodes   int
	stall   uint8
	// limited: the search ran past gorge's node budget (scriptSearch.limited),
	// the condition for its subsume run (deferSubsume).
	limited bool
	// subsume: this is the subsume run, whose multi-answer colour asks keep
	// only the colours costCol (the play's coloured pips) still lacks of the
	// node's pool.
	subsume bool
	costCol [kop.NumManaKinds]uint8
}

func (s *State) deferGapSearch(k *prep, a Action, p uint8, order []Slot, tapsDef slotMask, cp *castRestPay, ans *[MaxPerm]uint8, pick *[MaxPerm]int8, dead *bool) bool {
	if len(order) == 0 {
		return false
	}
	TapsDeferGapStats.Searches.Add(1)
	var w deferGapScratch
	noGrant := a.Kind() == AkActivate && s.restrictedGrantLive(k, p)
	resolvePlay := s.playNeedsEmptyStack(k, a)
	twinCut := cp == nil && isAbilityPlay(a) // gorge's potentialTwinCut on the prefix search
	if deferGapWalk(s, k, a, p, order, tapsDef, cp, ans, pick, dead, noGrant, resolvePlay, twinCut, &w, false) {
		return true
	}
	if !w.limited {
		return false
	}
	return s.deferSubsume(k, a, p, order, tapsDef, cp, ans, pick, dead, noGrant, resolvePlay, twinCut)
}

// deferSubsume is gorge's subsume run (potentialPrefixScript's third search,
// the one mana_grantsubsume.go models for the grant walk) over the deferred
// gaps: where the plain search ran out of its node budget and the play's base
// cost is coloured pips alone, gorge searches again with every multi-answer
// colour ask narrowed to the colours the node's pool still lacks of that cost
// (a covered colour pays nothing more, CR 601.2g-h, 605.1a). A basic land
// beside a dual land under Manabarbs (seven gaps, one {U} Aura) spends the
// plain search's 164 nodes on the dual land's two off-colour answers at every
// depth; the subsume run drops them and finds the Islands' line.
func (s *State) deferSubsume(k *prep, a Action, p uint8, order []Slot, tapsDef slotMask, cp *castRestPay, ans *[MaxPerm]uint8, pick *[MaxPerm]int8, dead *bool, noGrant, resolvePlay, twinCut bool) bool {
	cnt, ok := s.playCostCounts(k, a)
	if !ok {
		return false
	}
	var w deferGapScratch
	w.subsume, w.costCol = true, cnt
	return deferGapWalk(s, k, a, p, order, tapsDef, cp, ans, pick, dead, noGrant, resolvePlay, twinCut, &w, false)
}

// deferSubsumeKeeps is subsumeAnswers' keep-rule for answer ai of gap sl's
// colour ask, read against the node's pool (gorge's, restricted units too):
// kept when the pool holds fewer of the answer's colour than the cost asks.
// Only the plain fixed-colour shape names its answers' colours here; a rider
// or restricted gap's ask keeps every answer, as the plain run does.
func (s *State) deferSubsumeKeeps(k *prep, sl Slot, ai int, costCol, pool *[kop.NumManaKinds]uint8) bool {
	if s.restrCensusTapsGapShape(k, sl) || s.restrTapsGapShape(k, sl) {
		return true
	}
	if _, ok := s.riderAbilityOf(k, sl); ok {
		return true
	}
	pc := s.blgGapPC(k, sl)
	mask := s.srcMaskAt(pc, sl)
	var col int
	if pcol, ok := tapsDeferPainCol(k, pc, mask); ok && int(s.pl[SeatIx(s.perms[sl].ctrl)].life) >= int(pc.painN) && ai == bits.OnesCount8(mask) {
		col = int(pcol)
	} else {
		for i := 0; i < ai; i++ {
			mask &= mask - 1
		}
		col = bits.TrailingZeros8(mask)
	}
	return costCol[col] > pool[col]
}

// budgetExit records that the walk ended on gorge's node budget and, when a
// rule-gap stall was seen on the way, declares the capacity full.
func (w *deferGapScratch) budgetExit(s *State) {
	w.limited = true
	if w.stall != 0 {
		s.capFull(w.stall)
	}
}

// deferGapWalk is one visit of gorge's search at the used subset: the replay
// on a fork and the play's offered check, then the children, then the pass.
// The context is explicit parameters, not a struct: s and k flow into probes
// that leak them, so a context struct holding them would be heap-allocated,
// and with it every array the caller passes beside it.
func deferGapWalk(s *State, k *prep, a Action, p uint8, order []Slot, tapsDef slotMask, cp *castRestPay, ans *[MaxPerm]uint8, pick *[MaxPerm]int8, dead *bool, noGrant, resolvePlay, twinCut bool, w *deferGapScratch, post bool) bool {
	n := len(order)
	{
		var nodePool [kop.NumManaKinds]uint8 // the pool this node's children's asks read (the subsume run)
		if w.subsume {
			nodePool = s.poolProj(p)
		}
		if w.nodes++; w.nodes > potentialScriptNodes {
			w.budgetExit(s) // past gorge's search budget: no script
			return false
		}
		if w.nu > 0 {
			t := s.fork()
			t.g.noGrant = noGrant
			var spill [kop.NumManaKinds]uint8
			replay := deferReplay(t, k, p, a, order, w.used[:w.nu], w.answers[:w.nu], tapsDef, &spill)
			if w.subsume {
				nodePool = t.poolProj(p)
			}
			charge, fits := s.deferChainCharge(k, t, w.nodes, order, w.used[:w.nu], replay, post, resolvePlay, twinCut)
			if w.stall == 0 {
				w.stall = t.g.stalled
			}
			if !fits {
				// Gorge's visit charges every clone until its budget saturates,
				// then refuses every later node, so a genuinely too-long pass
				// chain ends its whole search. The chain term here is a
				// longest-line *overestimate* (t.npend counts the payer
				// surviving every queued trigger; a payer that dies leaves a
				// shorter chain), not a floor, so a branch it rejects may still
				// fit gorge's walk. Saturating nodes would spread that
				// overestimate to every later sibling; keeping the budget
				// unsaturated is deliberate, an approximation that may still
				// find a line rather than a modelled clone count.
				s.unfork()
				w.budgetExit(s)
				return false
			}
			w.nodes += charge
			found, foreign := replay, false
			if replay {
				t.hideDeferred(k, p, a, tapsDef)
				foreign = t.foreignPendAsks(k, p, resolvePlay)
				if cp != nil {
					// the planned cast's payment: the node's floats price it,
					// gorge's sr.priced node check (potential_script.go)
					found = t.castRestPriced(k, p, cp.o, &cp.cost) && !foreign
				} else {
					var d Decision
					t.restrOptions(k, &d, p)
					complete := w.nu == n
					found = hasOption(&d, a) && (t.npend == 0 || complete || !resolvePlay || post) && !foreign
					if found && t.npend != 0 {
						t.resolvePrefixPlay(k, a, p, t.sp, resolvePlay)
						if t.over {
							found = false // the prefix's triggers end the payer: the branch is dead
						} else if resolvePlay {
							var d2 Decision
							t.restrOptions(k, &d2, p)
							found = hasOption(&d2, a)
						}
					}
				}
			}
			if !found && w.stall == 0 {
				w.stall = t.g.stalled
			}
			if found {
				*dead = false
				if t.rbAny(SeatIx(p)) || spill != ([kop.NumManaKinds]uint8{}) {
					for c := range spill {
						t.pl[SeatIx(p)].pool[c] -= spill[c] // restricted mana the byte spilled pays nothing either
					}
					*dead = !t.scriptCompletes(k, p, a)
				}
				if t.playLifeNotSurvived(k, p, a) {
					*dead = true // the prefix's damage left no life above the play's own life cost: the planner prices no such play
				}
				s.unfork()
				for i := range *pick {
					(*pick)[i] = -1
				}
				for u := 0; u < w.nu; u++ {
					(*pick)[w.used[u]], (*ans)[w.used[u]] = int8(w.used[u]), w.answers[u]
				}
				return true
			}
			j := w.used[w.nu-1]
			w.cut[j] = foreign && (w.answers[w.nu-1] == 0 || w.cut[j])
			dup := deferDuplicate(replay, post, &w.seen, t, p, w.used[:w.nu])
			s.unfork()
			if foreign {
				return false // the line ends at another player's ask: no deeper activation under it
			}
			if dup {
				return false // gorge's visited set (scriptStateKey): the same taps, pool and stack, reached in another order, is no node of its own
			}
		}
		if post {
			return false // the pass is the node's last line: nothing deeper under it
		}
		var tw twinWalk // the gaps this node already walked, for gorge's twin cut
		for i := 0; i < n; i++ {
			if w.cut[i] || w.removed[i] || deferInUsed(w.used[:w.nu], i) {
				continue
			}
			if twinCut && tw.skip(s, order, i) {
				continue // gorge's scriptTwin (potential_script.go activations): an indistinguishable earlier gap already walked this subtree
			}
			na := int(s.deferGapAnswers(k, order[i]))
			if add := deferAskNodes(na); add != 0 {
				// the activation's colour ask is a clone of gorge's search of its
				// own, beside the one per answer (scriptSearch.visit)
				if w.nodes += add; w.nodes > potentialScriptNodes {
					w.budgetExit(s)
					return false
				}
			}
			for ai := 0; ai < na; ai++ {
				if w.subsume && na > 1 && !s.deferSubsumeKeeps(k, order[i], ai, &w.costCol, &nodePool) {
					continue // the subsume run's ask keeps only the colours the pool lacks
				}
				w.used[w.nu], w.answers[w.nu] = int8(i), uint8(ai)
				w.nu++
				ok := deferGapWalk(s, k, a, p, order, tapsDef, cp, ans, pick, dead, noGrant, resolvePlay, twinCut, w, false)
				w.nu--
				if ok {
					return true
				}
			}
			if w.cut[i] {
				w.removed[i] = true // gorge's search goes on without i (scriptDFS cutAll)
			}
		}
		// the pass that lets the queued triggers resolve, after every deeper
		// activation failed (mana_scriptpost.go): the play is offered only
		// once the stack is clear again. The payment search's stop reads no
		// offer, so its queued triggers never stand between it and the play.
		if cp == nil && resolvePlay && w.nu > 0 {
			return deferGapWalk(s, k, a, p, order, tapsDef, cp, ans, pick, dead, noGrant, resolvePlay, twinCut, w, true)
		}
		return false
	}
}

// planTiered is plan with gorge's tiers: the sources a Taps watcher
// defers are no unit of the first pass. gorge's planner never lists them, but
// a play the planner then proves insufficient is paid through the potential
// play's prefix (they are census gaps, activated first), which taps them: so
// when no plan exists without them a second pass plans with them, in
// battlefield order like any source.
func (s *State) planTiered(k *prep, p uint8, pk *planner, exclude Obj) {
	if pk.plain {
		// a resolution payment's plan is gorge's plain census: no tiered
		// pass, whatever the board defers (mana_resolutionpay.go)
		s.plan(k, p, pk, exclude)
		return
	}
	if k.gen != nil && k.gen.anyComboOff && !pk.comboIn && !DebugComboAnyPlanned {
		s.planComboTiered(k, p, pk, exclude) // mana_comboanyplan.go
		return
	}
	s.planTiered0(k, p, pk, exclude)
}

// planTiered0 is planTiered's Taps-deferred tiers.
func (s *State) planTiered0(k *prep, p uint8, pk *planner, exclude Obj) {
	if k.gen == nil || !k.gen.anyTapsDefer || DebugNoTapsDefer {
		s.plan(k, p, pk, exclude)
		return
	}
	saved := *pk
	pk.try = true
	s.plan(k, p, pk, exclude)
	if pk.found {
		pk.try = saved.try
		return
	}
	*pk = saved
	pk.tapsIn = true
	s.plan(k, p, pk, exclude)
}

// StallRestrDefer: a non-spell payment only a Taps-deferred restricted mana
// source can fund (restrSrcAffords). gorge pays it through its prefix search,
// tapping the source first and choosing among its restricted abilities; the
// kernel's Taps-deferred gap route models one plain production.
const StallRestrDefer = 160

func init() { stallNames[StallRestrDefer] = "restrdefer" }

// selfTapsPainN is the damage of card c's own Pain-class "whenever CARDNAME
// becomes tapped" line (kgen admits one beside a printed mana ability only as
// the vetted "deals N damage to you" body and a plain {T} ability: City of
// Brass), and whether it has one.
func selfTapsPainN(c *kop.CardDef) (uint8, bool) {
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags&kop.TfSelf != 0 &&
			a.Trig.TapPlan == kop.TapPlanPain && a.AFlags&kop.AfLatent == 0 &&
			(a.Trig.Zone == kop.ZoneNone || a.Trig.Zone == kop.ZoneBattlefield) {
			return a.Trig.TapPainN, true
		}
	}
	return 0, false
}

// prepSelfTapsPain prices a card with a self Pain-class Taps line as a
// pain-only mana source (mana_pain.go): gorge's planner reads the trigger as
// the source's own consequence (pay.PaymentPlanSelfDamageTrigger), a
// last-resort unit priced by its damage, exactly a pain-only source's tier.
// The damage itself is the trigger's, queued by the tap (manaTapped) and
// resolved from the stack, so painDamage deals none for it.
func prepSelfTapsPain(c *kop.CardDef, pc *pcard) bool {
	if pc.manaAb < 0 || pc.painCol != 0 {
		return false
	}
	n, ok := selfTapsPainN(c)
	a := &c.Abilities[pc.manaAb]
	if !ok || !primaryTapAb(a) {
		return false
	}
	pc.painCol, pc.painN, pc.painOnly = a.ManaColors, n, true
	return true
}

// hideDeferred taps, on the search's node copy t, every untapped source of p
// the planner defers (planDeferred) that scriptTapsDefer does not model: a
// two-colour land under a Defer-class Taps watcher (Turbulent Fen beside Price
// of Glory). gorge's census holds it back as a gap, so the node's offer and
// pricing (scriptSearch.visit) never count its mana, but the plain walk would
// and list the play a tap early; the play's payment would then tap the
// source after the prefix's other taps, putting its trigger above the play.
//
// A source tapsDef holds is the search's own gap: availFor already leaves its
// unit out (availLeavesOut), and tapping it here would also take away the
// {T} a cost of its own pays (Cabal Pit's {B},{T}, sacrifice beside Price of
// Glory, CR 602.2b), so it stays untapped -- as does the play's own
// tap-cost source (deferSelfTapped), which the search's chain never holds.
//
// Only a source is hidden: a Taps watcher over every creature (Captain America,
// Living Legend) classes a creature with no mana ability as deferred too, and
// tapping that one on the node copy would take away the {T} an activation of its
// own pays (Echo, Perceptive Prodigy's {1},{T} copy, CR 602.2b) that gorge's
// census never touches.
func (t *State) hideDeferred(k *prep, p uint8, a Action, tapsDef slotMask) {
	if !t.deferLive(k) {
		return
	}
	self := t.deferSelfTapped(k, a)
	for b := t.ctrlM[SeatIx(p)].and(t.occ); b.any(); b = b.rest() {
		if sl := b.first(); sl != self && t.perms[sl].flags&pfTapped == 0 && !tapsDef.has(sl) && t.hasManaAbility(k, sl) && t.planDeferred(k, p, sl) {
			t.perms[sl].flags |= pfTapped
			t.touchSl(sl)
		}
	}
}

// hasManaAbility reports whether the permanent at sl has a mana ability, printed
// or granted: a source the planner could defer, as opposed to a permanent a Taps
// watcher merely matches.
func (s *State) hasManaAbility(k *prep, sl Slot) bool {
	for i := range k.t.Cards[s.cardAt(sl)].Abilities {
		if k.t.Cards[s.cardAt(sl)].Abilities[i].Kind == kop.AbMana {
			return true
		}
	}
	return s.manaGranted(k, sl)
}

// twinWalk records the gaps of order a walk has taken, so skip reports a gap
// indistinguishable (twinSlotOf) from an earlier one: gorge's scriptTwin cut.
type twinWalk struct {
	at [MaxPerm]int8
	n  int
}

// skip reports whether gap i of order twins one already walked; if not it
// records gap i as walked.
func (w *twinWalk) skip(s *State, order []Slot, i int) bool {
	for _, j := range w.at[:w.n] {
		if twinSlotOf(s, order[j], order[i]) {
			return true
		}
	}
	w.at[w.n] = int8(i)
	w.n++
	return false
}

// otherGapTwin is visit's other-gap branch with gorge's twin cut
// (scriptTwin, potential_script.go activations) applied to a Taps-deferred
// land: a source indistinguishable from one this pass already walked reaches
// no payer state its subtree did not, so it is no branch and no node. gorge
// runs the cut over every activation of the census, a land under Manabarbs
// among them (a Taps-deferred gap is a census gap there), while the branch
// beside the filter sources returned before the cut; the five-permanent
// walk of Unknown Shores beside four lands under Manabarbs then cost about
// twice gorge's nodes (TestFilterSearchNodeBudgetParity). The cut runs where
// visit's own does, on a fee-only or printed-activation search; the other
// gap classes keep their walk.
func (fs *filtSearch) otherGapTwin(k *prep, t *State, sl Slot, funded int, tried *[MaxPerm]Slot, ntried *int) bool {
	if (fs.feeOnly || isAbilityPlay(fs.a)) && t.scriptTapsDefer(k, fs.p).has(sl) {
		if fs.twinTried(k, t, sl, tried[:*ntried]) {
			return false
		}
		tried[*ntried] = sl
		*ntried++
	}
	return fs.otherGap(k, t, sl, funded)
}
