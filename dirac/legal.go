package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Priority options, in this order: the hand in hand order (land plays,
// then casts; a kicker variant follows its plain cast), flashback casts
// from the graveyard, activated abilities of the player's permanents in
// slot order, then pass. Mana abilities are not offered: payment is
// gorge's planner (autopay) at cast time. Unlike gorge there is no concede
// option.

func (s *State) sorcerySpeed(p uint8) bool {
	return p == s.active && (s.step == kop.StepMain1 || s.step == kop.StepMain2) && s.sp == 0 && s.cfgx&cfgxNoSorcery == 0
}

func (s *State) priorityOptions(k *prep, d *Decision) {
	p := s.prio
	pl := &s.pl[p]
	sorc := s.sorcerySpeed(p)
	var av avail
	haveAv := false
	// faces: a table with built faces or a face-down card (one gate keeps
	// the hand loop's registers as they were: facedown's perf blocker)
	faces := k.gen != nil && (k.gen.anyFaces || k.gen.anyFaceDown)
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		c := s.objs[o].card
		pc := &k.cards[c]
		if pc.land {
			if sorc && pl.landsPlayed < s.landLimit(k, p) {
				d.add(MakeAction(AkPlayLand, uint8(o), 0, 0))
				if faces && k.gen.anyFaces {
					s.offerFaceLand(k, d, o) // face_multiface.go
				}
			}
			if faces {
				// CR 702.37b: a face-down cast is not a land play
				s.offerFaceDownHand(k, d, p, o, sorc) // face_facedown.go
				if k.gen.anyFaces {
					// a land front's Adventure is cast from the hand (CR
					// 715.3a: gorge appendAdventureAlt inside its land branch)
					s.offerFaceCasts(k, d, p, o, sorc) // face_castface.go
				}
			}
			continue
		}
		if faces {
			if k.gen.anyFaces {
				if sorc && pl.landsPlayed < s.landLimit(k, p) {
					s.offerFaceLand(k, d, o)
				}
				s.offerFaceCasts(k, d, p, o, sorc) // face_castface.go
			}
			s.offerFaceDownHand(k, d, p, o, sorc) // CR 702.37b (face_facedown.go)
		}
		if k.gen != nil && k.gen.anyPlot && k.t.Cards[c].HasPlot && (sorc || DebugPlotInstant) {
			// a plot special action waits for sorcery timing even on an
			// instant (CR 702.170a); before the plain affordability gate,
			// like the warp cast, and before the non-instant timing gate
			// (a plot card is usually a sorcery or creature)
			if !haveAv {
				s.availFor(k, p, &av)
				haveAv = true
			}
			s.offerPlotHand(k, d, p, o, c, &av) // kw_plot.go
		}
		if k.gen != nil && k.gen.anySneak && k.t.Cards[c].HasSneak && p == s.active && s.sneakStep() {
			// the sneak window is instant speed in the caster's own
			// declare-blockers step (CR 702.190a): before the non-instant
			// timing gate, which also tests castForbidden after it
			if !haveAv {
				s.availFor(k, p, &av)
				haveAv = true
			}
			s.offerSneakHand(k, d, p, o, c, &av) // kw_sneak.go
		}
		if !pc.instant && !sorc && !s.castFlash(k, p, c) {
			if k.gen != nil && k.gen.anyMayFlash && k.t.Cards[c].HasMayFlash {
				// the timing gate refused the plain cast: the may-flash
				// cast is the one option it adds (kw_mayflash.go)
				if !haveAv {
					s.availFor(k, p, &av)
					haveAv = true
				}
				s.offerMayFlashHand(k, d, p, o, c, &av)
			}
			continue
		}
		if k.gen != nil && k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !haveAv {
			s.availFor(k, p, &av)
			haveAv = true
		}
		if k.gen != nil && k.gen.anyWarp && k.t.Cards[c].HasWarp {
			// before the plain affordability gate: a warp cast is offered
			// even when the mana cost is not payable (CR 702.185a)
			s.offerWarpHand(k, d, p, o, c, &av) // kw_warp.go
		}
		if k.gen != nil && k.gen.anyWebSling && k.t.Cards[c].HasWebSling {
			// under the plain cast's timing and prohibition gates, before
			// its affordability gate (CR 702.188a)
			s.offerWebSlingHand(k, d, p, o, c, &av) // kw_websling.go
		}
		if pc.altCost {
			s.offerAltHand(k, d, p, o, c, &av) // kw_impending.go
		}
		cost := &pc.cost
		var credited kop.ManaCost
		if k.gen != nil && k.gen.anyConvoke && s.tapPays(k, p, c, o, true) {
			credited = s.convokeCredit(k, p, c, o, pc.cost)
			cost = &credited
		}
		var adj kop.ManaCost
		base := *cost
		if s.statM.any() || k.gen != nil {
			av.delve = s.tbDelveCredit(k, p, c, o, true) // the faces' credit (gorge FeasibleAny's delve, CR 702.66a)
			adj = s.adjustFit(k, &av, p, c, o, *cost, false)
			cost = &adj
		}
		if k.gen != nil && k.t.Cards[c].SpellWaterbend != 0 {
			adj = s.waterbendSpellCredit(k, p, c, o, *cost) // the offer gate's waterbend credit (act_waterbend.go)
			cost = &adj
		}
		preDelve := cost
		var dc kop.ManaCost
		if k.gen != nil && k.gen.anyDelve && !av.tbLeafCredited(&base) {
			// an unresolved twobrid's credit priced the faces at the leaf
			// (adjustFit); the outer credit would take it a second time
			dc = s.delveOfferCreditHand(k, p, c, o, *cost)
			cost = &dc // (the either-or check below takes the pre-delve cost: it credits its own total)
		}
		var conv kop.ManaCost
		if k.gen != nil && k.gen.anyTypeCreat && s.convAnyType(k, p, o) {
			conv = anyType(*cost) // any mana pays a creature's pips (mana_convertcreature.go)
			cost = &conv
		}
		if !av.affords(cost, pipMaskOf(cost), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), cost, &av)) {
			if k.bargainRed {
				s.offerBargainOnly(k, d, p, o, c, &av) // the discount alone pays it (cost_reducespell.go)
			}
			continue
		}
		tgtOK, twOK := true, false
		if k.teamwork && k.t.Cards[c].Teamwork != 0 {
			// the plain and the teamworked cast announce different targets
			// (kw_teamwork.go): either may be castable alone
			tgtOK, twOK = s.teamworkTargets(k, c, p)
		} else {
			tgtOK = s.spellTargetsFeasible(k, c, p)
		}
		if !(tgtOK || twOK) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastableFrom(k, c, p, o, preDelve, &base, false, false, &av) {
			continue
		}
		if s.castHeldOut(o) {
			continue
		}
		// the kicked cast is gorge's lowered potential play even when the
		// plain one is left out (pair 8638: Grow from the Ashes paid only
		// through the imprisoned lands)
		plainOK := tgtOK && s.gorgeOffers(p, o, 0) && (cost.X == 0 || s.spellXBoundPayable(k, c, p, cost))
		if plainOK {
			d.add(MakeAction(AkCast, uint8(o), 0, 0))
		}
		if plainOK && k.gen != nil && k.gen.anyConspire && s.conspireOffer(k, p, c, o) && s.casualtyOffers(p, o) {
			// the conspired cast (kw_conspire.go): beside the plain cast; like the casualty cast, gorge offers it only against a pool that pays
			d.add(MakeAction(AkCastConspire, uint8(o), 0, 0))
		}
		if twOK && s.teamworkOffer(k, p, c) && s.casualtyOffers(p, o) {
			// the teamworked cast (kw_teamwork.go): the same action kind
			d.add(MakeAction(AkCastCasualty, uint8(o), 0, 0))
		}
		if k.casualty && s.casualtyOffer(k, p, c, o) && s.casualtyOffers(p, o) {
			// the casualty cast (kw_casualty.go): the plain cast's price, one more option
			d.add(MakeAction(AkCastCasualty, uint8(o), 0, 0))
		}
		if k.bargain && s.bargainOffer(k, p, c) && s.bargainOffers(k, p, o) {
			// the bargained cast (kw_bargain.go): the same action kind
			d.add(MakeAction(AkCastCasualty, uint8(o), 0, 0))
		}
		if k.gen != nil && k.gen.anyOptCost && tgtOK && s.optCostOffer(k, p, c, o) {
			// the optional-cost cast (cost_optional.go): the kicked action
			d.add(MakeAction(AkCast, uint8(o), 0, 1))
		}
		if k.gen != nil && k.gen.anyBestow && k.t.Cards[c].HasBestow {
			s.offerBestowHand(k, d, p, o, c, &av) // kw_bestow.go
		}
		if pc.kw&kop.KwKicker != 0 || k.offspring && offspringOffer(k, c) {
			kc := addCost(pc.cost, k.t.Cards[c].Kicker)
			if s.statM.any() || k.gen != nil {
				kc = s.adjustCost(k, p, c, o, kc) // the reduction applies to the total (CR 601.2f)
			}
			if k.gen != nil && k.gen.anyTypeCreat && s.convAnyType(k, p, o) {
				kc = anyType(kc) // the kicked total under Vizier's conversion too (mana_convertcreature.go)
			}
			if s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &kc, &av) && s.gorgeOffersKicked(p, o) && (!k.sacMana || s.kickWitness(k, p, c, o, &kc)) && (k.gen == nil || !k.gen.anyKickCost || s.kickerCostOffer(k, p, c, o)) {
				d.add(MakeAction(AkCast, uint8(o), 0, 1))
			}
		}
	}
	if s.cfg&cfgCommander != 0 {
		if !haveAv {
			s.availFor(k, p, &av)
			haveAv = true
		}
		s.offerCommanderCast(k, d, p, &av)
	}
	// flashback
	var fbLo, fbHi *objBits // nil for a pool without a flashback grant: no State access below
	if fbi := k.fbIx(); int(fbi) < len(s.cold.mo) {
		fbLo, fbHi = &s.cold.mo[fbi], &s.cold.mo[fbi+1]
	}
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		pc := &k.cards[c]
		granted := pc.kw&kop.KwFlashback == 0 && fbLo != nil && (fbLo.has(o) || fbHi.has(o))
		if pc.kw&kop.KwFlashback == 0 && !granted || (!pc.instant && !sorc && !s.castFlash(k, p, c)) {
			continue // (cast as though it had flash: High Fae Trickster)
		}
		if k.gen != nil && k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !haveAv {
			s.availFor(k, p, &av)
			haveAv = true
		}
		fc := k.t.Cards[c].AltCost
		if granted {
			fc = k.t.Cards[c].Cost // the granted flashback costs the card's mana cost
		}
		if s.statM.any() || k.gen != nil {
			fc = s.adjustCost(k, p, c, o, fc)
		}
		fbGrant := granted // the cast pays a granted instance (no printed non-mana part)
		if !granted {
			if two, amb, g := s.fbInstances(k, p, o); amb {
				continue
			} else if two && g {
				fc = s.adjustCost(k, p, c, o, k.t.Cards[c].Cost)
				fbGrant = true
			}
		}
		fc = s.delveOfferCredit(k, p, c, o, false, fc)
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, &av) || !s.spellTargetsFeasibleFrom(k, c, p, o) || k.gen != nil && !s.spellCostsFeasible(k, c, p, o) {
			continue // (an additional cost too: Thrill of Possibility's discard with an empty hand)
		}
		if fb := &k.t.Cards[c].FbCost; fb.Flags != 0 && !fbGrant && !s.pickCostPayable(k, p, fb, o) {
			continue // a non-mana flashback cost (gorge castable's Sac / tapXType census)
		}
		if k.t.Cards[c].FbBehold && !fbGrant && !s.beholdPayable(k, p, c, o) {
			continue // the flashback cost's behold part (gorge castable's Behold census)
		}
		if s.castHeldOut(o) {
			continue // reversed twice in a row (Eaten Alive's unpayable part, pair 2359)
		}
		if !s.flashbackPartsDisjoint(k, p, c, fc) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && tgtDepReduce(&k.t.Cards[c]) && severalTargets(k, c) && !s.FeatPoolPays(p, o, 2) {
			// gorge's potential planner prices a target-conditional reduction on
			// a mode spell choosing several targets with no target: the full cost
			s.g.noTgtRed = true
			full := s.adjustCost(k, p, c, o, k.t.Cards[c].Cost)
			if !granted {
				full = s.adjustCost(k, p, c, o, k.t.Cards[c].AltCost)
			}
			s.g.noTgtRed = false
			if !av.affords(&full, pipMaskOf(&full), 0) {
				continue
			}
		}
		d.add(MakeAction(AkCast, uint8(o), 0, 2))
	}
	if k.gen != nil && k.gen.anyHarmonize {
		s.offerHarmonizeCasts(k, d, p, sorc, &av) // kw_harmonize.go
	}
	if k.gen != nil && k.gen.anyBeam {
		s.offerBeamCasts(k, d, p, sorc, &av) // kw_beammeup.go
	}
	if k.mayhem {
		s.offerMayhemCasts(k, d, p, sorc, &av) // kw_mayhem.go
	}
	if faces {
		s.offerAdventureRecasts(k, d, p, sorc) // face_castface.go
	}
	if k.gen != nil && k.gen.anyAirbend {
		s.offerAirbendCasts(k, d, p, sorc) // act_airbend.go (gorge offers it before the warp recast)
	}
	if k.gen != nil && k.gen.anyWarp {
		s.offerWarpRecasts(k, d, p, sorc) // kw_warp.go
	}
	if k.gen != nil && k.gen.anyPlot {
		s.offerPlotCasts(k, d, p, sorc) // kw_plot.go
	}
	if k.gen != nil && k.gen.anyParadigm {
		s.offerParadigm(k, d, p, sorc, &av) // kw_paradigm.go
	}
	if k.gen != nil && k.gen.anyPrepare {
		s.offerPreparedCasts(k, d, p, sorc) // face_prepare.go
	}
	// activated abilities
	m := s.ctrlM[p]
	for m.any() {
		sl := m.first()
		m = m.rest()
		cid := s.cardAt(sl)
		if k.cards[cid].nAct == 0 || s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		cd := &k.t.Cards[cid]
		for ai := range cd.Abilities {
			a := &cd.Abilities[ai]
			if a.Kind != kop.AbActivated {
				continue
			}
			if !haveAv {
				s.availFor(k, p, &av)
				haveAv = true
			}
			if a.Cost.Mana.X > 0 && s.cfg&cfgGorgeUnpriced != 0 {
				m := a.Cost.Mana
				if k.gen != nil && k.gen.anyPowerUp && a.Cost.Flags&kop.CostPowerUp != 0 {
					m = s.powerUpReduced(k, s.perms[sl].obj, m) // CR 702.193b: gorge prices the reduced cost (legal_walk_battlefield.go)
				}
				if k.gen != nil && k.gen.anyAbReduce {
					m = s.abReduced(k, a, sl, p, m) // the cost statics compose after it
				}
				if !s.poolPaysAct(k, p, &m, payDesc{class: pdAct, card: cid, src: sl}) {
					// gorge's planner declines an {X} activation (PlanCostDetail
					// cost:x, Domesticated Hydra's {X}{G}{G}{G}: Monstrosity X,
					// Fiend Artisan's hybrid {X}): it is offered only when the
					// floating pool alone pays it (Config.GorgeUnpriced). A hybrid
					// cost without {X} is planned (each pip resolved to a colour:
					// Puresight Merrow's {W/U}{Q}, kverify-c4).
					continue
				}
			}
			if a.Gate.Ctr == kop.CtrMaxSpeed && !s.maxSpeedPoolOK(p, a) {
				continue // a max-speed grant is offered from the floating pool alone (kw_maxspeed.go)
			}
			if s.canActivate(k, sl, ai, a, sorc, &av) {
				d.add(MakeAction(AkActivate, uint8(s.perms[sl].obj), 0, uint8(ai)))
			}
		}
	}
	if k.gen != nil {
		s.priorityGen(k, d, p, sorc, &av, haveAv)
		if k.gen.anyFaceDown {
			s.offerTurnFaceUp(k, d, p) // CR 708.6: special action, no stack
		}
		if k.anyRoom && sorc {
			s.offerUnlocks(k, d, p) // CR 709.5e: special action (face_rooms.go)
		}
		var capSpent Decision // the plays grantCap's grant search refused within gorge's budget
		if len(k.gen.grantPC) != 0 && s.g.availX == 0 {
			s.grantCap(k, d, p, &capSpent)
		}
		if s.scriptAny(k) && s.g.availX == 0 {
			s.scriptOffers(k, d, &capSpent, p)
		}
	}
	if faces {
		// after the may-play walks: one land play per object (CR 715.3d)
		s.offerAdventureLandPlays(k, d, p, sorc) // cast_adventureland.go
	}
	if k.gen != nil && k.gen.anySplitSecond && s.splitSecondHolds(k) {
		s.filterSplitSecond(k, d) // kw_splitsecond.go
	}
	d.add(MakeAction(AkPass, 0, 0, 0))
}

// addCost sums two costs. It does not merge XRed (the reduction excess
// adjustCost records): every caller adds kicker and alternative costs
// BEFORE adjustCost, so only a composed cost carries it.
func addCost(a, b kop.ManaCost) kop.ManaCost {
	a.Generic += b.Generic
	for i := range a.Pips {
		a.Pips[i] += b.Pips[i]
	}
	a.X += b.X
	for i, h := range b.Hyb {
		if h != 0 {
			j := hybN(&a)
			if j >= len(a.Hyb) {
				hybOverflow() // mana_hybridn.go
			}
			a.Hyb[j] = h
			if b.HybOrd&(uint8(1)<<i) != 0 {
				a.HybOrd |= uint8(1) << j
			} else {
				a.HybOrd &^= uint8(1) << j
			}
		}
	}
	return a
}

// hybN counts c's unresolved hybrid pips.
func hybN(c *kop.ManaCost) int {
	switch {
	case c.Hyb[0] == 0:
		return 0
	case c.Hyb[0] == kop.HybTwobrid:
		return c.TbN()
	}
	n := 0
	for _, h := range c.Hyb {
		if h != 0 {
			n++
		}
	}
	return n
}

func pipMaskOf(c *kop.ManaCost) uint8 {
	var m uint8
	for i, n := range c.Pips {
		if n > 0 {
			m |= 1 << i
		}
	}
	return m
}

// canActivate checks an activated ability's timing, non-mana costs, mana
// affordability and target feasibility.
func (s *State) canActivate(k *prep, sl Slot, ab int, a *kop.Ability, sorc bool, av *avail) bool {
	if k.gen != nil && ab != 255 && (!ownActivatable(a) || s.genLost(k, sl)) {
		return false
	}
	p := &s.perms[sl]
	if k.gen != nil && k.gen.anyNetLock && s.netLocked(p.obj) {
		return false // Braided Net's lock: no ability of a tapped, locked permanent (act_lockpermanent.go)
	}
	if k.gen != nil && k.gen.anyCantAct && s.actForbiddenAb(k, sl, a.Kind == kop.AbMana, a.Cost.Flags&kop.CostLoyalty != 0) {
		return false // a CantBeActivated lock (cast_cantactivate.go)
	}
	if a.Cost.Flags&kop.CostSacSelf != 0 && s.sacBlocked(k, sl) {
		return false // a self-sacrifice its source can't pay (static_cantsacrifice.go)
	}
	if k.gen != nil && (k.gen.anyWaterbend || k.gen.anyXBoundAb) && s.castHeldOut(p.obj) {
		// gorge holds a card's activation out after its second no-progress
		// abort in a row (cast_commit.go abortCast, CR 733.2)
		return false
	}
	f := a.Cost.Flags
	if f&kop.CostSorcery != 0 && !sorc {
		return false
	}
	if a.Cost.Host != 0 && ab != 255 {
		return false // a granting host's cost part: only a granted ability has a host (gorge leaves the referent unbound)
	}
	if a.Evidence != 0 && s.graveTotalMV(k, p.ctrl) < int(a.Evidence) {
		return false // CollectEvidence<N> needs a graveyard worth N (gorge pay.EvidencePayable)
	}
	if k.anyGate && !s.gateOK(sl, a) {
		return false
	}
	if f&kop.CostTap != 0 {
		if p.flags&pfTapped != 0 {
			return false
		}
		if p.flags&pfSick != 0 && s.isCreature(k, sl) && s.kw(k, sl)&kop.KwHaste == 0 {
			return false
		}
	}
	if f&kop.CostUntapSelf != 0 && !s.untapCostOK(k, sl) {
		return false
	}
	if f&kop.CostRemoveCtr != 0 && p.ctrN(s, int(a.Cost.Ctr)) < int(a.Cost.N) {
		return false
	}
	if f&kop.CostSubAmong != 0 && s.subAmongTotal(k, p.ctrl, &a.Cost, sl) < int(a.Cost.N) {
		return false // kw_training.go
	}
	if f&kop.CostPayLife != 0 && int(s.pl[p.ctrl].life) < int(a.Cost.N) {
		return false
	}
	if f&kop.CostDiscard != 0 {
		// a filtered discard (CostDiscardFiltered) counts only the hand
		// cards matching Cost.Filter (cost_discardfiltered.go)
		n := int(s.pl[p.ctrl].handN)
		if f&kop.CostDiscardFiltered != 0 {
			n = s.discardCandN(k, p.ctrl, a.Cost.Filter, NoObj)
		}
		if n < int(a.Cost.N) {
			return false
		}
	}
	if f&kop.CostSac != 0 && s.countSac(k, p.ctrl, a.Cost.Filter, sl) < int(a.Cost.N) {
		return false
	}
	if f&(costObjFlags|kop.CostPayEnergy) != 0 && !s.costObjPayable(k, p.ctrl, &a.Cost, p.obj, NoObj) {
		return false
	}
	if f&kop.CostTapX != 0 && s.countUntapped(k, p.ctrl, a.Cost.Filter, sl)-s.selfTapCounted(k, a, sl) < int(a.Cost.N) {
		return false
	}
	if f&kop.CostTapPower != 0 && !s.tapPowerPayable(k, a, sl) {
		return false
	}
	if f&kop.CostOncePerTurn != 0 && ab < 8 && p.actUsed&(1<<ab) != 0 {
		return false
	}
	if a.Kind == kop.AbActivated && a.Trig.Cond != 0 && !s.activateCondHolds(k, sl, a) {
		return false
	}
	if f&kop.CostLoyalty != 0 && (!sorc || p.flags&pfLoyaltyUsed != 0) {
		return false
	}
	if f&kop.CostOncePerGame != 0 && p.flags&pfOnceGame != 0 {
		return false
	}
	mana := a.Cost.Mana
	if k.gen != nil && k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) {
		mana = s.ownReducedBF(k, a, sl, p.ctrl)
	}
	if k.gen != nil && k.gen.anyPowerUp && f&kop.CostPowerUp != 0 {
		// CR 702.193b: the offer walk and the affordability check read the
		// reduced cost (gorge legal_walk_battlefield.go:414/553), and the
		// cost statics compose after it.
		mana = s.powerUpReduced(k, p.obj, mana)
	}
	if k.gen != nil && k.gen.anyAbReduce {
		mana = s.abReduced(k, a, sl, p.ctrl, mana) // after the ability's own and Power-up reductions
	}
	if k.gen != nil && k.gen.anyPowerUp && f&kop.CostPowerUp != 0 {
		// The autopay planner prices the PRINTED cost composed with the cost
		// statics (planAbilityPayment calls offerCostFor, which folds the
		// RaiseCost/ReduceCost statics and the ability's own ReduceCost$; it
		// never calls powerUpReduceCost), and an ability whose composed
		// printed cost it cannot witness is not offered (powerUpWitness; gorge seed-1
		// Hercules: {4}{G} reduced to {2}, three untapped Forests, no offer).
		// The floating pool paying the reduced cost is enough: the plan is
		// then a pool choice. So: offered iff the pool pays the reduced cost
		// or the sources pay the printed one AFTER the cost statics
		// (powerup_cost_static_planner_prices_reduced_print_torch: Hulk, Gamma
		// Goliath's {3} Power-up reduction makes Human Torch, Johnny Storm's
		// {6}{R} affordable from the sources at {3}{R}); the charge is always
		// the reduced one (buildCostCharge), and the plan/charge mismatch
		// falls back to the manual mana window (powerUpPlanRejected,
		// mana_window.go).
		if !s.poolPays(p.ctrl, &mana) && !s.powerUpWitnessAffords(k, sl, a, p.ctrl, av) {
			return false
		}
	}
	if mana.MV() > 0 || mana.X > 0 {
		manaCost := mana
		if f&kop.CostWaterbend != 0 && k.gen != nil && k.gen.anyWaterbend {
			manaCost = s.waterbendCredit(k, p.ctrl, sl, a, manaCost)
		}
		if k.phyMana && manaCost.Phyrexian() {
			// offered while some colour / life mix of its pips is payable
			// (mana_phyrexian.go)
			if !s.phyActAffords(k, sl, a, p.ctrl, manaCost, av) {
				return false
			}
		} else if !s.abilityAffords(k, sl, a, &manaCost, av) {
			return false
		}
	}
	return s.abilityTargetsFeasible(k, a, p.ctrl, sl, 0xff) && s.xBoundPayable(k, a, 0, p.ctrl, sl, &mana)
}

// abilityAffords reports whether av pays the mana cost cost of ability a of
// slot sl: the source's own tap cannot fund its cost's mana (av2), and a
// tap/sacrifice mana cost must stay disjoint from the sources it spends.
func (s *State) abilityAffords(k *prep, sl Slot, a *kop.Ability, cost *kop.ManaCost, av *avail) bool {
	f := a.Cost.Flags
	pm := pipMaskOf(cost)
	if f&kop.CostTap != 0 && s.canTapForMana(k, sl) {
		// the source can't also tap for its own cost's mana
		var av2 avail
		av2 = *av
		pc := &k.cards[s.cardAt(sl)]
		if k.gen != nil {
			pc = s.manaPCGen(k, sl)
		}
		s.removeSrc(k, &av2, sl, pc)
		if !av2.affords(cost, pm, 0) && !(k.restrSrc && s.actRestrAffords(k, sl, cost, &av2, sl)) && (!s.rbAny(s.perms[sl].ctrl) || !s.actPoolAffords(k, sl, cost, &av2)) {
			return false
		}
		if f&kop.CostTapX != 0 || f&kop.CostSac != 0 && k.sacMana {
			return s.costManaDisjoint(k, sl, a, &av2, pm)
		}
		return true
	}
	if f&kop.CostSacSelf != 0 && k.sacMana && s.sacSelfMana(k, sl) {
		return s.abilityAffordsSacSelf(k, sl, a, cost, av) // filter_combatwords.go
	}
	// a source that taps for mana only under a spend restriction (Isolinear
	// Processor) is outside the ordinary walk above, but its own {T} still
	// takes its mana out of the restricted sources this activation may spend.
	skip := NoSlot
	if f&kop.CostTap != 0 {
		skip = sl
	}
	if !av.affords(cost, pm, 0) && !(k.restrSrc && s.actRestrAffords(k, sl, cost, av, skip)) && (!s.rbAny(s.perms[sl].ctrl) || !s.actPoolAffords(k, sl, cost, av)) {
		return false
	}
	if f&kop.CostTapX != 0 || f&kop.CostSac != 0 && k.sacMana {
		return s.costManaDisjoint(k, sl, a, av, pm)
	}
	return true
}

// countUntapped counts p's untapped permanents matching f.
func (s *State) countUntapped(k *prep, p uint8, f kop.FilterID, src Slot) int {
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped == 0 && s.matchPerm(k, f, sl, p, src) {
			n++
		}
	}
	return n
}

// spellCostsFeasible checks a spell's additional pick costs (its spell
// ability's discard / sacrifice / tap / life costs) for p casting it as
// object o (from hand, o itself is not a discard candidate).
func (s *State) spellCostsFeasible(k *prep, c CardID, p uint8, o Obj) bool {
	ac := spellAddCost(k, c)
	if ac.Flags == 0 && !k.t.Cards[c].HasAltAdd && !k.t.Cards[c].BeholdExile {
		return true
	}
	// the cards a discard cost can take: the hand, less the card cast from
	// it (a flashback Thrill of Possibility is not in the hand: pair 1370)
	others := int(s.pl[p].handN)
	if s.objs[o].zone == kop.ZoneHand && s.objs[o].owner() == p {
		others--
	}
	if s.pickCostPayable(k, p, ac, NoObj) && (ac.Flags&kop.CostDiscard == 0 || others >= int(ac.N)) && s.beholdPayable(k, p, c, o) {
		return true
	}
	cd := &k.t.Cards[c]
	// the either-or part's discard cannot take the card being cast either
	// (Bitter Triumph as the last card in hand)
	return cd.HasAltAdd && s.pickCostPayable(k, p, &cd.AltAddCost, o)
}

// gorgeOffers: under Config.GorgeAutopaySurface, gorge's priority list
// has p's cast of o in mode (bit 0 kicked, bit 1 flashback): its planner
// prices the spell, or the floating pool alone pays it. Under
// Config.GorgeLowering a plain cast the planner does not price is also
// there when the oracle can lower its potential play (gorgeLowers).
func (s *State) gorgeOffers(p uint8, o Obj, mode uint8) bool {
	if s.g.availX&availWalk != 0 {
		return true // gorge's potential walk (grantCap) lists every play; scriptOffers' any-mana walk keeps the surface (pair 84743)
	}
	if s.cfg&cfgGorgeAutopay == 0 && (s.cfg&cfgGorgeLowering == 0 || mode != 0) {
		return true
	}
	if cost := &s.k().t.Cards[s.objs[o].card].Cost; s.g.availX&(availAny|availGaps) != 0 && cost.X != 0 || s.g.availX&availAny != 0 && cost.Hyb[0] != 0 && cost.Hyb[0] != kop.HybTwobrid && !cost.Phyrexian() {
		// an {X} cast is one of gorge's potential plays (its planner plans
		// the composed cost), so it takes one of the scripted prefix searches
		// (scriptOffers; pair 16186); a hybrid one does when no relaxed proof
		// runs, the census being unrelaxable
		return true
	}
	if mode&1 != 0 || s.FeatPlannedCast(p, o, mode) || s.FeatPoolPays(p, o, mode) {
		return true
	}
	return s.cfg&cfgGorgeAutopay == 0 && s.gorgeLowers(p, o, false)
}

// flashbackPartsDisjoint: the flashback cast of card c at cost, whose additional cost sacrifices
// or taps a permanent, is still priced once the permanents that part takes are held
// back from the mana (gorge planReserving, as gorgeLowers prices the plain cast): a
// source that sacrifices itself can use up what the part must sacrifice, and a
// creature tapped for mana is not also the one the part taps (CR 601.2g-h).
//
//go:noinline
func (s *State) flashbackPartsDisjoint(k *prep, p uint8, c CardID, cost kop.ManaCost) bool {
	ac := spellAddCost(k, c)
	if ac.Flags&kop.CostTapX == 0 && !(ac.Flags&kop.CostSac != 0 && k.sacMana) {
		return true
	}
	var av avail
	s.availFor(k, p, &av)
	return s.costPartDisjoint(k, p, NoSlot, ac, &cost, &av, pipMaskOf(&cost))
}

// gorgeOffersKicked: under Config.GorgeAutopaySurface a kicked or Offspring cast
// is still one of gorge's potential plays (pair 8638: Grow from the Ashes paid
// only through the imprisoned lands).
func (s *State) gorgeOffersKicked(p uint8, o Obj) bool {
	if s.cfg&cfgGorgeAutopay == 0 {
		return s.gorgeOffers(p, o, 1)
	}
	return true
}

// gorgeOffersMay is gorgeOffers for a cast through a permission: under
// either oracle surface gorge lowers it when its planner does not price it
// (the autopay surface leaves out plain casts only), so it is offered when
// the planner prices it, the pool pays it, or the potential planner does.
func (s *State) gorgeOffersMay(p uint8, o Obj) bool {
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) == 0 {
		return true
	}
	if s.FeatPlannedCast(p, o, 0) {
		return true
	}
	return s.FeatPoolPays(p, o, 0) || s.gorgeLowers(p, o, true)
}

// offerSourcesAfford: p's sources pay cost through the offer gate -- the
// script gaps the gate sees included, and the restricted sources the spell
// may spend (restrFallback), as a cast from the hand reads them.
func (s *State) offerSourcesAfford(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	wasX := s.g.availX
	// The printed sources alone: a cost they pay is priced by gorge's planner
	// whatever a census gap would cost beside them. The gaps' view below charges
	// each paid filter's fee, so it can read short a cost the printed sources
	// pay (Starfleet Academy's {5},{T} makes nothing with no creature, yet its
	// fee came off the five lands that pay Guidance Failure's {3}{U}).
	var pl avail
	s.availFor(k, p, &pl)
	if pl.affords(cost, pipMaskOf(cost), 0) {
		return true
	}
	s.g.availX |= availGaps
	var av avail
	s.availFor(k, p, &av)
	rf := s.restrFallback(k, p, c, s.restrOriginOfObj(o), cost, &av)
	s.g.availX = wasX
	return av.affords(cost, pipMaskOf(cost), 0) || rf
}

// gorgeLowers: gorge's potential-play planner prices p's plain cast of o,
// a spell its cast planner does not (FeatPlannedCast), from the spell's
// composed cost (rules potentialModeCastPlan), and the cast is offered
// once that witness is tapped (potentialWitnessOffers). For a
// target-conditional reduction (Luminous Rebuke) the composition prices
// the cheapest legal target; a spell choosing several stays at the full
// cost in that composition (potentialModeCastPlan's TargetMax$ decline).
// mayPlay is a cast through a play permission, whose offer gate is not the
// potential planner: mayPlayAffordable retries offerCastable against the
// legal targets (CR 601.2f), and that retry prices an assignment of up to
// the spell's TargetMax$ targets, so one matching target takes the
// reduction (This Town Ain't Big Enough, the one such card in the corpus).
// For an either-or additional cost (Eaten Alive) it prices the spell's mana
// cost alone when a part with no mana can be paid (a creature to sacrifice),
// and otherwise with a part that has mana.
func (s *State) gorgeLowers(p uint8, o Obj, mayPlay bool) bool {
	k := s.k()
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	if s.contribPays(k, p, c, o, s.objs[o].zone == kop.ZoneHand) {
		return false // gorge's potential planner refuses a convoke cost
	}
	if s.waterbendOwnPart(k, p, c, o) != 0 {
		return false // ... and a waterbend one (potentialModeCastPlan, "shape:contribution")
	}
	if cd.Cost.Hyb[0] == kop.HybTwobrid {
		return false // ... and a twobrid one ("cost:twobrid", pay.PlanCostDetail)
	}
	if cd.Cost.X != 0 {
		return s.xWitnessOffered(k, p, c, o)
	}
	if cd.HasAltAdd {
		cost := s.altAddPlanCost(k, p, c, o)
		if s.g.availX&availGaps != 0 {
			return s.affordsFor(k, p, &cost) // with the mana a scripted prefix makes (scriptOffers)
		}
		if s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}) {
			return true
		}
		// the planner's census has no granted source, so a cost only a granted ability
		// pays is a census gap: gorge's prefix search taps it first and the cast is
		// offered once the mana floats (Deadly Precision's {B} from Great Divide Guide's
		// grant, the guide also the creature the cast sacrifices; CR 601.2g before 601.2h)
		return k.gen != nil && (len(k.gen.grantPC) != 0 || s.scriptSourceLive(k, p)) && s.affordsFor(k, p, &cost)
	}
	if ac := spellAddCost(k, c); ac.Flags&kop.CostTapX != 0 || ac.Flags&kop.CostSac != 0 && k.sacMana {
		// a permanent tapped for the part is no mana source, and a source that
		// sacrifices itself can use up what the part must sacrifice (CR 601.2g-h;
		// gorge planReserving)
		// the planner's census, not the unless / ward window's: a pain source's
		// life-paid colour pays the mana beside the part (Haunted Screen; CR 601.2g-h)
		cost := s.featCost(k, p, c, o, k.cards[c].cost)
		if !s.spellPartDisjoint(k, p, c, o, ac, &cost) {
			return false
		}
	}
	if ac := spellAddCost(k, c); ac.Flags&costObjFlags != 0 || k.gen != nil && k.gen.anyEvTargets && spellEvidenceTargets(cd) {
		// an exile / forage / return / blight / collect-evidence part: the cast
		// planner declines the shape, and the potential planner prices it only
		// when the printed sources pay the mana (potentialPlayVerdictShared), with
		// no prefix search for a granted source beside it (Abhorrent Oculus'
		// exile; Urgent Necropsy's evidence beside Insidious Roots' grant)
		cost := s.featCost(k, p, c, o, k.cards[c].cost)
		if s.g.mayPlayAny {
			cost = anyType(cost)
		}
		return s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}) ||
			mayPlay && s.mayPlayPrefixPays(k, p, c, o) // (a may-play cast is composed, not declined: Urgent Necropsy from Outrageous Robbery's exile)
	}
	if s.spellTgtDep(k, p, c, o) {
		if mayPlay {
			// mayPlayAffordable prices the cast in one of two ways. A spell
			// choosing several targets is priced by offerCastable's retry at
			// the cheapest legal target assignment (CR 601.2f), and only the
			// floating pool pays that retry; the first arm is the no-target
			// full price its sources pay. A single-target spell's composed
			// price already takes the reduction, so its sources pay it.
			several := severalTargets(k, c)
			if several {
				s.g.noTgtRed = true
			}
			cost := s.featCost(k, p, c, o, k.cards[c].cost)
			s.g.noTgtRed = false
			if s.g.mayPlayAny {
				cost = anyType(cost)
			}
			if s.offerSourcesAfford(k, p, c, o, &cost) {
				return true
			}
			if !several {
				return false
			}
			cost = s.featCost(k, p, c, o, k.cards[c].cost)
			if s.g.mayPlayAny {
				cost = anyType(cost)
			}
			// the retry taps no source: the pool (and the restricted mana it
			// may spend) pays the reduced price on its own
			a := s.poolAvailPlus(p, nil)
			if s.rbAny(p) {
				s.restrAvail(k, p&1, c, s.restrOriginOfObj(o), &a)
			}
			return a.affords(&cost, pipMaskOf(&cost), 0)
		}
		// gorge's potential planner prices the cheapest legal target set for
		// a target-dependent price, including a spell choosing several
		// targets (potentialTargetPricedPlan): one matching target in the
		// announced set applies a flat "if it targets ..." reduction, so the
		// composed cost already carries it (costTargetOK finds the matching
		// candidate).
		cost := s.featCost(k, p, c, o, k.cards[c].cost)
		if s.g.mayPlayAny {
			cost = anyType(cost)
		}
		// the planner's census (potentialTargetPricedPlan): no granted source
		// (Imprisoned in the Moon's {C}), and a painful source makes its
		// colours while one point of damage is not lethal (Starting Town's W)
		return s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)})
	}
	if cd.BeholdExile || k.gen != nil && k.gen.anyOptCost && optCostCard(k, c) {
		if s.plannerPaysCast(k, p, c, o) { // a shape lowered only on the composed plan's witness
			return true
		}
		// a cast through a play permission is composed (mayPlayPrefixPays),
		// and a prefix taps a census gap (Flamebraider's restricted two
		// mana) before it prices the cast again, as for an either-or
		// additional cost above (CR 601.2b, 601.2f, 601.2g). A plain hand
		// cast is not: gorge's potential planner refuses it at
		// "shape:additional_cost" or "shape:optional_cost" before any mana
		// is priced, so its prefix search never runs for one (Bloom Tender
		// beside Champions of the Perfect, the lone Elf the cost also exiles).
		return mayPlay && s.mayPlayPrefixPays(k, p, c, o)
	}
	return true
}

// scriptSourceLive: p has a source the planner's census leaves a gap (an
// each-colour source, restricted mana, a tap-another or counter source), which
// gorge's prefix search taps before it prices the play again (Bloom Tender's
// colours beside a Swamp pay Bogslither's Embrace's {1}{B}; CR 601.2b, 601.2f).
func (s *State) scriptSourceLive(k *prep, p uint8) bool {
	return s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapXModelled(k, p)).or(s.scriptCtrModelled(k, p)).or(s.scriptCtr(k, p)).any()
}

// altAddPlanCost is the mana gorge's potential planner prices an either-or additional
// cost's spell at (potentialAltAddPlan): the spell's own cost when a part with no mana
// can be paid, and otherwise with the part that has mana.
func (s *State) altAddPlanCost(k *prep, p uint8, c CardID, o Obj) kop.ManaCost {
	base := k.cards[c].cost
	if !s.altAddFreePart(k, p, c, o) {
		base = addCost(base, s.altAddManaPart(k, c))
	}
	cost := s.featCost(k, p, c, o, base)
	if s.g.mayPlayAny {
		cost = anyType(cost)
	}
	return cost
}

func manaEmpty(m *kop.ManaCost) bool { return m.MV() == 0 && m.X == 0 }

// severalTargets: card c's spell chooses more than one target.
func severalTargets(k *prep, c CardID) bool {
	t := tspec(k, c, spellAb(k, c), 0)
	return t != nil && t.Max > 1
}

// altAddFreePart: card c's either-or additional cost has a part with no mana
// that can be paid (a creature to sacrifice).
func (s *State) altAddFreePart(k *prep, p uint8, c CardID, o Obj) bool {
	cd := &k.t.Cards[c]
	ac := spellAddCost(k, c)
	return manaEmpty(&ac.Mana) && s.pickCostPayable(k, p, ac, NoObj) && (ac.Flags&kop.CostDiscard == 0 || int(s.pl[p].handN)-s.inHandN(o) >= int(ac.N)) && s.beholdPayable(k, p, c, o) && s.altAddPartDisjoint(k, p, c, o, ac) ||
		manaEmpty(&cd.AltAddCost.Mana) && s.pickCostPayable(k, p, &cd.AltAddCost, NoObj) && s.altAddPartDisjoint(k, p, c, o, &cd.AltAddCost)
}

// altAddPartDisjoint: the permanents a part sacrifices or taps are not the
// sources that pay the spell's own mana, as gorge's planner holds them back
// (planReserving): one Treasure sacrificed for {B} is gone, so it is not also
// the artifact Stir Up Trouble's part sacrifices (CR 601.2g-h).
func (s *State) altAddPartDisjoint(k *prep, p uint8, c CardID, o Obj, part *kop.Cost) bool {
	if part.Flags&kop.CostTapX == 0 && !(part.Flags&kop.CostSac != 0 && k.sacMana) {
		return true
	}
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	var av avail
	s.availFor(k, p, &av)
	return s.costPartDisjoint(k, p, NoSlot, part, &cost, &av, pipMaskOf(&cost))
}

// altAddManaPart is the mana of card c's either-or additional cost part that has any.
func (s *State) altAddManaPart(k *prep, c CardID) kop.ManaCost {
	if ac := spellAddCost(k, c); !manaEmpty(&ac.Mana) {
		return ac.Mana
	}
	return k.t.Cards[c].AltAddCost.Mana
}

// altAddCastable: a spell with an either-or additional cost has a part
// that can be paid in full -- its picks, and the spell's mana cost with
// the part's -- unless gorge's shape is asked for (Config.GorgeAltAdd),
// where a payable pick cost is enough (spellCostsFeasible).
func (s *State) altAddCastable(k *prep, c CardID, p uint8, o Obj, cost *kop.ManaCost, av *avail) bool {
	return s.altAddCastableFrom(k, c, p, o, cost, nil, false, false, av)
}

// altAddCastableFrom is altAddCastable for a plain cast whose unreduced cost is
// base: the either-or part's mana joins base before the reductions take their
// generic off the total (CR 601.2f), as gorge's offer composes it, so Uncle
// Iroh's {1} off Redirect Lightning's {R} plus {2} leaves {1}{R}, not {2}{R}.
// gen is the generic-cost statics' form of adjustFit. A nil base adds the part
// to cost as it stands. anyMana is a may-play rider letting mana of any type
// pay the cast (Outrageous Robbery): it covers the recomposed total, the
// part's mana included (CR 601.2f-h), as it covers cost.
func (s *State) altAddCastableFrom(k *prep, c CardID, p uint8, o Obj, cost, base *kop.ManaCost, gen, anyMana bool, av *avail) bool {
	cd := &k.t.Cards[c]
	if !cd.HasAltAdd {
		return true
	}
	ac := spellAddCost(k, c)
	if s.pickCostPayable(k, p, ac, NoObj) && (ac.Flags&kop.CostDiscard == 0 || int(s.pl[p].handN)-s.inHandN(o) >= int(ac.N)) && s.beholdPayable(k, p, c, o) {
		return true
	}
	if !s.pickCostPayable(k, p, &cd.AltAddCost, NoObj) {
		return false
	}
	t := addCost(*cost, cd.AltAddCost.Mana)
	if base != nil && (s.statM.any() || k.gen != nil) {
		t = s.adjustFit(k, av, p, c, o, addCost(*base, cd.AltAddCost.Mana), gen)
	}
	if anyMana {
		t = anyType(t)
	}
	if k.gen != nil && k.gen.anyDelve && o != NoObj && (base == nil || !av.tbLeafCredited(base)) {
		t = s.delveOfferCredit(k, p, c, o, s.objs[o].zone == kop.ZoneHand, t) // the total's one credit
	}
	if o == NoObj {
		return av.affords(&t, pipMaskOf(&t), 0)
	}
	// the part's mana may come from restricted sources the spell may spend
	// (Eclipsed Realms' chosen-type mana; CR 106.6, gorge mayPlayAffordable)
	return s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &t, av)
}

// inHandN is 1 when spell object o is itself in a hand (it cannot be discarded
// to pay for its own cast), else 0: a card cast from exile leaves the hand whole.
func (s *State) inHandN(o Obj) int {
	if o != NoObj && s.objs[o].zone == kop.ZoneHand {
		return 1
	}
	return 0
}

func (s *State) countSac(k *prep, p uint8, f kop.FilterID, src Slot) int {
	return s.countPermMatch(k, p, f, src, true)
}

// countPermMatch counts p's permanents matching f; sac leaves out those a
// CantSacrifice static forbids sacrificing (static_cantsacrifice.go).
func (s *State) countPermMatch(k *prep, p uint8, f kop.FilterID, src Slot, sac bool) int {
	n := 0
	m := s.ctrlM[p]
	for m.any() {
		sl := m.first()
		m = m.rest()
		if s.matchPerm(k, f, sl, p, src) && !(sac && s.sacBlocked(k, sl)) {
			n++
		}
	}
	return n
}

// --- targets ---

// tspec returns target slot i's spec for card c's ability ab (ab == 255:
// the permanent spell itself, whose only possible target is an Aura's
// enchant target).
func tspec(k *prep, c CardID, ab uint8, i int) *kop.TargetSpec {
	cd := &k.t.Cards[c]
	if ab == 255 {
		if i == 0 && cd.Types&kop.TAura != 0 {
			return &cd.Enchant
		}
		return nil
	}
	a := &cd.Abilities[ab]
	if i >= int(a.NTargets) {
		return nil
	}
	return &a.Targets[i]
}

// spellXBoundPayable is xBoundPayable for spell c's plain cast; a free or kicked
// cast keeps the wider answer, as gorge prices only the plain one.
func (s *State) spellXBoundPayable(k *prep, c CardID, p uint8, cost *kop.ManaCost) bool {
	ab := spellAb(k, c)
	return ab == 255 || s.xBoundPayable(k, &k.t.Cards[c].Abilities[ab], c, p, NoSlot, cost)
}

func spellAb(k *prep, c CardID) uint8 {
	if ab := k.cards[c].spellAb; ab >= 0 {
		return uint8(ab)
	}
	return 255
}

// spellTargetsFeasibleFrom is spellTargetsFeasible for the spell card o, cast
// from the graveyard: o is on the stack by the time its targets are chosen, so
// it is no candidate for them (gorge castTargetsAvailable names the cast card).
// Flashback under The Dawning Archaic with no other instant or sorcery in the
// graveyard has no target and is not offered.
func (s *State) spellTargetsFeasibleFrom(k *prep, c CardID, p uint8, o Obj) bool {
	if s.objs[o].zone != kop.ZoneGraveyard {
		return s.spellTargetsFeasible(k, c, p)
	}
	s.g.tgtSelf = uint8(o) + 1
	ok := s.spellTargetsFeasible(k, c, p)
	s.g.tgtSelf = 0
	return ok
}

func (s *State) spellTargetsFeasible(k *prep, c CardID, p uint8) bool {
	if k.gen != nil && s.g.probe.kind == 0 {
		return s.probeSpell(k, c, p)
	}
	ab := spellAb(k, c)
	if ab == 255 {
		sp := tspec(k, c, ab, 0)
		return sp == nil || s.hasTarget(k, sp, p, NoSlot, NoObj)
	}
	return s.abilityTargetsFeasible(k, &k.t.Cards[c].Abilities[ab], p, NoSlot, 0xff)
}

// abilityTargetsFeasible: every mandatory target slot (of some chosen
// mode set; modal abilities are feasible when any one mode is) has a
// candidate.
func (s *State) abilityTargetsFeasible(k *prep, a *kop.Ability, p uint8, src Slot, modes uint8) bool {
	if k.gen != nil && s.g.probe.kind == 0 {
		return s.probeAbility(k, a, p, src, modes)
	}
	if a.NModes > 0 && modes == 0xff {
		if a.ModesPick > 1 || a.ModesFew != 0 || a.Repeats() {
			return s.charmFeasible(k, a, p, src)
		}
		for m := 0; m < int(a.NModes); m++ {
			if k.gen != nil && k.gen.anyCharmPick && s.charmPickBarred(a, src, m) {
				continue // ChoiceRestriction$: chosen before (modal_charmpick.go)
			}
			if s.abilityTargetsFeasible(k, a, p, src, 1<<m) {
				return true
			}
		}
		return false
	}
	excl := NoObj
	if a.NModes > 0 && src != NoSlot && a.Kind == kop.AbTriggered {
		// gorge's modeTargetsAvailable excludes a trigger's source from a mode's
		// census; an activated ability's mode may target its own source
		excl = s.perms[src].obj
	}
	for i := 0; i < int(a.NTargets); i++ {
		t := &a.Targets[i]
		if t.UpTo || t.TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		if t.TFlags&kop.TgfGiftRange != 0 {
			return s.giftTargetsFeasible(k, a, p, src)
		}
		if tm := a.TgtModes[i]; tm != 0 && tm&modes == 0 {
			continue
		}
		if t.TFlags&kop.TgfBoundsV != 0 {
			// "X target creatures": X candidates (tgt_variablemax.go)
			if !s.tgtBoundsFeasible(k, t, p, src) {
				return false
			}
			continue
		}
		if excl != NoObj {
			if !s.hasTargetBut(k, t, p, src, excl) {
				return false
			}
		} else if !s.hasTarget(k, t, p, src, NoObj) {
			return false
		}
	}
	if a.NTargets > 1 && !s.distinctCastFeasible(k, a, p, src, modes, excl) {
		return false // TargetUnique$ chain: jointly distinct (tgt_uniqueoffer.go)
	}
	return true
}

// hasTargetBut is hasTarget with the permanent but not counted.
func (s *State) hasTargetBut(k *prep, t *kop.TargetSpec, p uint8, src Slot, but Obj) bool {
	var buf [MaxOptions]uint8
	n := s.targetCands(k, t, p, src, but, &buf)
	for i := 0; i < n; i++ {
		if buf[i] != uint8(but) {
			return true
		}
	}
	return false
}

// objTargetable: battlefield slot sl can be targeted by a spell/ability
// controlled by p (hexproof, shroud, protection from everything).
func (s *State) objTargetable(k *prep, sl Slot, p uint8) bool {
	if k.gen != nil && !s.targetableGen(k, sl, p) {
		return false
	}
	kw := s.kw(k, sl)
	if kw&(kop.KwShroud|kop.KwProtectionAll) != 0 {
		return false
	}
	if kw&kop.KwHexproof != 0 && s.perms[sl].ctrl != p {
		return false
	}
	return true
}

// targetOK reports whether code is a legal choice for spec.
func (s *State) targetOK(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, code uint8) bool {
	if t.ParentPlus1 != 0 && !s.parentTargetOK(t, code) {
		return false
	}
	if code >= PlayerTarget0 && code <= PlayerTarget1 {
		pp := code - PlayerTarget0
		if pp != p && k.gen != nil && k.gen.anyRepl&rbPlHex != 0 && s.playerHexproof(k, pp) {
			return false
		}
		switch t.Kind {
		case kop.TgtAny:
			return true
		case kop.TgtPlayer:
			if t.Filter != 0 {
				switch k.t.Filters[t.Filter].Ctrl {
				case kop.CtrlYou:
					return pp == p
				case kop.CtrlOpp:
					return pp != p
				}
			}
			return true
		}
		return false
	}
	if t.Kind == kop.TgtStackObj || t.Kind == kop.TgtStackAb || copyAbKind(t.Kind) || ctrShapeKind(t.Kind) {
		return s.stackTgtOK(k, t, p, self, code) // tgt_stackability.go
	}
	if int(code) >= MaxObj {
		return false
	}
	o := Obj(code)
	if t.Kind == kop.TgtPermOrSpell {
		return s.targetOKPermOrSpell(k, t, p, src, self, o) // tgt_permorspell.go
	}
	if t.Kind == kop.TgtPermOrGraveCard {
		return s.targetOKPermOrGraveCard(k, t, p, src, self, o) // tgt_permorgravecard.go
	}
	ob := &s.objs[o]
	switch t.Kind {
	case kop.TgtObject, kop.TgtAny:
		if ob.zone != kop.ZoneBattlefield {
			return false
		}
		sl := ob.slot
		if t.Kind == kop.TgtAny {
			if s.typesAt(k, sl)&(kop.TCreature|kop.TPlaneswalker|kop.TBattle) == 0 {
				return false
			}
			if DebugTgtAnyNoFilter { // the test-only mutation
				return s.objTargetable(k, sl, p)
			}
		}
		return s.matchPerm(k, t.Filter, sl, p, src) && s.objTargetable(k, sl, p)
	case kop.TgtSpell:
		if ob.zone != kop.ZoneStack || o == self {
			return false
		}
		return s.matchCardAt(k, t.Filter, ob.card, s.stackCtrl(o), p, o)
	case kop.TgtCard:
		z := k.t.Filters[t.Filter].Zone
		if z == kop.ZoneNone {
			z = kop.ZoneGraveyard
		}
		if k.gen != nil {
			// the object's own state too (Abyssal Harvester's "put there this turn")
			return ob.zone == z && (t.Filter == 0 || s.matchCardTgt(k, t.Filter, ob.card, ob.owner(), p, o, src, self) && (o != self || k.t.Filters[t.Filter].Flags&kop.FOther == 0))
		}
		return ob.zone == z && s.matchCard(k, t.Filter, ob.card, ob.owner(), p)
	}
	return false
}

func (s *State) stackCtrl(o Obj) uint8 {
	for i := int(s.sp) - 1; i >= 0; i-- {
		if s.stack[i].kind == seSpell && s.stack[i].obj == o {
			return s.stack[i].ctrl
		}
	}
	return s.objs[o].owner()
}

// targetCands writes every legal choice for spec into out (players first,
// then battlefield in slot order, then the stack top-down, then cards).
func (s *State) targetCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]uint8) int {
	n := 0
	if t.Kind == kop.TgtAny || t.Kind == kop.TgtPlayer {
		for pp := uint8(0); pp < 2; pp++ {
			if s.targetOK(k, t, p, src, self, PlayerTarget0+pp) {
				out[n] = PlayerTarget0 + pp
				n++
			}
		}
	}
	switch t.Kind {
	case kop.TgtObject, kop.TgtAny:
		m := s.occ
		for m.any() {
			sl := m.first()
			m = m.rest()
			o := s.perms[sl].obj
			if s.targetOK(k, t, p, src, self, uint8(o)) {
				out[n] = uint8(o)
				n++
			}
		}
	case kop.TgtSpell, kop.TgtSpellTgting:
		for i := int(s.sp) - 1; i >= 0; i-- {
			e := &s.stack[i]
			if e.kind == seSpell && s.targetOK(k, t, p, src, self, uint8(e.obj)) {
				out[n] = uint8(e.obj)
				n++
			}
		}
	case kop.TgtStackObj, kop.TgtStackAb, kop.TgtCopyAb, kop.TgtCopyTrig, kop.TgtStackObjTrig:
		n = s.stackTgtCands(k, t, p, self, out, n) // tgt_stackability.go
	case kop.TgtPermOrSpell:
		n = s.permOrSpellCands(k, t, p, src, self, out, n) // tgt_permorspell.go
	case kop.TgtPermOrGraveCard:
		n = s.permOrGraveCardCands(k, t, p, src, self, out, n) // tgt_permorgravecard.go
	case kop.TgtCard:
		for pp := 0; pp < 2; pp++ {
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if uint8(o)+1 == s.g.tgtSelf {
					continue
				}
				if s.targetOK(k, t, p, src, self, uint8(o)) && n < MaxOptions-1 {
					out[n] = uint8(o)
					n++
				}
			}
		}
		if k.t.Filters[t.Filter].Zone == kop.ZoneExile {
			n = s.exileCardCands(k, t, p, src, self, out, n) // filter_props.go
		}
	}
	return n
}

func (s *State) hasTarget(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj) bool {
	if k.gen != nil && t.Min > 1 {
		return s.hasTargetsMin(k, t, p, src, self)
	}
	switch t.Kind {
	case kop.TgtAny:
		return true // a player is always a legal any-target
	case kop.TgtObject:
		for m := s.occ; m.any(); m = m.rest() {
			if s.targetOK(k, t, p, src, self, uint8(s.perms[m.first()].obj)) {
				return true
			}
		}
		return false
	}
	var buf [MaxOptions]uint8
	return s.targetCands(k, t, p, src, self, &buf) > 0
}

// fbInstances classifies a graveyard card with a printed flashback and a
// granted one besides (Sphinx of Forgotten Lore: the card's mana cost), the
// two instances gorge offers as separate casts (rules extraFlashbackCosts,
// CR 702.34a). Its potential walk lists an instance its potential pool
// (PotentialMana: a dual adds one of each of its colours) affords; two
// listed are "ambiguous" and neither is offered under the oracle surface
// (Think Twice, pair 7295); the granted one alone is offered at the mana
// cost (Inspiration from Beyond's {2}{U} beside its {5}{U}{U}, pair 1841).
// two reports a distinct granted instance; granted that the cast takes it.
func (s *State) fbInstances(k *prep, p uint8, o Obj) (two, ambiguous, granted bool) {
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	if k.gen == nil || k.cards[c].kw&kop.KwFlashback == 0 || !s.fbGranted(k, o) || cd.Cost == cd.AltCost {
		return false, false, false
	}
	// gorge prices each instance for this spell: a source whose restriction
	// refuses it adds nothing (Secluded Courtyard's any colour for an instant)
	spell := payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}
	var pa avail
	s.availPotentialFor(k, p, &pa, &spell)
	pf := s.adjustCost(k, p, c, o, cd.AltCost)
	gf := s.adjustCost(k, p, c, o, cd.Cost)
	pPot, gPot := pa.affords(&pf, pipMaskOf(&pf), 0), pa.affords(&gf, pipMaskOf(&gf), 0)
	// a printed non-mana part (Group Project's tap three creatures) the
	// census cannot pay leaves the printed instance uncastable whatever its
	// mana: gorge offers the granted one (CR 702.34a)
	printedOK := (cd.FbCost.Flags == 0 || s.pickCostPayable(k, p, &cd.FbCost, o)) && (!cd.FbBehold || s.beholdPayable(k, p, c, o))
	pPot = pPot && printedOK
	var av avail
	s.availFor(k, p, &av)
	if printedOK && av.affords(&pf, pipMaskOf(&pf), 0) {
		// the printed instance is the planner's planned flashback cast,
		// offered whatever the potential walk says (Think Twice, pair 90106)
		return true, false, false
	}
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 {
		return true, pPot && gPot, !pPot
	}
	// gorge's own decision lists both: the kernel's one cast takes the
	// granted instance when its sources cannot pay the printed one
	return true, false, true
}

// availPotential is gorge's potential pool (rules PotentialMana) as an
// avail: the floating pool, and what each untapped source could produce. It is a
// deliberate over-bound: a source adds one unit of every colour it can make, and
// an open production (any colour, an unread chosen colour) is unlimited.
func (s *State) availPotential(k *prep, p uint8, a *avail) {
	s.availPotentialFor(k, p, a, nil)
}

// potentialOpenUnits stands for gorge's potentialUnbounded in each colour: more
// than any cost the kernel prices, and within an avail's uint8 count.
const potentialOpenUnits = 2 * trigXCap

// availPotentialFor is availPotential; payment prices one mana payment (gorge
// potentialManaFor with a descriptor, a trigger's own cost): an ability whose
// RestrictValid$ refuses it adds nothing and a chosen colour is exact (CR 106.6).
// Every mana ability of a source counts on its own, and a paid one joins once the
// pool built from the other sources covers its cost, so the walk runs to a fixpoint.
//
//go:noinline
func (s *State) availPotentialFor(k *prep, p uint8, a *avail, d *payDesc) {
	payment := d != nil
	var pot [kop.NumManaKinds]int
	for c := range pot {
		pot[c] = int(s.pl[p].pool[c])
	}
	var own [MaxPerm][kop.NumManaKinds]int
	var done [MaxPerm]uint16
	open := false
	add := func(sl Slot, mask uint8, n uint8) {
		for mk := mask; mk != 0; mk &= mk - 1 {
			c := bits.TrailingZeros8(mk)
			pot[c] += int(n)
			own[sl][c] += int(n)
		}
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		pc := &k.cards[s.cardAt(sl)]
		if k.gen != nil {
			pc = s.manaPCGen(k, sl)
		}
		if !s.canTapForManaPC(k, sl, pc) || pc.manaAb < 0 && s.perms[sl].ovr.flags&ovCopy == 0 || payment && k.gen != nil && s.openManaRefused(k, p, sl, d) {
			continue
		}
		if pc.manaAb >= 0 {
			done[sl] = 1 << pc.manaAb
		}
		dynamic := false
		if k.gen != nil {
			if c := s.cardAt(sl); k.gen.flags[c]&gfManaVar != 0 && !s.filtFeeOwed(k, sl, c) {
				// a gated or reflected source's view already holds its abilities
				done[sl] = ^uint16(0)
				cd := &k.t.Cards[c]
				dynamic = pc.manaAb >= 0 && int(pc.manaAb) < len(cd.Abilities) && cd.Abilities[pc.manaAb].AFlags&kop.AfManaDynamic != 0
			}
		}
		if dynamic || !pc.manaMulti && (pc.manaCol == kop.ColAny && !pc.manaChosen || pc.manaChosen && !payment) {
			open = true
			continue
		}
		mask := s.srcMaskAt(pc, sl)
		if k.manaEach {
			mask |= xvMask(pc) // an all-at-once source adds each of its colours (gorge addPotentialMana: every rune)
		}
		if pc.manaMulti && k.gen != nil && done[sl] != ^uint16(0) && pc.manaAb >= 0 {
			mask = s.potentialFirstMask(k, sl, pc) // the walk below admits its other abilities, each once
		}
		add(sl, mask, srcYield(pc))
		if k.gen != nil && len(k.gen.grantPC) != 0 && s.grantMay(k, sl) && s.ownManaUsable(k, sl) {
			// a mana ability granted beside the source's own one (an aura's
			// "{T}: Add two mana of any one color") counts on its own, as
			// gorge's walk admits each of a source's abilities
			var gi [maxGrants]int16
			for i, ng := 0, s.grantIdxs(k, sl, &gi); i < ng; i++ {
				gv := s.grantView(k.gen, gi[i], sl)
				if gv.manaCol == kop.ColAny && !gv.manaChosen && !gv.manaMulti {
					open = true
					continue
				}
				add(sl, s.srcMaskAt(gv, sl), gv.manaN)
			}
		}
	}
	for progressed := k.gen != nil; progressed; {
		progressed = false
		for m := s.ctrlM[p]; m.any(); m = m.rest() {
			sl := m.first()
			cd := &k.t.Cards[s.cardAt(sl)]
			others := pot
			for c := range others {
				others[c] -= own[sl][c]
			}
			for j := range cd.Abilities {
				ab := &cd.Abilities[j]
				if j >= 16 || ab.Kind != kop.AbMana || ab.AFlags&kop.AfLatent != 0 || done[sl]&(1<<j) != 0 || !s.potentialAdmits(k, p, sl, j, ab, d) {
					continue
				}
				if !open && !potentialCovers(&others, &ab.Cost.Mana) {
					continue
				}
				done[sl] |= 1 << j
				progressed = true
				if ab.ManaColors == kop.ColAny || ab.AFlags&(kop.AfManaDynamic|kop.AfManaEachColor) != 0 { // gorge's walk reads an each-colour production (Bloom Tender) as open
					open = true
				} else {
					add(sl, uint8(ab.ManaColors), max(ab.ManaN, 1))
				}
			}
		}
	}
	a.total, a.n = 0, 0
	for c, n := range pot {
		if open {
			n = potentialOpenUnits
		}
		if n > 0 {
			a.addUnits(1<<c, uint8(min(n, 255)))
		}
	}
}

// filtFeeOwed reports whether the source at sl is a filter whose fee the
// reducers leave owed (mana_filterfree.go): its gate-off view is the free {C}
// alone, so the fixpoint below must still admit the paid filter once the other
// sources cover its fee, as gorge potentialManaFor admits each ability (Surveillance
// Room's {1},{T} any-colour beside its {T}: {C}, CR 605.3a).
func (s *State) filtFeeOwed(k *prep, sl Slot, c CardID) bool {
	return k.gen.flags[c]&gfFiltFee != 0 && s.filtFee(k, sl) != 0
}

// potentialAdmits reports whether a further mana ability of the source at sl can
// count toward p's potential pool: usable now, its non-mana costs a {T}, a
// once-each-turn limit or a sacrifice of the source, and its restriction
// admitting the payment.
func (s *State) potentialAdmits(k *prep, p uint8, sl Slot, j int, ab *kop.Ability, d *payDesc) bool {
	pp := &s.perms[sl]
	if ab.Cost.Flags&^(kop.CostTap|kop.CostSacSelf|kop.CostPayLife|kop.CostOncePerTurn) != 0 && !s.potentialGapCost(k, p, sl, j, ab) || pp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.genLost(k, sl) {
		return false
	}
	if ab.Cost.Flags&kop.CostOncePerTurn != 0 && j < 8 && pp.actUsed&(1<<j) != 0 {
		return false // Gravestone Strider, used this turn
	}
	if ab.Cost.Flags&kop.CostPayLife != 0 && int(s.pl[p&1].life) < int(ab.Cost.N) {
		return false
	}
	if k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	if ab.Cost.Flags&kop.CostTap != 0 && (pp.flags&pfTapped != 0 || pp.flags&pfSick != 0 && s.isCreature(k, sl) && s.kw(k, sl)&kop.KwHaste == 0) {
		return false
	}
	if ab.Trig.Cond != 0 && !s.condEval(k, ab.Trig.Cond, evalCtx{src: pp.obj, srcSl: sl, ctrl: pp.ctrl}) {
		return false
	}
	return d == nil || ab.ManaRestrict == 0 || s.restrSrcAdmits(k, p, sl, ab, d)
}

// potentialGapCost reports whether mana ability j of the source at sl, whose cost goes
// beyond a tap, a self-sacrifice and life, is one gorge's potential-mana walk
// admits while its cost can be paid: a tap-another-permanent source with a permanent to
// tap (Springleaf Drum, Baylen's tokens), or a counter cost put on its source once each
// turn (Wall of Roots). They are the shapes kop.ManaAbilityHonoured lists.
func (s *State) potentialGapCost(k *prep, p uint8, sl Slot, j int, ab *kop.Ability) bool {
	c := s.cardAt(sl)
	if k.cards[c].manaAb != int8(j) {
		return false
	}
	switch {
	case kop.CtrManaAb(ab):
		return ab.Cost.Flags&kop.CostRemoveCtr == 0 || s.perms[sl].ctrN(s, int(ab.Cost.Ctr)) >= int(ab.Cost.N)
	case k.tapMana && kop.TapXMana(ab):
		var cs [MaxPerm]Slot
		return s.tapXCands(k, p, sl, &cs) >= int(ab.Cost.N)
	}
	return false
}

// potentialCovers reports whether the pool others covers cost's pips and total.
func potentialCovers(others *[kop.NumManaKinds]int, cost *kop.ManaCost) bool {
	total := 0
	for c, n := range others {
		if int(cost.Pips[c]) > n {
			return false
		}
		total += n
	}
	return total >= cost.MV()
}

// --- applying a priority action ---

func (s *State) applyPriority(k *prep, a Action) {
	p := s.prio
	sp0 := s.sp
	// A filter beside a grant is one search in gorge (scriptSearch.visit:
	// every census gap in battlefield order), and the grant search is that
	// search (mana_grantedoffers.go candidates). An activation's filter
	// search funds a filter's fee from printed sources only
	// (mana_mixgaps.go otherGaps), so running it first would tap those
	// before the granted source gorge taps; run the grant search first and
	// let the filter search see the play priced.
	grants := k.gen != nil && len(k.gen.grantPC) != 0
	// A deferred source (Pyromancer's Goggles) is a gap candidate of the same
	// search (mana_grantdeferred.go): scriptPrefix would tap it alone, ahead of a
	// granted source that precedes it in battlefield order, which gorge's search
	// taps first (Imprisoned in the Moon's land beside Goggles).
	grantFirst := grants && (a.Kind() == AkActivate && k.gen.anyFilt && s.filtOnly(k, p) || k.gen.anyDeferred && s.scriptDeferred(k, p).any())
	grantFirst = grantFirst || grants && s.deferBesideGrant(k, p) // the same one search (mana_grantdeferred.go)
	if grantFirst {
		s.grantPrefix(k, a, p)
		if s.npend != 0 {
			s.flushPrefixTriggers(k)
		}
	}
	if s.scriptAny(k) && isScriptPlayKind(a.Kind()) {
		s.scriptPrefix(k, a, p) // a play only a scripted activation pays (script_gen.go)
		if s.npend != 0 {
			s.flushPrefixTriggers(k) // its trigger resolves before the play (seed 640)
		}
	}
	if grants && !grantFirst {
		s.grantPrefix(k, a, p) // a play only a granted mana ability pays (script_gen.go)
		if s.npend != 0 {
			s.flushPrefixTriggers(k)
		}
	}
	if s.sp > sp0 || s.npend != 0 {
		empty := s.playNeedsEmptyStack(k, a)
		if s.sp > sp0 && empty || s.npend != 0 {
			s.resolvePrefixStack(k, sp0, empty) // the prefix's triggers placed (and, sorcery-speed, resolved) before the play
		}
	}
	switch a.Kind() {
	case AkPass:
		s.pass(k)
	case AkPlayLand:
		o := Obj(a.Source())
		if k.gen != nil && s.occ.full() {
			s.stall(StallBoard) // the battlefield is full: the capacity draw
			return
		}
		if a.Mode() != 0 {
			s.turnFace(k, o, a.Mode()) // a modal DFC's land face (face_multiface.go)
		}
		if k.gen != nil && s.preLandAsk(k, o) {
			return // its as-enters choice is asked first (gorge)
		}
		s.playLandNow(k, o, p)
	case AkCast:
		face := a.Mode()&castFace != 0
		if face && (a.Target() != 0 || k.anyRoom && roomDef(k, s.objs[a.Source()].card)) {
			s.turnFaceCast(k, Obj(a.Source()), a.Target()) // a face cast (face_castface.go)
		}
		if a.Target() == castImpTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyImpending {
			s.g.impCast = true // the impending cost (kw_impending.go)
		}
		if a.Target() == castEvokeTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyEvoke {
			s.g.impCast = true // the evoke cost (kw_evoke.go): beginCast marks the entry evoked
		}
		if a.Target() == castAltTgt && a.Mode() == 0 && k.gen != nil && k.gen.altFree != nil {
			s.g.altCast = true // the card's own mana alternative cost (cast_altfree.go)
		}
		if a.Mode() == 2 && k.gen != nil && k.gen.anyBeam {
			s.noteBeamCast(k, a) // kw_beammeup.go
		}
		s.beginCast(k, Obj(a.Source()), a.Mode())
		if face {
			s.lowerFaceCast(k, p, Obj(a.Source())) // cast_faceroute.go
		}
	case AkCastConspire:
		s.beginConspireCast(k, Obj(a.Source())) // CR 702.78a; kw_conspire.go
	case AkCastCasualty:
		s.beginCasualtyCast(k, Obj(a.Source()))
	case AkActivate:
		if k.gen != nil && s.activateGen(k, a) {
			break
		}
		s.beginActivate(k, Obj(a.Source()), a.Mode())
	case AkTurnFaceUp:
		s.turnFaceUp(k, Obj(a.Source()), p, a.Mode()) // CR 708.6; mode = procedure
	case AkPlot:
		s.plotAction(k, Obj(a.Source()), p) // CR 702.170a; kw_plot.go
	case AkUnlock:
		s.unlock(k, Obj(a.Source()), p, a.Mode()) // CR 709.5e; face_rooms.go
	case AkSneak:
		s.beginSneak(k, Obj(a.Source())) // CR 702.190a; kw_sneak.go
	case AkBestow:
		s.beginBestow(k, Obj(a.Source())) // CR 702.103a; kw_bestow.go
	case AkWebSling:
		s.beginWebSling(k, Obj(a.Source())) // CR 702.188a; kw_websling.go
	case AkMayFlash:
		s.beginMayFlash(k, Obj(a.Source())) // CR 702.8; kw_mayflash.go
	default:
		panic("kernel: bad priority action")
	}
}

// playLandNow moves land o from its zone (hand, or exile for a land-front
// Adventure, CR 715.3d) onto the battlefield.
func (s *State) playLandNow(k *prep, o Obj, p uint8) {
	counted := false // gorge counts the land play when it is made, before the order ask (repl_entryorder.go)
	if k.gen != nil && k.gen.anyEntUntap {
		counted = s.g.ro.entryAnswered(o)
		if s.entryOrderPark(k, o, p, entLand) {
			s.pl[p].landsPlayed++
			s.g.ctlRet, s.ctl = ctlPriority, ctlGenAsk // the CR 616.1 entry-order ask, the land still in its zone
			return
		}
	}
	if z := s.objs[o].zone; z == kop.ZoneExile || z == kop.ZoneGraveyard {
		s.spendMayRecord(k, o, p) // a land played through a permission record spends it, as a cast does (CR 701.18b)
	}
	if z := s.objs[o].zone; k.gen != nil && k.gen.anyMayPlay && (z == kop.ZoneExile || z == kop.ZoneGraveyard || z == kop.ZoneSideboard) {
		s.spendLandStatic(k, o, p) // a once-per-turn static's land play uses its turn (zone_sideboard.go)
	}
	from := s.detach(o)
	if k.gen != nil && k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from) // a may-play land from a graveyard or exile
	}
	// a shock land's election is posed once it is on the battlefield: the
	// play's land count is taken when the answer completes it
	// (repl_shockland.go)
	deferCount := k.gen != nil && k.gen.has(s.objs[o].card, gfShockEntry)
	if !deferCount && !counted {
		s.pl[p].landsPlayed++
	}
	if k.gen != nil && k.gen.anyLandPlayed {
		s.enterLandPlayed(k, o, p, from)
	} else {
		s.enter(k, o, p, false, from)
	}
	if deferCount && !counted {
		s.shockLandCount(o, p)
	}
	s.passes = 0
}

// castStatic reports whether a battlefield static of kind sk controlled by
// p applies to card c being cast by p.
func (s *State) castStatic(k *prep, sk kop.StaticKind, p uint8, c CardID) bool {
	m := s.statM.and(s.ctrlM[p])
	for m.any() {
		src := m.first()
		m = m.rest()
		if s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || k.gen != nil && s.ovrLoses(k, src) {
			continue // the source lost its abilities (Witness Protection on High Fae Trickster)
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == sk && (sk != kop.StatCastWithFlash || !cd.SelfFlash) && s.matchCard(k, a.Static.Filter, c, p, p) &&
				(a.Static.Cond2 == 0 || s.condEval(k, a.Static.Cond2, evalCtx{src: s.perms[src].obj, srcSl: src, ctrl: p})) {
				return true // the static's CheckSVar$/IsPresent$ gate, as gorge castWithFlash reads the static's conditions
			}
		}
	}
	return false
}

// adjustCost applies ReduceCost / IncreaseCost statics (generic only; a
// reduction never goes below zero generic) to card c cast by p. A
// ReduceCost static reduces its controller's spells; an IncreaseCost
// static taxes every player's spells matching its filter.
func (s *State) adjustCost(k *prep, p uint8, c CardID, priced Obj, base kop.ManaCost) kop.ManaCost {
	if k.gen != nil {
		return s.adjustCostGenB(k, p, c, NoObj, priced, base, k.bargainRed && s.bargainCharge(k, c))
	}
	g := int(base.Generic)
	m := s.statM
	for m.any() {
		src := m.first()
		m = m.rest()
		sp := &s.perms[src]
		if sp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic {
				continue
			}
			switch a.Static.Kind {
			case kop.StatReduceCost:
				if (sp.ctrl == p || a.Static.StFlags&kop.StReduceAny != 0) && s.matchCard(k, a.Static.Filter, c, p, sp.ctrl) {
					g -= int(a.Static.A)
				}
			case kop.StatIncreaseCost:
				if s.matchCard(k, a.Static.Filter, c, p, sp.ctrl) {
					g += int(a.Static.A)
				}
			}
		}
	}
	if g < 0 {
		if base.X != 0 {
			base.XRed = uint8(min(-g, 255)) // CR 601.2f: the excess pays into {X}
		}
		g = 0
	}
	base.Generic = uint8(g)
	return base
}
