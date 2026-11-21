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

// castGateAffordable is the hand-cast offer gate's affordability read over
// cav: the printed cost when its own is affordable, else for a Phyrexian
// cost the whole cost over the cast window's census when its pip-free mana
// half is (s.phyCastGateAffords, mana_phyrexian.go, CR 601.2b), else the
// restricted-spend fallback (a restricted source the spell may spend prices
// it, as gorge's planner's prefix search).
func (s *State) castGateAffordable(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, cav *avail) bool {
	if s.phyCastGateAffords(k, p, cav, cost) {
		return true
	}
	// On the autopay surface a life-only census gap's unit (Treasonous Ogre's
	// "Pay 3 life: Add {R}") funds the cast in gorge's planner -- the relaxed
	// census it prices the play over (paymentPlanRelaxedAlternatives), which
	// the kernel's gaps census holds (availScript's life arm, mana_lifegap.go).
	// The plain census leaves it out, and the hand-cast gate dropped the play
	// (Noose Constrictor's {1}{G} beside a Forest and the Ogre: pair 97, pool
	// dr-mbc-bbbb, the options lockstep). On the lowering surface the cast
	// stays the script route's (the recorded life-gap repros: gorge lowers the
	// potential play; the planner has no unit for the ability there). Not in the
	// scripted any-colour and gaps walks (scriptOffers): their censuses count
	// the life units on their own (availScript), and a play they list is
	// gorge's potential play, which its prefix search prices, not the planner
	// (Hollow Marauder beside the Ogre: the Ogre repro, decision 198).
	if s.cfg&cfgGorgeAutopay != 0 && s.g.availX&(availAny|availGaps) == 0 && k.gen != nil && k.gen.uses.has(useLifeGap) && s.lifeGapMask(k, p).any() {
		var t avail
		t = *cav
		s.availLifeGaps(k, p, &t)
		if t.affords(cost, pipMaskOf(cost), 0) {
			return true
		}
	}
	return (s.rbAny(p) || k.restrSrc) && s.restrFallback(k, p, c, s.restrOriginOfObj(o), cost, cav)
}

// castGateOrPoolPays is the plain cast's affordability read: the gate's own
// read, or the conversion pool paying the pips (mana_convertcreature.go).
func (s *State) castGateOrPoolPays(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, cav *avail) bool {
	return s.castGateAffordable(k, p, c, o, cost, cav) || s.convPoolPays(k, p, o, cost)
}

// kickedAffordsOrPoolPays is the kicked cast's corresponding read over kc.
func (s *State) kickedAffordsOrPoolPays(k *prep, p uint8, c CardID, o Obj, kc *kop.ManaCost, cav *avail) bool {
	return s.kickedAffords(k, p, c, o, kc, cav) || s.convPoolPays(k, p, o, kc)
}

func (s *State) priorityOptions(k *prep, d *Decision) {
	p := s.prio
	pl := &s.pl[p]
	sorc := s.sorcerySpeed(p)
	var av avail
	// A cast's census leaves a dynamic {T}, sacrifice source out (avail.dynSac
	// stays false): the counter-cost gap lists the cast that source funds
	// (mana_sacctr.go), and gorge activates the source at priority (its script
	// route), so a plain census cast would tap the printed sources instead.
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
				d.add(MakeAction(AkPlayLand, TCode(o), 0, 0))
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
		if k.gen != nil && k.gen.anyForetell && k.t.Cards[c].HasForetell && p == s.active && (sorc || !DebugForetellSorcery) {
			// the foretell special action needs only priority on its
			// controller's own turn, never sorcery timing (CR 702.143a;
			// gorge offers it before the front-face timing gate)
			s.offerForetellHand(k, d, p, o, c) // kw_foretell.go
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
		if k.gen != nil && k.gen.anyCantCast && s.castForbiddenCost(k, p, c, o, &pc.cost, nil) {
			continue
		}
		if !haveAv {
			s.availFor(k, p, &av)
			haveAv = true
		}
		if k.gen != nil && k.gen.anyWarp && (k.t.Cards[c].HasWarp || k.gen.anyWarpGrant) {
			// before the plain affordability gate: a warp cast is offered
			// even when the mana cost is not payable (CR 702.185a)
			s.offerWarpHand(k, d, p, o, c, &av) // kw_warp.go
		}
		if k.gen != nil && k.gen.anySuspend && k.t.Cards[c].Suspend != 0 {
			// under the plain cast's timing and prohibition gates, before its
			// affordability gate and without a target check (CR 702.62a)
			s.offerSuspendHand(k, d, p, o, c, &av) // kw_suspend.go
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
		var fav avail
		cav := s.castGrantAvail(k, p, o, &av, &fav) // a restricted grant the cast cannot spend is no unit of its census
		if !s.castGateOrPoolPays(k, p, c, o, cost, cav) {
			if k.bargainRed {
				s.offerBargainOnly(k, d, p, o, c, &av) // the discount alone pays it (cost_reducespell.go)
				s.offerOptCostOnly(k, d, p, o, c, &av)
			}
			if k.gen == nil || !k.gen.anySacX || !s.sacXPoolOffer(k, p, o) {
				// the bestow cost needs no affordable mana cost (CR 702.103a)
				s.offerBestowIfPrinted(k, d, p, o, c, &av)
				if pc.kw&kop.KwKicker != 0 || k.t.Cards[c].HasMultikicker {
					s.offerKickedPotentialOnly(k, d, p, c, o, pc.cost, preDelve, &base, cav, &av) // kw_multikicker.go
				}
				continue // (an announced Sac<X> the floating pool pays at some X is offered, cost_sacx.go)
			}
		}
		tgtOK, twOK := true, false
		if k.teamwork && k.t.Cards[c].Teamwork != 0 {
			// the plain and the teamworked cast announce different targets
			// (kw_teamwork.go): either may be castable alone
			tgtOK, twOK = s.teamworkTargets(k, c, p)
		} else {
			tgtOK = s.spellTargetsFeasibleFrom(k, c, p, o)
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
			d.add(MakeAction(AkCast, TCode(o), 0, 0))
		}
		if plainOK && k.gen != nil && k.gen.anyConspire && s.conspireOffer(k, p, c, o) && s.casualtyOffers(p, o) {
			// the conspired cast (kw_conspire.go): beside the plain cast; like the casualty cast, gorge offers it only against a pool that pays
			d.add(MakeAction(AkCastConspire, TCode(o), 0, 0))
		}
		if twOK && s.teamworkOffer(k, p, c) && s.casualtyOffers(p, o) {
			// the teamworked cast (kw_teamwork.go): the same action kind
			d.add(MakeAction(AkCastCasualty, TCode(o), 0, 0))
		}
		if k.casualty && s.casualtyOffer(k, p, c, o) && s.casualtyOffers(p, o) {
			// the casualty cast (kw_casualty.go): the plain cast's price, one more option
			d.add(MakeAction(AkCastCasualty, TCode(o), 0, 0))
		}
		if k.bargain && s.bargainOffer(k, p, c) && s.bargainOffers(k, p, o) {
			// the bargained cast (kw_bargain.go): the same action kind
			d.add(MakeAction(AkCastCasualty, TCode(o), 0, 0))
		}
		if k.gen != nil && k.gen.anyOptCost && tgtOK && s.optCostOffer(k, p, c, o) {
			// the optional-cost cast (cost_optional.go): the kicked action
			d.add(MakeAction(AkCast, TCode(o), 0, 1))
		}
		s.offerBestowIfPrinted(k, d, p, o, c, &av) // kw_bestow.go
		if pc.kw&kop.KwKicker != 0 || k.t.Cards[c].HasMultikicker || k.offspring && offspringOffer(k, c) {
			s.offerKickedHand(k, d, p, c, o, pc.cost, cav)
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
		marked := fbLo != nil && (fbLo.has(o) || fbHi.has(o))
		granted := pc.kw&kop.KwFlashback == 0 && (marked || s.fbLive(k, o)) // a stamped grant or a live static's (kw_fbgrant.go)
		if pc.kw&kop.KwFlashback == 0 && !granted || (!pc.instant && !sorc && !s.castFlash(k, p, c)) {
			continue // (cast as though it had flash: High Fae Trickster)
		}
		if !haveAv {
			s.availFor(k, p, &av)
			haveAv = true
		}
		fc := k.t.Cards[c].AltCost
		if granted {
			fc = s.fbGrantCost(k, p, o, marked) // the granted flashback costs the card's mana cost, or the grant's own
		}
		if s.statM.any() || k.gen != nil {
			fc = s.adjustCost(k, p, c, o, fc)
		}
		fbGrant := granted // the cast pays a granted instance (no printed non-mana part)
		if !granted {
			if two, amb, g := s.fbInstances(k, p, o); amb {
				continue
			} else if two && g {
				fc = s.adjustCost(k, p, c, o, s.fbGrantCost(k, p, o, s.fbGranted(k, o)))
				fbGrant = true
			}
		}
		fc = s.delveOfferCredit(k, p, c, o, false, fc)
		if k.gen != nil && k.gen.anyCantCast && s.castForbiddenCost(k, p, c, o, &fc, &av) {
			continue // the flashback cost's own X (CR 202.3e, 107.3b)
		}
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
		if s.cfg&cfgGorgeUnpriced != 0 && s.contribPays(k, p, c, o, false) && !s.FeatPoolPays(p, o, 2) {
			continue // gorge's potential planner declines a flashback cast carrying a credit ("cost:credit"): only the floating pool casts it (Inspiring Statuary's improvise)
		}
		if !s.flashbackPartsDisjoint(k, p, c, fc) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && tgtDepReduce(&k.t.Cards[c]) && severalTargets(k, c) && !s.FeatPoolPays(p, o, 2) {
			// gorge's potential planner prices a target-conditional reduction on
			// a mode spell choosing several targets with no target: the full cost
			s.g.noTgtRed = true
			full := s.adjustCost(k, p, c, o, s.fbGrantCost(k, p, o, marked))
			if !granted {
				full = s.adjustCost(k, p, c, o, k.t.Cards[c].AltCost)
			}
			s.g.noTgtRed = false
			if !av.affords(&full, pipMaskOf(&full), 0) {
				continue
			}
		}
		if s.cfg&cfgGorgeAutopay != 0 && !s.FeatPoolPays(p, o, 2) && s.contribPays(k, p, c, o, false) {
			// the autopay surface's credit class: a convoke, improvise or delve
			// contribution live for the cast (Inspiring Statuary's improvise
			// grant on Krosan Reclamation's {1}{G} flashback) is one gorge's
			// potential planner withholds -- "flashback" is no alternative-cost
			// row, so potentialModeCredit keeps the old unknown verdict and the
			// potential play is dropped ("cost:credit"), while gorge's own walk
			// prices the cast from the floating pool alone. The ordinary
			// castAffordsR above prices the sources too, so without this gate
			// the kernel offered the cast gorge does not list (tail-53 pair 2,
			// seed 142735). A pool that pays the composed flashback cost needs
			// no credit and stays offered (gorge's walk lists it), and the
			// lowering surface keeps the plain offer (the scripts route).
			continue
		}
		d.add(MakeAction(AkCast, TCode(o), 0, 2))
	}
	if k.gen != nil && k.gen.anyHarmonize {
		s.offerHarmonizeCasts(k, d, p, sorc, &av) // kw_harmonize.go
	}
	if k.gen != nil && k.gen.anyBeam {
		s.offerBeamCasts(k, d, p, sorc, &av) // kw_beammeup.go
	}
	if k.escape || k.gen != nil && k.gen.anyEscapeGrant {
		s.offerEscapeCasts(k, d, p, sorc, &av) // kw_escape.go: gorge lists the escape walk before mayhem's (legal_walk_alt.go)
	}
	if k.mayhem {
		s.offerMayhemCasts(k, d, p, sorc, &av) // kw_mayhem.go
	}
	if k.retrace {
		s.offerRetraceCasts(k, d, p, sorc, &av) // kw_retrace.go
	}
	if faces {
		s.offerAdventureRecasts(k, d, p, sorc) // face_castface.go
		if k.gen.anyAftermath {
			s.offerAftermathCasts(k, d, p, sorc) // face_aftermath.go
		}
	}
	if k.gen != nil && k.gen.anyAirbend {
		s.offerAirbendCasts(k, d, p, sorc) // act_airbend.go (gorge offers it before the warp recast)
	}
	if k.gen != nil && k.gen.anyWarp {
		s.offerWarpRecasts(k, d, p, sorc) // kw_warp.go
	}
	if k.gen != nil && k.gen.anyPlot {
		s.offerPlotCasts(k, d, p, sorc, &av) // kw_plot.go
	}
	if k.gen != nil && k.gen.anyForetell {
		s.offerForetellCasts(k, d, p, sorc) // kw_foretell.go
	}
	if k.gen != nil && k.gen.anyParadigm {
		s.offerParadigm(k, d, p, sorc, &av) // kw_paradigm.go
	}
	if k.gen != nil && k.gen.anyPrepare {
		s.offerPreparedCasts(k, d, p, sorc) // face_prepare.go
	}
	// activated abilities
	dynWas := s.g.availX
	if k.gen != nil && len(k.gen.varPC) != 0 {
		av.dynSac = true
		haveAv = false // rebuild the census with the dynamic units
	}
	haveAv = s.activateOffers(k, d, p, sorc, &av, haveAv)
	if k.gen != nil && k.gen.anyAnyAct {
		s.offerAnyActivations(k, d, p, sorc, &av, &haveAv) // cast_anyactivator.go
	}
	if k.gen != nil {
		s.priorityGen(k, d, p, sorc, &av, haveAv)
		if k.gen.anyFaceDown {
			s.offerTurnFaceUp(k, d, p) // CR 708.6: special action, no stack
		}
		if k.anyRoom && sorc {
			s.offerUnlocks(k, d, p) // CR 709.5e: special action (face_rooms.go)
		}
		s.g.availX = dynWas // the script walks below read their own modes
		av.dynSac = false   // the casts below leave the dynamic sacrifice source to its counter-cost gap
		haveAv = false
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
	if k.gen != nil && k.gen.anyNoManaCost {
		s.dropNoManaCostCasts(k, d) // CR 118.6 (cast_nomanacost.go)
	}
	if k.gen != nil && k.gen.anySplitSecond && s.splitSecondHolds(k) {
		s.filterSplitSecond(k, d) // kw_splitsecond.go
	}
	d.add(MakeAction(AkPass, 0, 0, 0))
}

// activateOffers appends p's printable activated abilities to d and reports
// whether av was built. The walk's census carries dynSac when the table has
// dynamic-amount variants (priorityOptions): an activated-ability offer
// prices a dynamic {T}, sacrifice source, unlike a cast.
func (s *State) activateOffers(k *prep, d *Decision, p uint8, sorc bool, av *avail, haveAv bool) bool {
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
				s.availFor(k, p, av)
				haveAv = true
			}
			if (a.Cost.Mana.X > 0 || a.Cost.XMin != 0) && s.cfg&cfgGorgeUnpriced != 0 || a.Cost.Mana.Snow != 0 {
				m := xFloorCost(a.Cost.Mana, costXMin(&a.Cost)) // (gorge's cost:x is X != 0 || XMin != 0 (pay.PlanCostDetail); an announced Sac<X> part without {X} or XMin is planned like any other cost (Krav, the Unredeemed's {B}); the pool pays the cheapest legal announcement; cost_sacx.go)
				if k.gen != nil && k.gen.anyPowerUp && a.Cost.Flags&kop.CostPowerUp != 0 {
					m = s.powerUpReduced(k, s.perms[sl].obj, m) // CR 702.193b: gorge prices the reduced cost (legal_walk_battlefield.go)
				}
				if k.gen != nil && k.gen.anyAbReduce {
					m = s.abReduced(k, a, sl, p, m) // the cost statics compose after it
				}
				if k.gen != nil && k.gen.anyWaterbend && waterbendIsX(a) {
					m = s.waterbendCredit(k, p, sl, a, m) // gorge mana.go: Waterbend<X>'s offer credit at the floor (act_waterbend.go)
				}
				if !s.poolPaysAct(k, p, &m, payDesc{class: pdAct, card: cid, src: sl}) {
					// gorge's planner declines an {X} activation (PlanCostDetail
					// cost:x, Domesticated Hydra's {X}{G}{G}{G}: Monstrosity X,
					// Fiend Artisan's hybrid {X}): it is offered only when the
					// floating pool alone pays it (Config.GorgeUnpriced). A hybrid
					// cost without {X} is planned (each pip resolved to a colour:
					// Puresight Merrow's {W/U}{Q}, kverify-c4).
					//
					// A {S} activation (CR 107.4h; PlanCostDetail cost:snow,
					// Boreal Centaur's {S}: Regenerate) is gated the same way
					// and on every surface: gorge never plans it, so it is
					// offered only when the pool's snow units pay it (poolPays
					// reads snowPays). With cfgGorgeUnpriced off gorge's search
					// surface would still plan the sources; the kernel keeps
					// the pool-only gate there too, a declared limitation of
					// the search surface (avail.affords never affords {S}).
					continue
				}
			}
			if a.Gate.Ctr == kop.CtrMaxSpeed && !s.maxSpeedPoolOK(k, p, sl, a) {
				continue // a max-speed grant is offered from the floating pool alone (kw_maxspeed.go)
			}
			if s.canActivate(k, sl, ai, a, sorc, av) {
				d.add(MakeAction(AkActivate, TCode(s.perms[sl].obj), 0, uint8(ai)))
				if st := DebugSnowStats; st != nil && a.Cost.Mana.Snow != 0 {
					st.Offered++
				}
			}
		}
	}
	return haveAv
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
	if a.XPip == 0 {
		a.XPip = b.XPip // the one XColor$ an added part may carry (CR 107.3)
	}
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

// onceUsed reports whether a once-per-game ability a has been activated for
// the permanent at sl. The card's first such ability is held in pfOnceGame;
// the second and third in the arena's mechOnce2/mechOnce3 slot masks (a pool
// with no card holding two of them uses neither, and mSl reads false).
func (s *State) onceUsed(k *prep, sl Slot, a *kop.Ability) bool {
	switch a.Once {
	case 2:
		return s.mSl(k, mechOnce2, sl)
	case 3:
		return s.mSl(k, mechOnce3, sl)
	default:
		return s.perms[sl].flags&pfOnceGame != 0
	}
}

// onceBlocked reports whether a's once-only mark refuses an activation for
// the permanent at sl: used, and not waived by an Activations static
// (static_turnbits.go). Every once gate, mana abilities included, reads it.
func (s *State) onceBlocked(k *prep, sl Slot, a *kop.Ability) bool {
	return s.onceUsed(k, sl, a) && !s.exhaustOnceWaived(k, s.perms[sl].ctrl, a)
}

// onceUse marks a's activation for the permanent at sl.
func (s *State) onceUse(k *prep, sl Slot, a *kop.Ability) {
	s.noteTurnExhaust(k, sl, a)
	switch a.Once {
	case 2:
		s.mSlSet(k, mechOnce2, sl)
	case 3:
		s.mSlSet(k, mechOnce3, sl)
	default:
		s.perms[sl].flags |= pfOnceGame
	}
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
	if s.castHeldOut(p.obj) {
		// gorge holds a card's activation out after its second no-progress
		// abort in a row (cast_commit.go abortCast, CR 733.2), on every
		// table: not only beside a waterbend card or an X-bound ability, the
		// families whose reversal the hold-out used to be read beside
		// (cast.go's push arm now captures bflNoProg for every activation)
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
		if s.activationSick(k, sl) {
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
	if !s.costTokenPartsPayable(k, p.ctrl, p.obj, &a.Cost) {
		return false // cost_millcost.go: life, CR 701.17b's library, CR 201.5's self-reveal
	}
	if f&kop.CostDiscard != 0 && !s.discardPayable(k, p.ctrl, &a.Cost, NoObj) {
		return false // cost_discardfiltered.go
	}
	if f&kop.CostSac != 0 && s.countCostSac(k, p.ctrl, &a.Cost, sl) < int(a.Cost.N) {
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
	if f&kop.CostLoyalty != 0 && (!sorc || s.loyaltyUsedAtLimit(k, sl)) {
		return false
	}
	if f&kop.CostOncePerGame != 0 && s.onceBlocked(k, sl, a) {
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
	poolOK := false
	if k.gen != nil && k.gen.anyPowerUp && f&kop.CostPowerUp != 0 {
		// The autopay planner prices the PRINTED cost composed with the cost
		// statics (planAbilityPayment calls offerCostFor, which folds the
		// RaiseCost/ReduceCost statics and the ability's own ReduceCost$; it
		// never calls powerUpReduceCost), and an ability whose composed
		// printed cost it cannot witness is not offered (powerUpWitness; gorge seed-1
		// Hercules: {4}{G} reduced to {2}, three untapped Forests, no offer).
		// The floating pool paying the reduced cost is enough: the plan is
		// then a pool choice, and gorge prices that pool through
		// pay.AvailableFor (manaFeasibleDescriptorKind, CR 106.6): the
		// restricted batches the activation may spend count with it -- a
		// CantCastNonArtifactSpells batch leaves every non-spell payment
		// class free (pay/payment.go), so it pays a Power-up activation
		// (powerup_offer_restrbatch_ignored: Hydraulic Helper's batch beside
		// Loki Laufeyson's {3}). So: offered iff that pool pays the reduced
		// cost or the sources pay the printed one AFTER the cost statics
		// (powerup_cost_static_planner_prices_reduced_print_torch: Hulk, Gamma
		// Goliath's {3} Power-up reduction makes Human Torch, Johnny Storm's
		// {6}{R} affordable from the sources at {3}{R}); the charge is always
		// the reduced one (buildCostCharge), and the plan/charge mismatch
		// falls back to the manual mana window (powerUpPlanRejected,
		// mana_window.go). On the pool branch the untapped sources stay out
		// of it -- gorge's pool branch prices the pool alone -- so the
		// sources read below is skipped when poolOK held.
		poolOK = s.poolPaysAct(k, p.ctrl, &mana, payDesc{class: pdAct, card: s.cardAt(sl), src: sl})
		if !poolOK && !s.powerUpWitnessAffords(k, sl, a, p.ctrl, av) {
			return false
		}
	}
	if sourcesPay(&mana) && !poolOK && !s.actSourcesAfford(k, sl, ab, a, mana, av) {
		return false
	}
	if mana.X == 0 && sacXCost(&a.Cost) && k.gen != nil && k.gen.anySacX {
		return s.sacXTargetsFeasible(k, a, p.ctrl, sl) // cost_sacx.go
	}
	return s.abilityTargetsFeasible(k, a, p.ctrl, sl, 0xff) && s.xBoundPayable(k, a, 0, p.ctrl, sl, &mana)
}

// actSourcesAfford is canActivate's untapped-sources read: the cheapest legal
// announcement (cost_sacx.go), the Waterbend credit, then the sources or the
// filter script pay it. It is the only mana read of an activation the
// floating pool does not already pay.
func (s *State) actSourcesAfford(k *prep, sl Slot, ab int, a *kop.Ability, mana kop.ManaCost, av *avail) bool {
	manaCost := xFloorCost(mana, costXMin(&a.Cost)) // the cheapest legal announcement (cost_sacx.go)
	if a.Cost.Flags&kop.CostWaterbend != 0 && k.gen != nil && k.gen.anyWaterbend {
		manaCost = s.waterbendCredit(k, s.perms[sl].ctrl, sl, a, manaCost)
	}
	if k.phyMana && manaCost.Phyrexian() {
		// offered while some colour / life mix of its pips is payable
		// (mana_phyrexian.go)
		return s.phyActAffords(k, sl, a, s.perms[sl].ctrl, manaCost, av)
	}
	return s.abilityAffords(k, sl, a, &manaCost, av) || s.actScriptPays(k, sl, ab, a)
}

// sourcesPay reports whether an activation's mana cost is priced against the
// untapped sources (canActivate). A {S} cost never is: gorge never plans it, and
// activateOffers gates it on the floating pool's snow units (snowPays).
func sourcesPay(m *kop.ManaCost) bool {
	return m.Snow == 0 && (m.MV() > 0 || m.X > 0)
}

// abilityAffords reports whether av pays the mana cost cost of ability a of
// slot sl: the source's own tap cannot fund its cost's mana (av2), and a
// tap/sacrifice mana cost must stay disjoint from the sources it spends.
func (s *State) abilityAffords(k *prep, sl Slot, a *kop.Ability, cost *kop.ManaCost, av *avail) bool {
	f := a.Cost.Flags
	pm := pipMaskOf(cost)
	own := s.g.availX&availOwn != 0 // gorge's potential walk prices the bound with the source's own mana in it (mana_ownwalk.go)
	if f&kop.CostTap != 0 && !own && s.canTapForMana(k, sl) {
		// the source can't also tap for its own cost's mana
		var av2 avail
		av2 = *av
		pc := &k.cards[s.cardAt(sl)]
		if k.gen != nil {
			pc = s.manaPCGen(k, sl)
		}
		if !s.ownTapBoundKept(k, sl) {
			s.removeSrc(k, &av2, sl, pc) // (the gaps walk's bound keeps the source's own mana: mana_reflamtowntap.go)
		}
		if !av2.affords(cost, pm, 0) && !(k.restrSrc && s.actRestrAffords(k, sl, a, cost, &av2, sl, s.perms[sl].ctrl)) && (!s.rbAny(s.perms[sl].ctrl) || !s.actPoolAffords(k, sl, cost, &av2)) {
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
	if f&kop.CostTap != 0 && !own {
		skip = sl
	}
	if !av.affords(cost, pm, 0) && !(k.restrSrc && s.actRestrAffords(k, sl, a, cost, av, skip, s.perms[sl].ctrl)) && (!s.rbAny(s.perms[sl].ctrl) || !s.actPoolAffords(k, sl, cost, av)) {
		return false
	}
	if f&kop.CostTapX != 0 || f&kop.CostSac != 0 && k.sacMana {
		return s.costManaDisjoint(k, sl, a, av, pm)
	}
	return true
}

// availNoActScript is an availX mode bit for scriptPrefix's "does the plain
// walk list this play" probe: the listing it reads is the plain census's own,
// not the one actScriptPays adds, so a play only the filter script pays still
// gets that script played at its prefix (CR 601.2g-h) rather than reaching the
// mana step with no plan.
const availNoActScript uint16 = 2048

// availNoHandFund is a mode bit read only by filtScript's entry: the search is an
// activation's payment (actScriptPays), which gorge's planner prices from the census,
// and the census prices no hand source (mana_handguide.go). The play's prefix search
// (scriptOffers) keeps the guides.
const availNoHandFund uint16 = 4096

// actScriptPays prices an activation whose plain census is short against the
// filter script (mana_filter.go filtScript), the route scriptOffers gives a
// play the census itself lists: on a board with a first-only filter beside
// plain mana — Viridescent Bog's "{1},{T}: Add {B}{G}" beside the Prism
// token's tap-sacrifice in the repro — the offer census counts neither
// (availFilt's filtOnly/mixBoard gate, mana_filter.go), while gorge's planner
// prices the chain: the Prism's mana pays the Bog's fee, the Bog's {B}{G} pays
// the activation (CR 605.3a-b, mana_filterfirst.go filtFirstAdmit). The search
// is the payment itself, so its verdict is exact — it pays the equip {2} and
// refuses a {3} one.
//
// A board holding a net-negative filter (fee above production) keeps the older
// scripts' answer: the fallback declines wholesale, because the search prices
// that fee as part of the payment (the Orzhova board's Energy Refractor ran
// the filter search inside its own search until the cap). That withholds a
// mixed board's play which its positive filters could fund; the census/prefix
// route owns only some of those shapes. Narrowing the gate needs a search that
// skips the net-negative filters, not a board-wide test.
//
// Only a plain printed cost is priced (plainPricedCost): an X, hybrid, snow,
// Phyrexian, own-reduce, power-up, waterbend, tap-X or sacrifice cost needs
// the asks the build poses first, so the offer filtScript priced (a target-0
// action) is not the plan the play's mana step needs (the Mana Rig's {X}{X}{X}).
func (s *State) actScriptPays(k *prep, sl Slot, ai int, a *kop.Ability) bool {
	// A table without a generated extension has k.gen == nil (prep.go), so
	// scriptFilt would dereference nil. A granted/gained ability's index (255)
	// names no printed ability, so its MakeAction could never list.
	if k.gen == nil || ai >= 255 || a.Kind == kop.AbMana || !plainPricedCost(a) || s.g.availX&availNoActScript != 0 || s.pool != nil && s.pool.inFilt != 0 {
		return false
	}
	p := s.perms[sl].ctrl
	f := s.scriptFilt(k, p)
	if !f.any() || s.filtNetNegative(k, f) != 0 {
		return false
	}
	pa := MakeAction(AkActivate, TCode(s.perms[sl].obj), 0, uint8(ai))
	if handManaOn(k) && s.scriptRelax(k, p, f, false) && !s.relaxedWalkLists(k, pa) {
		return false
	}
	av := s.g.availX
	s.g.availX = av | availNoHandFund
	ok := s.filtScript(k, pa, p, false)
	s.g.availX = av
	return ok
}

// relaxedWalkLists reports whether the relaxed census's walk (the gaps walk)
// lists the activation a. On a relaxable census gorge's relaxed proof
// (paymentPlanRelaxProof) reads no hand mana: a play it cannot pay is
// "insufficient" and runs no prefix search, so a Spirit Guide's {R} never
// funds a search that the relaxed filters' colours leave short (Golgari
// Signet's {B}{G} beside Plains and a Simian Spirit Guide: Goblin
// Fireleaper's {1}{R} pump, CR 605.1a). The filters' subsets matter only for
// a net-negative one, which actScriptPays has already declined.
func (s *State) relaxedWalkLists(k *prep, a Action) bool {
	// actScriptPays runs under an outer walk (canActivate), and plainOptions
	// ends with availX = 0: restore the outer walk's census and filter skip.
	x, sk := s.g.availX, s.filtSkip()
	var d Decision
	s.setFiltSkip(0)
	s.plainOptions(k, &d, availGaps|availNoActScript)
	s.g.availX = x
	s.setFiltSkip(sk)
	return hasOption(&d, a)
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
	discardOK := ac.Flags&kop.CostDiscard == 0 || others >= int(ac.N)
	if ac.Flags&kop.CostDiscardFiltered != 0 {
		// a filtered discard (Magmatic Insight's land card) never takes the
		// card being cast, whatever its filter says (cost_discardfiltered.go)
		discardOK = s.discardPayable(k, p, ac, o)
	}
	// the card cast is on the stack when its exile cost is paid, so it is no
	// fodder for it (Skaab Ruinator cast from the graveyard, CR 601.2h)
	if s.pickCostPayable(k, p, ac, o) && discardOK && s.beholdPayable(k, p, c, o) {
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
	if mode&1 != 0 || s.FeatPlannedCast(p, o, mode) || s.FeatPoolPays(p, o, mode) { // seat-ok: a cast-mode bit
		return true
	}
	if k := s.k(); mode == 0 && s.g.availX&(availGaps|availAny) != 0 && k.gen != nil && k.gen.uses.has(useLifeGap) && s.lifeGapPriced(k, p, o) {
		return true // gorge's planner prices it with a life-only source's last-resort unit (mana_lifegap.go)
	}
	if k := s.k(); mode == 0 && k.gen != nil && k.gen.anySacX && s.sacXPoolOffer(k, p, o) {
		return true // the floating pool pays it at some announced Sac<X> count (cost_sacx.go)
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
// anyColor is the permission's MayPlayIgnoreColor rider, which gorge's
// offer gate prices the cast under (mayPlayAffordable).
func (s *State) gorgeOffersMay(p uint8, o Obj, anyColor bool) bool {
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) == 0 {
		return true
	}
	if s.FeatPlannedCast(p, o, 0) {
		return true
	}
	return s.FeatPoolPays(p, o, 0) || s.gorgeLowersRider(p, o, true, anyColor)
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
	return s.gorgeLowersRider(p, o, mayPlay, false)
}

// gorgeLowersRider is gorgeLowers for a may-play cast under the any-colour
// rider when anyColor: a target-priced cast's composed price is funded with
// its coloured pips as generic (pay.PlanManaUnderRider), so Vanish into
// Eternity's {2}{W} at a noncreature target is paid by three red and blue
// sources (CR 601.2f, 609.4b).
func (s *State) gorgeLowersRider(p uint8, o Obj, mayPlay, anyColor bool) bool {
	k := s.k()
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	if s.contribPays(k, p, c, o, s.objs[o].zone == kop.ZoneHand) {
		return false // gorge's potential planner refuses a convoke cost
	}
	if s.waterbendOwnPart(k, p, c, o) != 0 {
		return false // ... and a waterbend one (potentialModeCastPlan, "shape:contribution")
	}
	if handled, lower := s.twobridLowers(k, p, c, o, anyColor); handled {
		return lower
	}
	if (cd.HasMultikicker || cd.HasEscalate) && !mayPlay && s.g.availX&(availAny|availGaps) != 0 {
		// a plain Multikicker or Escalate hand cast is "shape:optional_cost" to the cast
		// planner; gorge keeps the composition's plan (potentialPlayVerdictShared)
		// but not its "insufficient", so the cast never enters the prefix search
		// (potentialScriptPlays) and a census gap (a depletion land's counter
		// mana) never pays it: it is no insufficient play and no scripted one
		return false
	}
	if cd.Cost.X != 0 {
		if !mayPlay && s.xTgtDepDeclined(k, p, c, o) {
			return false
		}
		return s.xCostLowers(k, p, c, o)
	}
	if sacXCost(spellAddCost(k, c)) {
		return false // an announced Sac<X/Spec>: the potential planner's composition declines it too, so only the floating pool offers it (cost_sacx.go)
	}
	if cd.HasAltAdd {
		cost := s.altAddPlanCost(k, p, c, o)
		if s.g.availX&availGaps != 0 {
			return s.affordsFor(k, p, &cost) // with the mana a scripted prefix makes (scriptOffers)
		}
		if s.ungrantedPaysNoDH(k, p, c, o, &cost) { // the Diamonds alone fund it: gorge's planner prices no plan for a hand play
			return true
		}
		// the planner's census has no granted source, so a cost only a granted ability
		// pays is a census gap: gorge's prefix search taps it first and the cast is
		// offered once the mana floats (Deadly Precision's {B} from Great Divide Guide's
		// grant, the guide also the creature the cast sacrifices; CR 601.2g before 601.2h).
		// A filter-only board is a script gap as well: the prefix search activates the
		// filter first (Llanowar Envoy's {1}{G} makes the {B} Bone Shards costs; CR 601.2g)
		return k.gen != nil && (len(k.gen.grantPC) != 0 || s.scriptSourceLive(k, p) || k.gen.anyFilt && s.filtOnly(k, p)) && s.affordsFor(k, p, &cost)
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
	if ac := spellAddCost(k, c); !mayPlay && ac.Flags == kop.CostPayLife && lifeCostOwed(ac) > 0 {
		// a fixed PayLife<N> additional cost (Withering Boon's 3 life): the cast
		// planner declines it and the potential planner prices it only when the
		// printed sources pay the mana ("shape:additional_cost"), with no prefix
		// search for a filter land its census cannot price, whose fee another
		// source funds (Viridescent Bog beside a Swamp; CR 601.2g-h)
		cost := s.featCost(k, p, c, o, k.cards[c].cost)
		return s.ungrantedPaysNoDH(k, p, c, o, &cost) // the Diamonds alone fund it: gorge's planner prices no plan for a hand play
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
		return s.ungrantedPaysNoDH(k, p, c, o, &cost) || // the Diamonds alone fund it: gorge's planner prices no plan for a hand play
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
			cost = s.mayRiderCost(cost, anyColor)
			if !several {
				// gorge's potential planner declines the target-dependent cast's
				// prefix search (potentialTargetPriced), so only its census pays:
				// no granted source, as the hand arm below (Song of Freyalise's
				// creature mana beside Esior's raise, Arrow Storm from exile)
				return s.ungrantedPaysNoDH(k, p, c, o, &cost)
			}
			if s.offerSourcesAfford(k, p, c, o, &cost) {
				return true
			}
			cost = s.featCost(k, p, c, o, k.cards[c].cost)
			cost = s.mayRiderCost(cost, anyColor)
			// the retry taps no source: the pool (and the restricted mana it
			// may spend) pays the reduced price on its own
			a := s.poolAvailPlus(p, nil)
			if s.rbAny(p) {
				s.restrAvail(k, SeatIx(p), c, s.restrOriginOfObj(o), &a)
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
		cost = s.mayRiderCost(cost, anyColor)
		// the planner's census (potentialTargetPricedPlan): no granted source
		// (Imprisoned in the Moon's {C}), and a painful source makes its
		// colours while one point of damage is not lethal (Starting Town's W)
		return s.ungrantedPaysNoDH(k, p, c, o, &cost) // the Diamonds alone fund it: gorge's planner prices no plan for a hand play
	}
	if cd.BeholdExile || (cd.HasMultikicker || cd.HasEscalate) && !mayPlay || k.gen != nil && k.gen.anyOptCost && optCostCard(k, c) {
		return s.optCostCastLowers(k, p, c, o, mayPlay)
	}
	if cd.HasModeCost && !mayPlay && !s.modalFloorFree(k, p, c) {
		// a Spree or Tiered face whose cheapest mode still costs mana: the
		// cast planner declines the shape ("shape:modal_cost") and the
		// potential composition prices the cast only at its printed cost
		// (potentialModeCastPlan), which is a witness just when the
		// planner's census -- the printed sources and the pool, not a granted
		// ability -- pays it; no free mode makes that price the cast's
		// cheapest legal total (potentialModalCastFreeMode, potential_plan.go).
		// A may-play cast is composed at its printed cost by that arm's own
		// mayplay case, so it keeps its gate.
		cost := s.featCost(k, p, c, o, k.cards[c].cost)
		return s.ungrantedPaysNoDH(k, p, c, o, &cost) // the Diamonds alone fund it: gorge's planner prices no plan for a hand play
	}
	return s.plainHandPlayLowers(k, p, c, o)
}

// xCostLowers is gorgeLowersRider's {X}-cost arm. An additional-cost shape's
// plain cast is declined first when the composition's census read says no:
// gorge's potential planner declines the shape ("shape:additional_cost",
// paymentPlanCastShapeDetail's PlanNonManaAdmissible arms) and prices the
// composition only on its census -- the printed sources and the pool, never a
// gap -- so an insufficient one drops the play and no prefix search runs for
// it (potentialPlayVerdictShared). The census pays the cheapest announcement
// X = 0, as the fixed additional-cost arms below do.
func (s *State) xCostLowers(k *prep, p uint8, c CardID, o Obj) bool {
	if addCostShapeUnscripted(k, c) {
		cost := s.featCost(k, p, c, o, k.cards[c].cost)
		if s.g.mayPlayAny {
			cost = anyType(cost)
		}
		if !s.ungrantedPaysNoDH(k, p, c, o, &cost) {
			return false
		}
	}
	// a source that sacrifices itself can use up what the part must sacrifice,
	// as for the fixed-cost arm in gorgeLowersRider (Reshape's only artifact a
	// Treasure that also pays {U}; CR 601.2g-h, gorge planReserving). The
	// census pays the cheapest announcement X = 0.
	if ac := spellAddCost(k, c); ac.Flags&kop.CostSac != 0 && k.sacMana {
		cost := s.featCost(k, p, c, o, k.cards[c].cost)
		if !s.spellPartDisjoint(k, p, c, o, ac, &cost) {
			return false
		}
	}
	return s.xWitnessOffered(k, p, c, o)
}

// optCostCastLowers is gorgeLowersRider for a Behold / Multikicker / Escalate /
// optional-cost cast.
func (s *State) optCostCastLowers(k *prep, p uint8, c CardID, o Obj, mayPlay bool) bool {
	// the Diamond gaps are not the census's units either (dhWithholdsCast):
	// a hand play only they fund gets no prefix, so gorge prices no plan
	// for it either, before the printed-sources read can count them
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	if s.dhWithholdsCast(k, p, o, &cost) {
		return false
	}
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
	// A plain Multikicker hand cast is the same: its granted source (Gemhide
	// Sliver's any-colour mana) is a gap in the planner's census too, and no
	// prefix search taps it (the availAny/availGaps rule above).
	return mayPlay && s.mayPlayPrefixPays(k, p, c, o)
}

// plainHandPlayLowers is gorgeLowersRider for an ordinary cast.
func (s *State) plainHandPlayLowers(k *prep, p uint8, c CardID, o Obj) bool {
	// an ordinary cast's plain hand play is the planner's own: the census (the
	// printed sources and the pool) pays it — the Diamond gaps are no units of
	// that census and its prefix search never funds a hand play, so a hand cast
	// only they fund is unpriced (dhWithholdsCast, potential_plan.go)
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	if s.g.mayPlayAny {
		cost = anyType(cost)
	}
	if s.dhWithholdsCast(k, p, o, &cost) {
		return false
	}
	// gorge's potential planner prices the play only through its witness or a
	// scripted prefix, each replayed before it is offered; a plan paid only by
	// consuming the play's own targets is no way to pay it
	// (potential_plan.go potentialWitnessReaches, cast_tgtkeep.go,
	// mana_sacplaintarget.go)
	return s.plainSacKeepsTarget(k, p, c, o, &cost) && s.witnessKeepsTargets(k, p, c, o, &cost)
}

// twobridLowers is gorgeLowersRider's twobrid arm (CR 107.4e): the cast
// planner resolves the pips and prices the printed census, so a cast only a
// granted or scripted source pays (or one a face-sensitive modifier reprices
// below the planner's face-blind price) is the potential route's.
func (s *State) twobridLowers(k *prep, p uint8, c CardID, o Obj, anyColor bool) (handled, lower bool) {
	cd := &k.t.Cards[c]
	if cd.Cost.Hyb[0] != kop.HybTwobrid || cd.Cost.Phyrexian() {
		return false, false
	}
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	cost = s.mayRiderCost(cost, anyColor)
	d := payDesc{class: pdSpell, card: c, og: s.restrOriginOfObj(o)}
	if s.affordsUngrantedCast(k, p, &cost, d) {
		return true, s.twobridSpentReader(k, p, o) // the planner prices it; FeatPlannedCast offers it
	}
	return true, k.gen != nil && (len(k.gen.grantPC) != 0 || s.scriptSourceLive(k, p) || k.gen.anyFilt && s.filtOnly(k, p)) && s.affordsFor(k, p, &cost)
}

// twobridSpentReader: the cast planner declines the twobrid spell only for a
// mana-spent reader ("shape:mana_spent_reader"), which gorge's potential
// planner prices from the composed cost with its pips resolved
// (manaSpentReaderComposed, planTwobridCost): the play is the potential
// route's, not FeatPlannedCast's (Magmablood Archaic's Converge).
func (s *State) twobridSpentReader(k *prep, p uint8, o Obj) bool {
	if _, ok := s.plannedCastCost(k, p, o, 0); ok {
		return false
	}
	_, ok := s.plannedCastCostOf(k, p, o, 0, false, true)
	return ok
}

// scriptSourceLive: p has a source the planner's census leaves a gap (an
// each-colour source, restricted mana, a tap-another or counter source), which
// gorge's prefix search taps before it prices the play again (Bloom Tender's
// colours beside a Swamp pay Bogslither's Embrace's {1}{B}; CR 601.2b, 601.2f).
func (s *State) scriptSourceLive(k *prep, p uint8) bool {
	return s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapXModelled(k, p)).or(s.scriptCtrModelled(k, p)).or(s.scriptCtr(k, p)).any() || k.gen.anyComboAny && s.caBoard(k, p)
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

// mayRiderCost is cost under the may-play cast's mana rider: the any-type
// one (s.g.mayPlayAny) or, when anyColor, the any-colour one.
func (s *State) mayRiderCost(cost kop.ManaCost, anyColor bool) kop.ManaCost {
	if s.g.mayPlayAny {
		return anyType(cost)
	}
	return planUnderRider(cost, false, anyColor)
}

func manaEmpty(m *kop.ManaCost) bool { return m.MV() == 0 && m.X == 0 }

// severalTargets: card c's spell chooses more than one target.
func severalTargets(k *prep, c CardID) bool {
	t := tspec(k, c, spellAb(k, c), 0)
	return t != nil && t.Max > 1
}

// altAddFreePart: card c's either-or additional cost has a part with no mana
// that can be paid (a creature to sacrifice). The candidates exclude the card
// being cast (o): for a cast the card is on the stack while its cost is paid,
// so it can never pay its own exile or sacrifice fodder (gorge pay
// NonManaCastable's cast self-skip, CR 601.2a/h) -- a may-play cast from the
// graveyard (Ark of Hunger's permission) whose only other grave card is a
// third one must price the mana part, not the exile part.
func (s *State) altAddFreePart(k *prep, p uint8, c CardID, o Obj) bool {
	cd := &k.t.Cards[c]
	ac := spellAddCost(k, c)
	return manaEmpty(&ac.Mana) && s.pickCostPayable(k, p, ac, o) && (ac.Flags&kop.CostDiscard == 0 || int(s.pl[p].handN)-s.inHandN(o) >= int(ac.N)) && s.beholdPayable(k, p, c, o) && s.altAddPartDisjoint(k, p, c, o, ac) ||
		manaEmpty(&cd.AltAddCost.Mana) && s.pickCostPayable(k, p, &cd.AltAddCost, o) && s.altAddPartDisjoint(k, p, c, o, &cd.AltAddCost)
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
	if s.pickCostPayable(k, p, ac, o) && (ac.Flags&kop.CostDiscard == 0 || int(s.pl[p].handN)-s.inHandN(o) >= int(ac.N)) && s.beholdPayable(k, p, c, o) {
		return true
	}
	if !s.pickCostPayable(k, p, &cd.AltAddCost, o) {
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
// from the graveyard or exile: o is on the stack by the time its targets are
// chosen, so it is no candidate for them (CR 115.5, 601.2a; gorge
// castTargetsAvailable names the cast card in every zone).
// Flashback under The Dawning Archaic with no other instant or sorcery in the
// graveyard has no target and is not offered.
func (s *State) spellTargetsFeasibleFrom(k *prep, c CardID, p uint8, o Obj) bool {
	if z := s.objs[o].zone; z != kop.ZoneGraveyard && z != kop.ZoneExile {
		if z == kop.ZoneHand {
			return s.spellTargetsFeasibleHand(k, c, p, o) // cast_handleaves.go
		}
		return s.spellTargetsFeasible(k, c, p)
	}
	s.g.tgtSelf = o + 1
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
		srcObj := NoObj
		if src != NoSlot {
			srcObj = s.perms[src].obj
		}
		for m := 0; m < int(a.NModes); m++ {
			if k.gen != nil && k.gen.anyCharmPick && s.charmPickBarred(a, srcObj, m) {
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
		if t.UpTo || t.TFlags&kop.TgfAtResolution != 0 || slotNoneUnkicked(t) {
			continue // (a kicked-only slot takes no object in the cast this gate prices: Tear Asunder's nonland permanent, CR 601.2c)
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
	if a.NTargets > 1 && !s.distinctCastFeasible(k, a, p, src, modes, excl, false) {
		return false // TargetUnique$ chain: jointly distinct (tgt_uniqueoffer.go)
	}
	return true
}

// hasTargetBut is hasTarget with the permanent but not counted.
func (s *State) hasTargetBut(k *prep, t *kop.TargetSpec, p uint8, src Slot, but Obj) bool {
	var buf [MaxOptions]TCode
	n := s.targetCands(k, t, p, src, but, &buf)
	for i := 0; i < n; i++ {
		if buf[i] != TCode(but) {
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

// DebugTgtAnyPlDmgNoGate drops the seat arm's wasDealtDamageThisTurn gate
// (targetOK's TgtAny branch): gorge still gates the seat on its own record,
// so a pool that reaches a Needle Drop target ask with a damaged seat
// diverges. Test-only; dirac/oracle mutation tests set it.
var DebugTgtAnyPlDmgNoGate bool

// anySeatOK reports whether seat pp is a legal player choice for p's TgtAny
// slot t: not p itself under TgfOppOnly, and dealt damage this turn when the
// slot's filter gates the seat arm (anySeatGated).
func (s *State) anySeatOK(k *prep, t *kop.TargetSpec, p, pp uint8) bool {
	if t.TFlags&kop.TgfOppOnly != 0 && pp == p {
		return false
	}
	return DebugTgtAnyPlDmgNoGate || !anySeatGated(k, t) || s.g.turnF&turnDamagedMask(pp) != 0
}

// anySeatGated reports whether a TgtAny slot's filter gates its player arm:
// wasDealtDamageThisTurn gates BOTH arms through the one flag
// (kop.F2WasDamaged). The object arm reads dmgTurn (filters_gen.go
// flags2Perm), the seat arm the seat's own record (genState.turnF's
// turnDamagedShift bits; gorge state.Player.WasDealtDamageThisTurn, Needle
// Drop's ValidTgts$ Any.wasDealtDamageThisTurn). Filter 0 (bare Any) carries
// no flag and gates nothing.
func anySeatGated(k *prep, t *kop.TargetSpec) bool {
	return t.Filter != 0 && k.t.Filters[t.Filter].Flags2&kop.F2WasDamaged != 0
}

// targetOK reports whether code is a legal choice for spec.
func (s *State) targetOK(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, code TCode) bool {
	if t.ParentPlus1 != 0 && !s.parentTargetOK(t, code) {
		return false
	}
	if isPlayerTarget(code) {
		pp := CodeSeat(code)
		if pp != p && k.gen != nil && (k.gen.anyRepl&rbPlHex != 0 || k.gen.anyEmbPlHex) && s.playerHexproof(k, pp) {
			return false
		}
		if s.plProtAll(k, pp) {
			// CR 702.16b: the protected player cannot be the target of
			// anything, their own spells included (no hexproof-style
			// self-exemption; gorge stack.go/target_legal.go's
			// playerProtectedFrom arm)
			return false
		}
		switch t.Kind {
		case kop.TgtAny:
			return s.anySeatOK(k, t, p, pp)
		case kop.TgtPlayer:
			if t.TFlags&kop.TgfPlayerCompareOK != 0 && !s.playerThanActiveOK(k, t.TFlags, pp) {
				return false // tgt_thanactive.go
			}
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
	if t.Kind == kop.TgtGraveExileCard {
		return s.targetOKGraveExileCard(k, t, p, src, self, o) // tgt_graveexilecard.go
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
		if ob.zone != kop.ZoneStack || o == self || o+1 == s.g.tgtSelf {
			// CR 115.5: a spell on the stack is an illegal target for itself;
			// tgtSelf names the spell being built whose feasibility sweep runs
			// after its push, the cast card gorge's targetSAAvailable excludes
			// (xask_self_spell_target: the cmcEQX ask of Spell Blast's X)
			return false
		}
		return s.matchCardAt(k, t.Filter, ob.card, s.stackCtrl(o), p, o) && (t.TFlags&kop.TgfSingleTgt == 0 || s.ksrSingleSpell(o))
	case kop.TgtCard:
		z := k.t.Filters[t.Filter].Zone
		if z == kop.ZoneNone {
			z = kop.ZoneGraveyard
		}
		if z == kop.ZoneGraveyard && s.graveCantTargetGen(k) {
			return false // Ground Seal's class (kw_canttgtgrave.go)
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
func (s *State) targetCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]TCode) int {
	n := 0
	if t.Kind == kop.TgtAny || t.Kind == kop.TgtPlayer {
		// every seat still in the game, in seat order (gorge offers
		// AliveFrom(0), target_legal.go)
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if !s.out(pp) && s.targetOK(k, t, p, src, self, PlayerCode(pp)) {
				out[n] = PlayerCode(pp)
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
			if s.targetOK(k, t, p, src, self, TCode(o)) {
				out[n] = TCode(o)
				n++
			}
		}
	case kop.TgtSpell, kop.TgtSpellTgting:
		for i := int(s.sp) - 1; i >= 0; i-- {
			e := &s.stack[i]
			if e.obj+1 == s.g.tgtSelf {
				continue // the spell being built (CR 115.5; hasTarget's arm)
			}
			if e.kind == seSpell && s.targetOK(k, t, p, src, self, TCode(e.obj)) {
				out[n] = TCode(e.obj)
				n++
			}
		}
	case kop.TgtStackObj, kop.TgtStackAb, kop.TgtCopyAb, kop.TgtCopyTrig, kop.TgtStackObjTrig, kop.TgtStackTgting:
		n = s.stackTgtCands(k, t, p, self, out, n) // tgt_stackability.go
	case kop.TgtPermOrSpell:
		n = s.permOrSpellCands(k, t, p, src, self, out, n) // tgt_permorspell.go
	case kop.TgtPermOrGraveCard:
		n = s.permOrGraveCardCands(k, t, p, src, self, out, n) // tgt_permorgravecard.go
	case kop.TgtGraveExileCard:
		n = s.graveExileCardCands(k, t, p, src, self, out, n) // tgt_graveexilecard.go
	case kop.TgtCard:
		for pp := 0; pp < int(s.nSeats()); pp++ {
			if s.out(uint8(pp)) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if o+1 == s.g.tgtSelf {
					continue
				}
				if s.targetOK(k, t, p, src, self, TCode(o)) && n < MaxOptions-1 {
					out[n] = TCode(o)
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
		// a player is always a legal any-target (Bolt), unless the slot's
		// filter gates the player arm too (anySeatGated: Needle Drop's census
		// runs like any filtered slot's).
		if t.TFlags&kop.TgfOppOnly == 0 && !anySeatGated(k, t) {
			return true
		}
	case kop.TgtObject:
		for m := s.occ; m.any(); m = m.rest() {
			if s.targetOK(k, t, p, src, self, TCode(s.perms[m.first()].obj)) {
				return true
			}
		}
		return false
	}
	var buf [MaxOptions]TCode
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
	pf, gf, pPot, gPot, printedOK, ok := s.fbPotentials(k, p, o)
	if !ok {
		return false, false, false
	}
	if s.g.fbProbe {
		pPot, gPot = true, true // the root's classification (fbProbeRoot), not this node's thinner board
	}
	var av avail
	s.availFor(k, p, &av)
	if printedOK && s.castAffordsR(k, p, s.objs[o].card, s.restrOriginOfObj(o), &pf, &av) {
		// the printed instance is the planner's planned flashback cast,
		// offered whatever the potential walk says (Think Twice, pair 90106).
		// Paid as a hand cast is, restricted sources counted (castAffordsR):
		// Hermitic Herbalist's two Lesson mana pay Solstice Revelations'
		// {6}{R} beside Iroh's granted {1}
		return true, false, false
	}
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 {
		if pPot && gPot && !s.g.fbProbe {
			// the planner drops both as ambiguous, but gorge's walk offers an
			// instance the floating pool pays outright (Fire Nation Attacks'
			// granted {4}{R} beside its printed {8}{R}, pool-paid). Not in a filter
			// script's trial node (fbProbe): its pool is the prefix's, and gorge
			// prices the script's play, the printed instance, from it
			var pool avail
			s.availPool(p, &pool)
			if pool.affords(&gf, pipMaskOf(&gf), 0) {
				return true, false, true
			}
		}
		return true, pPot && gPot, !pPot
	}
	// gorge's own decision lists both: the kernel's one cast takes the
	// granted instance when its sources cannot pay the printed one
	return true, false, true
}

// fbPotentials prices the two flashback instances of o (see fbInstances) against
// gorge's potential pool: both costs adjusted for this spell, whether each is
// affordable, and whether the printed one's non-mana part is payable. ok is false
// when o has no distinct granted instance beside its printed one.
func (s *State) fbPotentials(k *prep, p uint8, o Obj) (pf, gf kop.ManaCost, pPot, gPot, printedOK, ok bool) {
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	marked := k.gen != nil && s.fbGranted(k, o)
	if k.gen == nil || k.cards[c].kw&kop.KwFlashback == 0 || !marked && !s.fbLive(k, o) {
		return
	}
	gc := s.fbGrantCost(k, p, o, marked) // the granted instance's cost (kw_fbgrant.go)
	if gc == cd.AltCost {
		return
	}
	// gorge prices each instance for this spell: a source whose restriction
	// refuses it adds nothing (Secluded Courtyard's any colour for an instant)
	spell := payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}
	var pa avail
	s.availPotentialFor(k, p, &pa, &spell, false)
	pf = s.adjustCost(k, p, c, o, cd.AltCost)
	gf = s.adjustCost(k, p, c, o, gc)
	pPot, gPot = pa.affords(&pf, pipMaskOf(&pf), 0), pa.affords(&gf, pipMaskOf(&gf), 0)
	// a printed non-mana part (Group Project's tap three creatures) the
	// census cannot pay leaves the printed instance uncastable whatever its
	// mana: gorge offers the granted one (CR 702.34a)
	printedOK = (cd.FbCost.Flags == 0 || s.pickCostPayable(k, p, &cd.FbCost, o)) && (!cd.FbBehold || s.beholdPayable(k, p, c, o))
	return pf, gf, pPot && printedOK, gPot, printedOK, true
}

// fbProbeRoot reports whether a filter script's play a is a flashback cast whose
// printed and granted instances gorge both lists at s (fbInstances: potential
// pool affords both). The script's trial nodes keep that classification
// (fbProbe): the play gorge prices from a prefix's pool is the printed
// instance, and a node's thinner board must not turn the granted one into it.
func (s *State) fbProbeRoot(k *prep, p uint8, a Action) bool {
	if a.Kind() != AkCast || a.Mode()&3 != 2 || s.objs[Obj(a.Source())].zone != kop.ZoneGraveyard {
		return false
	}
	_, _, pPot, gPot, _, ok := s.fbPotentials(k, p, Obj(a.Source()))
	return ok && pPot && gPot
}

// availPotential is gorge's potential pool (rules PotentialMana) as an
// avail: the floating pool, and what each untapped source could produce. It is a
// deliberate over-bound: a source adds one unit of every colour it can make, and
// an open production (any colour, an unread chosen colour) is unlimited.
func (s *State) availPotential(k *prep, p uint8, a *avail) {
	s.availPotentialFor(k, p, a, nil, false)
}

// availPotentialBoundedFor is availPotentialFor's exact any-colour mode.
func (s *State) availPotentialBoundedFor(k *prep, p uint8, a *avail, d *payDesc) {
	s.availPotentialFor(k, p, a, d, true)
}

// potUnit is one production of the potential walk as a real plan pays it:
// n taps or activations, each making one mana of any of mask's colours (a
// "Combo U B" dual's choice, CR 118.3c). The over-bound fold prices the same
// production once per colour. pick marks the same-pick shape a reflected
// TapsForMana extra gives a fixed several-colour tap: ONE tap makes n mana
// all in one colour of mask (the pick), not n independent units -- the extra
// is one more OF THE PICK (CR 605.3b, kgen's StTapReflect doc; gorge
// planalts.go addTapExtra adds it per declared production), so
// potUnitsAvail prices such a unit as a one-colour source beside its wide
// backing.
type potUnit struct {
	mask uint8
	n    uint8
	// pain is one activation's life-plus-damage (abPainOf), saturated; the
	// per-tap read caps the unit's count at what the lethal guard lets one
	// plan take (potUnitsAvail)
	pain uint8
	// pick marks a one-tap unit of n mana all in one colour of mask (a
	// reflected extra over a several-colour production, growTap). It never
	// merges with another record (two picks price two taps, not one tap of
	// the sum) and potUnitsAvail skips the pain count cap for it (a one-tap
	// unit's count is one, not n).
	pick bool
}

// maxPotUnits bounds the units availPotentialFor records for
// potUnitsAvail: one per production of the walk (a source's own, its granted
// ones, the fixpoint's), so a board with more productions leaves the record
// marked incomplete and potUnitsAvail reports no exact view (the caller
// keeps the over-bound answer).
const maxPotUnits = 32

// recordPotUnit appends mask/n, merging into the unit of the same mask and
// pain when both are unpicked; the full record (more productions than fit) is
// the overflow marker units[0] with a zero count.
func recordPotUnit(units *[maxPotUnits]potUnit, n *int, mask uint8, cnt, pain uint8, pick bool) {
	if units[0].n == 0 && units[0].mask != 0 {
		return // already overflowed: keep the marker
	}
	if mask == 0 || cnt == 0 {
		return
	}
	for i := 0; i < *n; i++ {
		if units[i].mask == mask && units[i].pain == pain && !units[i].pick && !pick {
			if s := int(units[i].n) + int(cnt); s < 256 {
				units[i].n = uint8(s)
			} else {
				*units = [maxPotUnits]potUnit{{mask: 1}} // the count overflows uint8: the incomplete marker
			}
			return
		}
	}
	if *n >= maxPotUnits {
		*units = [maxPotUnits]potUnit{{mask: 1}} // the overflow marker
		return
	}
	units[*n] = potUnit{mask: mask, n: cnt, pain: pain, pick: pick}
	*n++
}

// unrecordPotUnit takes cnt of mask's taps back out of the record (a source's
// tap production the caller folds into a wider one, tapUnit); an overflowed
// record keeps its marker and a unit left empty is dropped.
func unrecordPotUnit(units *[maxPotUnits]potUnit, n *int, mask uint8, cnt, pain uint8, pick bool) {
	if units[0].n == 0 && units[0].mask != 0 {
		return // overflowed: the record is incomplete anyway
	}
	for i := 0; i < *n; i++ {
		if units[i].mask != mask || units[i].pain != pain || units[i].pick != pick {
			continue
		}
		if units[i].n > cnt {
			units[i].n -= cnt
			return
		}
		copy(units[i:*n], units[i+1:*n])
		*n--
		units[*n] = potUnit{}
		return
	}
}

// potCensus is availPotentialFor's per-tap record: the units (potUnit) and
// each source's tap production. A source taps once, so its further {T}
// abilities widen that one unit's colours instead of adding units (CR 118.3c;
// gorge's planner makes one mana per tap: Lupinflower Village's {C} beside its
// restricted {W}).
type potCensus struct {
	on    bool
	n     int
	units [maxPotUnits]potUnit
	tap   [MaxPerm]potUnit
	// pain is the activation pain of the ability the walk is recording
	// (abPainOf, saturated; 0 for the floats and a production the walk does
	// not price): every unit rec records carries it.
	pain uint8
	// tapPain is the foreign damage this source's tap deals its payer
	// (tapsCensusPain: a Pain-class Taps watcher, or a deferred TapsForMana
	// watcher's disclosed literal damage such as Manabarbs'): the units carry
	// it too, so potUnitsAvail's lethal guard caps what one plan may tap.
	tapPain uint8
	// tapX is the tap-another candidates this walk's census has already
	// spent. Two Birchlore Rangers ("Tap two untapped Elves you control:
	// Add one mana of any color") see the same two Elves, but one
	// activation taps both, so the second source makes no unit (CR 601.2g;
	// gorge's payment search elects and claims pay.ManaTapCandidates once).
	tapX slotMask
}

// unitPain is the pain one recorded unit carries: the activation's own pain
// plus the foreign tap damage, saturated.
func (c *potCensus) unitPain() uint8 {
	n := int(c.pain) + int(c.tapPain)
	if n > 255 {
		return 255
	}
	return uint8(n)
}

func (c *potCensus) rec(mask, n uint8) {
	if c.on {
		recordPotUnit(&c.units, &c.n, mask, n, c.unitPain(), false)
	}
}

// recDrop is rec for a production whose paid activation the planner's
// lethal guard refuses: drop's colours price at nothing (an activation
// whose own life or damage reaches the payer's life is never a plan, so
// the production is no unit of the planner-shaped per-tap bound), the
// fold keeps it (gorge's potentialManaFor admits the ability, as
// potentialAdmits' life >= cost read and painAvailMask's window read do).
func (c *potCensus) recDrop(sl Slot, mask, n, drop uint8) {
	if c.on {
		if m := mask &^ drop; m != 0 {
			c.rec(m, n)
		}
	}
}

// gap marks the record incomplete (the overflow marker): a paid mana ability the
// census cannot price stands outside it, so potUnitsAvail reports no exact view.
func (c *potCensus) gap() {
	if c.on {
		c.units = [maxPotUnits]potUnit{{mask: 1}}
	}
}

// recPickDrop records sl's tap as one same-pick unit of n mana all in one
// colour of mask (a reflected extra over an open production, born grown:
// potUnitsAvail prices it as a one-colour source) and notes it for mergeTap.
// A lethal-guard refusal's drop colours price at nothing, as recTapDrop's.
func (c *potCensus) recPickDrop(sl Slot, mask, n, drop uint8) {
	if !c.on || n == 0 {
		return
	}
	if mask &^= drop; mask == 0 {
		return
	}
	recordPotUnit(&c.units, &c.n, mask, n, c.unitPain(), true)
	c.tap[sl] = potUnit{mask: mask, n: n, pain: c.unitPain(), pick: true}
}

// tapRec notes sl's tap production (already recorded) for mergeTap.
func (c *potCensus) tapRec(sl Slot, mask, n uint8) {
	if c.on && mask != 0 && n > 0 {
		c.tap[sl] = potUnit{mask: mask, n: n, pain: c.unitPain()}
	}
}

// recTap records and notes a source's tap production.
func (c *potCensus) recTap(sl Slot, mask, n uint8) {
	if n > 0 {
		c.rec(mask, n)
		c.tapRec(sl, mask, n)
	}
}

// recTapDrop is recTap for a production the lethal guard refuses
// (availPotentialFor's painDrop, cenPainOf's read): drop's colours record no
// census unit, the fold keeps the production whole, and when drop swallows
// the whole production nothing is noted for mergeTap either (no unit to
// widen).
func (c *potCensus) recTapDrop(sl Slot, mask, n, drop uint8) {
	if drop != 0 {
		if m := mask &^ drop; m != 0 {
			c.recTap(sl, m, n)
		}
		return
	}
	c.recTap(sl, mask, n)
}

// mergeTap widens sl's recorded tap unit by the further {T} ability ab and
// reports it handled; false when ab is no tap ability or sl has no unit.
// The widening colours a lethal-guard refusal drops are withheld (drop).
func (c *potCensus) mergeTap(sl Slot, ab *kop.Ability, drop uint8) bool {
	return c.mergeTapN(sl, ab, max(ab.ManaN, 1), drop)
}

// mergeTapN is mergeTap for a further {T} ability making n mana in one tap (a
// storage battery's counters, one mana each): the source's unit grows to n
// when n exceeds its own tap's production, never to the sum of the two.
func (c *potCensus) mergeTapN(sl Slot, ab *kop.Ability, n, drop uint8) bool {
	t := c.tap[sl]
	if !c.on || ab.Cost.Flags&kop.CostTap == 0 || t.mask == 0 {
		return false
	}
	unrecordPotUnit(&c.units, &c.n, t.mask, t.n, t.pain, t.pick)
	// the widened unit keeps the cheaper pain: the over-bound the census is
	// (a tap makes one mana through whichever of its abilities is cheapest)
	t = potUnit{mask: t.mask | uint8(ab.ManaColors)&^drop, n: max(t.n, n), pain: min(t.pain, c.unitPain()), pick: t.pick}
	recordPotUnit(&c.units, &c.n, t.mask, t.n, t.pain, t.pick)
	c.tap[sl] = t
	return true
}

// growTap widens sl's recorded tap unit by r more mana of the same pick (a
// reflected TapsForMana extra over a several-type production: one tap makes
// one mana of the pick plus r more of it). The unit keeps its union mask but
// is marked pick: one tap makes t.n+r mana all in one colour of it, so
// potUnitsAvail prices the record as a one-colour source (a plain widened
// record would be t.n+r independent units, an {R}{G} mix one tap cannot
// make). The record is marked incomplete when sl has no unit to grow.
func (c *potCensus) growTap(sl Slot, r uint8) {
	if !c.on {
		return
	}
	t := c.tap[sl]
	if t.mask == 0 || int(t.n)+int(r) > 255 {
		c.gap()
		return
	}
	unrecordPotUnit(&c.units, &c.n, t.mask, t.n, t.pain, t.pick)
	t.n += r
	t.pick = true
	recordPotUnit(&c.units, &c.n, t.mask, t.n, t.pain, true)
	c.tap[sl] = t
}

// recStore records a storage battery's further tap ability in the census,
// merging into the source's plain tap unit when there is one (mergeTapN) and
// recording a fresh unit of the battery's colours otherwise (a battery whose
// source has no tap unit, Dwarven Hold).
func (c *potCensus) recStore(sl Slot, ab *kop.Ability, n int, drop uint8) {
	m := uint8(min(n, 255))
	if !c.mergeTapN(sl, ab, m, drop) {
		c.recDrop(sl, uint8(ab.ManaColors), m, drop)
	}
}

// censusDynamicAb is the census's read of a further dynamic-amount ability: at
// what the source makes now (an Everflowing Chalice holding no charge counter
// makes nothing), the planner's natural amount, while the over-bound pool
// keeps the unbounded fold. It reports it handled.
func (s *State) censusDynamicAb(k *prep, sl Slot, ab *kop.Ability, c *potCensus, fold func(Slot, uint8, uint8), drop uint8) bool {
	if !c.on || ab.AFlags&kop.AfManaDynamic == 0 || ab.ManaColors == kop.ColAny || ab.AFlags&kop.AfManaEachColor != 0 || anyDifferentTapAb(ab) {
		return false
	}
	fold(sl, uint8(ab.ManaColors), potentialOpenUnits)
	c.recTapDrop(sl, uint8(ab.ManaColors), uint8(min(max(s.dynManaN(k, ab, sl), 0), 255)), drop) // a lethal-guard refusal records no unit; a negative amount (a Vivi Ornitier shrunk below 0 power) makes nothing, never a wrapped uint8 (CR 107.1b)
	return true
}

// potUnitsAvail builds the per-tap view of the potential walk's productions:
// the floating pool and every recorded unit, a production of several colours
// counted once. It is the planner-shaped bound of availPotentialFor's
// over-bound: a fixed-choice source's single tap makes one mana of one of its
// colours, so {2}{G}{B} does not read payable off {G:2, U:1, B:1} when the
// board makes 3 mana (Submerged Boneyard beside two Forests). ok is false
// when the record is incomplete (the overflow marker) and the caller keeps
// the over-bound answer.
//
// life is the payer's life: gorge's payment search cuts every plan whose
// summed pain reaches it and caps a pain-bearing class at (life-1)/pain
// activations (pay/search.go walk), so a unit's count is capped there. Several
// pain-bearing units that together exceed the budget keep their individual
// caps (an over-bound; the planner's joint choice is unmodelled).
func potUnitsAvail(a *avail, units *[maxPotUnits]potUnit, life int) bool {
	if units[0].mask != 0 && units[0].n == 0 {
		return false // the overflow marker: the record is incomplete
	}
	return potUnitsAdd(a, units, life)
}

// potUnitsAdd is potUnitsAvail's walk over a record already known complete.
func potUnitsAdd(a *avail, units *[maxPotUnits]potUnit, life int) bool {
	for i := 0; i < maxPotUnits; i++ {
		u := units[i]
		if u.mask == 0 {
			break
		}
		if u.pain > 0 && !u.pick {
			u.n = uint8(max(0, min(int(u.n), (life-1)/int(u.pain))))
		}
		if u.n > 0 {
			a.addUnits(u.mask, u.n)
			if u.pick {
				// the same-pick unit is one tap making u.n mana all in one
				// colour of mask: a one-colour entry beside its wide backing
				// (oneColourPays takes the backing out and prices the pick,
				// as gorge's planner prices the tap, planalts.go
				// addTapExtra), not u.n independent units of the union
				if int(a.nOne) == len(a.one) {
					return false // more same-pick sources than fit: no exact view
				}
				a.one[a.nOne], a.oneMask[a.nOne] = u.n, u.mask
				a.oneAltMask[a.nOne], a.oneAltN[a.nOne] = 0, 0
				a.nOne++
			}
		}
	}
	return true
}

// potentialOpenUnits stands for gorge's potentialUnbounded in each colour: more
// than any cost the kernel prices, and within an avail's uint8 count.
const potentialOpenUnits = 2 * trigXCap

// censusOpenTap records an open (any-colour or chosen-colour) source's natural
// production once, modal, in availPotentialFor's census: gorge's planner prices
// the tap at one mana of one of the source's colours (its relaxed census bounds
// each gap source the same way, paymentPlanRelaxedAlternatives' per-production
// alternatives), so the per-tap view is no unbounded fold. Who records depends
// on the mode: with boundedAny the sweep's potentialAnyColourUnit adds the tap
// (and counts it in anyFree), so the walk records nothing here and would
// otherwise double-count (three any-colour taps as six wildcard units, an
// X-bound-target cast gorge's planner refuses off three mana); without it the
// multikicker arm's census records here, because potentialAnyColourUnit reports
// open and adds nothing.
// painDrop is the printed ability's lethal-guard refusal drop the caller's
// add cenMasks the census record with (cenMaskAdd's): when it swallows the
// production there is no unit, so mergeTap is noted neither (a note over a
// record that is not there would re-record a widened unit carrying the
// refused production's colours).
// A pain-only source whose damage reaches p's life is no unit of gorge's
// planner either (potentialTapUnit's lethal guard: City of Brass at 1 life,
// CR 119.4; its damage is a trigger, so painDrop never sees it): the
// over-bound keeps it, the census does not.
func (s *State) censusOpenTap(k *prep, p uint8, sl Slot, pc *pcard, boundedAny bool, add, fold func(Slot, uint8, uint8), painDrop uint8, cen *potCensus) bool {
	mask := s.srcMaskAt(pc, sl)
	if mask == 0 {
		return true
	}
	n := s.srcYieldAt(k, sl, pc)
	if n == 0 {
		return true
	}
	if !s.censusTapXClaim(k, p, sl, int(pc.manaAb), k.tapXAbOf(s.cardAt(sl)), cen) {
		return false
	}
	if !boundedAny && pc.painOnly && int(pc.painN) >= int(s.pl[p].life) {
		fold(sl, mask, n)
		return true
	}
	if s.censusPickOwns(k, sl, pc, cen) {
		// a reflected extra over the open tap: the grown amount is one tap's
		// mana all of the pick (CR 605.3b), so the census records it as a
		// same-pick unit (recPickDrop) in either mode; the walk's fold keeps
		// the grown amount (potentialAnyColourUnit folds it in bounded mode)
		if !boundedAny {
			fold(sl, mask, n)
		}
		cen.recPickDrop(sl, mask, n, painDrop)
		return true
	}
	if !boundedAny {
		add(sl, mask, n)
	}
	if painDrop == 0 {
		cen.tapRec(sl, mask, n) // mergeTap's note, either mode
	}
	return true
}

// censusPickOwns reports whether censusOpenTap records sl's open tap in the
// census as a same-pick unit: the census is on and a reflected TapsForMana
// extra (srcReflectPick) reaches the tap, so potentialAnyColourUnit must fold
// the production into the over-bound pool alone.
func (s *State) censusPickOwns(k *prep, sl Slot, pc *pcard, cen *potCensus) bool {
	return cen.on && s.srcReflectPick(k, sl, pc) > 0
}

// potentialTapUnit records a source's tap production in availPotentialFor: a
// unit of both views. A pain-only source whose own damage or life reaches the
// payer's life is no unit of gorge's planner (the lethal guard: pay's
// PlanRemainingPain, PlanLastResortChoices' one-step guard): the potential
// walk's over-bound still lists it, the per-tap census does not (Cabal Pit at
// 1 life; CR 119.4).
//
// add is the walk's painAdd (cenMaskAdd when the printed ability's own pain
// reaches the life); painRefused withholds the mergeTap note then. The tap's
// TapsForMana extra is recorded here too, after the production, as part of
// the same tap (potentialTapExtra).
func (s *State) potentialTapUnit(k *prep, p uint8, sl Slot, pc *pcard, mask uint8, add, fold func(Slot, uint8, uint8), painRefused bool, cen *potCensus) {
	if !s.censusTapXClaim(k, p, sl, int(pc.manaAb), k.tapXAbOf(s.cardAt(sl)), cen) {
		return
	}
	if pc.painOnly && int(pc.painN) >= int(s.pl[p].life) {
		fold(sl, mask, pc.manaN)
		return
	}
	if xvMask(pc) != 0 {
		// An all-at-once source's tap makes manaN mana of EACH colour in its
		// production (Everglades' {C}{B}; gorge's planner adds each symbol),
		// so the census records one unit per colour: the union unit would
		// price the tap at ONE mana of one of its colours -- the choose-one
		// semantics a Combo dual's unit carries -- and strand an X-bound-target
		// announcement the source funds (Molder's {2}{G} off an Everglades and
		// a Forest, xBoundPayable's sweep). The over-bound fold below adds n
		// per colour either way. mergeTap's note stays empty: the note widens
		// one unit, and an all-at-once tap's units are one per colour, so a
		// further tap ability records its own unit.
		if !painRefused {
			for m := mask; m != 0; m &= m - 1 {
				cen.rec(m&-m, pc.manaN)
			}
		}
		fold(sl, mask, pc.manaN)
	} else {
		add(sl, mask, pc.manaN)
		if !painRefused {
			cen.tapRec(sl, mask, pc.manaN)
		}
	}
	cen.pain = 0 // the tap's extra is priced unpainted, as before the gate pair
	s.potentialTapExtraGated(k, sl, pc, mask, fold, cen, painRefused)
}

// potentialTapExtraGated is potentialTapExtra behind its caller-side checks:
// a TapsForMana carrier exists, the lethal guard did not refuse the tap's
// production and the source is not a pain-only face.
func (s *State) potentialTapExtraGated(k *prep, sl Slot, pc *pcard, mask uint8, fold func(Slot, uint8, uint8), cen *potCensus, painRefused bool) {
	if k.gen != nil && k.gen.anyTapExtra && !painRefused && !pc.painOnly {
		s.potentialTapExtra(k, sl, pc, mask, fold, cen)
	}
}

// markCensusAbDone marks the printed ability a source's view is as done in
// availPotentialFor's fixpoint. An intrinsic view (manaIntrinsic, blgView) is
// the granted basic land type's unit and marks no ability: the card's own
// rider is the walk's admission below (potentialManaFor counts the printed
// abilities beside the appended intrinsics, CR 305.6, 605.3b).
func markCensusAbDone(pc *pcard, sl Slot, done *[MaxPerm]uint16) {
	if pc.manaAb >= 0 && !pc.manaIntrinsic {
		done[sl] = 1 << pc.manaAb
	}
}

// availPotentialFor is availPotential; payment prices one mana payment (gorge
// potentialManaFor with a descriptor, a trigger's own cost): an ability whose
// RestrictValid$ refuses it adds nothing and a chosen colour is exact (CR 106.6).
// Every mana ability of a source counts on its own, and a paid one joins once the
// pool built from the other sources covers its cost, so the walk runs to a fixpoint.
// open reports a bound gorge prices potentialUnbounded in every colour.
//
// boundedAny is the X-bound target sweep's mode: a fixed any-colour source is
// one unit of any colour instead of potentialUnbounded in every colour. A lone
// Paradise Mantle then pays {U}, not {1}{U} (Spell Burst, CR 601.2b-c). A
// genuinely unbounded source still opens the bound, and a fixed multi-colour
// source keeps the per-colour over-count (Dominate's refusal).
//
// units, when given, receives the walk's productions as potUnit records (the
// floating pool first, each production once with its colours' mask) so the
// caller can price them per tap (potUnitsAvail); the over-bound avail keeps
// its once-per-colour fold either way.
//
//go:noinline
func (s *State) availPotentialFor(k *prep, p uint8, a *avail, d *payDesc, boundedAny bool, units ...*[maxPotUnits]potUnit) (open bool) {
	payment := d != nil
	cen := potCensus{on: len(units) != 0}
	var pot [kop.NumManaKinds]int
	// anyFree counts a fixed any-colour source's units in bounded mode: they
	// are added to every potential colour for the fixpoint's coverage test,
	// then priced once as wildcard units in the final avail.
	anyFree := 0
	for c := range pot {
		pot[c] = int(s.pl[p].pool[c])
		cen.rec(1<<uint(c), uint8(min(pot[c], 255)))
	}
	// A non-spell payment's potential pool starts from the floating restricted
	// mana its restriction admits (potentialRestrSeed; a nil d is no payment).
	s.potentialRestrSeed(k, p, d, &pot)
	var own [MaxPerm][kop.NumManaKinds]int
	var done [MaxPerm]uint16
	fold := func(sl Slot, mask uint8, n uint8) {
		for mk := mask; mk != 0; mk &= mk - 1 {
			c := bits.TrailingZeros8(mk)
			pot[c] += int(n)
			own[sl][c] += int(n)
		}
	}
	add := func(sl Slot, mask uint8, n uint8) {
		cen.rec(mask, n)
		fold(sl, mask, n)
	}
	// life is the payer's life: a mana ability whose activation pain reaches
	// it is no unit of the census (potCensus.painGate), while the fold keeps
	// it (gorge's potentialManaFor admits the ability, as potentialAdmits'
	// life >= cost read and painAvailMask's window read do).
	life := int(s.pl[SeatIx(p)].life)
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		cen.pain = 0
		cen.tapPain = 0
		if cen.on {
			cen.tapPain = s.tapsCensusPain(k, p, sl)
		}
		pc := &k.cards[s.cardAt(sl)]
		if k.gen != nil {
			pc = s.manaPCGen(k, sl)
		}
		if !s.canTapForManaPC(k, sl, pc) || pc.manaAb < 0 && s.perms[sl].ovr.flags&ovCopy == 0 || payment && k.gen != nil && s.openManaRefused(k, p, sl, d) {
			if k.gen != nil && !s.canTapForManaPC(k, sl, pc) {
				s.potentialGrantSac(k, sl, payment, add, &open) // a granted sacrifice-only ability, tapped or not (mana_grantcost.go)
				if s.riderPotential(k, p, sl) {
					// a bounded rider source's activation is one of gorge's
					// potential walk's units: its fixed production counts
					// once, an any-colour or all-at-once one is unbounded
					// (gorge potentialUnbounded; the Deplete land's is fixed)
					if a, ok := s.riderAbilityOf(k, sl); ok {
						_, radd := cen.painGate(abPainOf(a), life, uint8(a.ManaColors), add, fold) // a rider's lethal-guard refusal records no unit
						open = s.potentialRiderUnit(a, sl, boundedAny, radd, &anyFree) || open
					}
					// counted here, so the fixpoint below must not admit the same
					// ability again (potentialGapCost's Deplete arm): gorge counts it once
					s.markRiderDone(k, sl, &done)
				}
			}
			if k.gen != nil && pc.manaAb < 0 {
				if s.reflAmtPotential(k, sl, pc, &done, fold) {
					continue // a reflected Amount$ source's {C}-only fold (mana_reflamt.go)
				}
				s.potentialReflNone(k, sl, &done) // a reflected view with no colour makes no mana: the walk below must not admit the printed ability open
			}
			continue
		}
		markCensusAbDone(pc, sl, &done)
		// the printed ability's own activation pain: when it reaches the
		// life its production is no census unit, the fold keeps it (a
		// pain-only source makes nothing there, a paid face beside a normal
		// one keeps the normal face's colours alone)
		pain := s.printedAbPain(k, sl, pc)
		cen.pain = uint8(min(pain, 255)) // the units below carry it (potUnitsAvail's cap)
		painDrop, painAdd := cen.painGate(pain, life, s.srcMaskAt(pc, sl), add, fold)
		painRefused := painDrop != 0
		if payment && pc.manaChosen && s.perms[sl].chosen > 4 {
			// a Produced$ Chosen source whose colour is not chosen yet: gorge's
			// exact handler (potential.go addPotentialChosenColour) declines
			// it, so its fold prices the word open in every colour
			// (producedOpen), the payment bound included
			open = true
			continue
		}
		dynamic := s.potentialDynamicSource(k, sl, pc, &done)
		if dynamic {
			// gorge's potentialAmount prices an indeterminate Amount$
			// unbounded in exactly the produced faces it names (CR 107.4a;
			// rules potential.go addIndeterminatePotentialMana), never in a
			// colour the ability does not make. An open production (any
			// colour, or a chosen colour at a payment the bound only bounds)
			// stays gorge's whole-pool open; a fixed one is unbounded in its
			// own faces alone.
			if potentialOpenProduction(pc, payment) {
				open = true
				// the planner's census leaves a dynamic any-colour source
				// out (a gap source, no proof against a play it could
				// fund), so the per-tap view counts it at the amount it
				// makes now (Lotus Blossom's petals, CR 601.2g); recTap is
				// a no-op when the census is off. A lethal-guard refusal
				// drops the production's colours out of that unit too
				// (recTapDrop).
				cen.recTapDrop(sl, s.srcMaskAt(pc, sl), srcYield(pc), painDrop)
			} else if cen.on {
				// the census reads the source at the amount it makes now, the
				// planner's natural one (an Everflowing Chalice holding no
				// charge counter makes nothing); the over-bound pool keeps
				// the unbounded fold
				mask := s.srcMaskAt(pc, sl)
				fold(sl, mask, potentialOpenUnits)
				cen.recTapDrop(sl, mask, srcYield(pc), painDrop)
			} else {
				painAdd(sl, s.srcMaskAt(pc, sl), potentialOpenUnits)
			}
			// the tap's TapsForMana extra is a fixed vector beside the
			// dynamic amount (Tower Worker's {C} under Leyline of Abundance's
			// {G}): gorge folds it into each alternative of the tap
			s.potentialTapExtraGated(k, sl, pc, s.srcMaskAt(pc, sl), fold, &cen, painRefused)
			continue
		}
		if !pc.manaMulti && potentialOpenProduction(pc, payment) {
			open = s.potentialOpenTap(k, p, sl, pc, boundedAny, painAdd, fold, painDrop, &anyFree, &cen) || open
			continue
		}
		mask := s.srcMaskAt(pc, sl)
		if k.manaEach {
			mask |= xvMask(pc) // an all-at-once source adds each of its colours (gorge addPotentialMana: every rune)
		}
		if pc.manaMulti && k.gen != nil && done[sl] != ^uint16(0) && pc.manaAb >= 0 {
			mask = s.potentialFirstMask(k, sl, pc) // the walk below admits its other abilities, each once
		}
		// painAdd is cenMaskAdd when the lethal guard refuses the printed ability's production
		s.potentialTapUnit(k, p, sl, pc, mask, painAdd, fold, painRefused, &cen)
		cen.pain = 0 // the gate pair's and the granted units are priced unpainted
		if n := gatePairPotential(pc); n != 0 {
			add(sl, uint8(pc.wideCol), n) // gorge's walk sums both listed abilities of a gate-on pair
		}
		s.potentialGrantedExtra(k, sl, d, boundedAny, add, &open, &anyFree)
	}
	for progressed := k.gen != nil; progressed; {
		progressed = false
		for m := s.ctrlM[p]; m.any(); m = m.rest() {
			sl := m.first()
			cen.tapPain = 0
			if cen.on {
				cen.tapPain = s.tapsCensusPain(k, p, sl)
			}
			cd := &k.t.Cards[s.cardAt(sl)]
			others := pot
			for c := range others {
				others[c] -= own[sl][c]
			}
			for j := range cd.Abilities {
				ab := &cd.Abilities[j]
				cen.pain = 0
				if j >= 16 || ab.Kind != kop.AbMana || ab.AFlags&kop.AfLatent != 0 || done[sl]&(1<<j) != 0 || !s.potentialAdmits(k, p, sl, j, ab, d) {
					continue
				}
				if cen.on && s.dynManaYieldsNothing(k, ab, sl) {
					// gorge's planner window never taps the ability
					// (manaAbilityYieldsNothing, gen.go dynManaYieldsNothing):
					// the census prices its tap at nothing -- no gap, no
					// unit, no fold (Baldur's Gate's second ability with no
					// other Gates, its {2} fee gapping the census and leaving
					// an X-bound sweep the over-bound open pool; CR 601.2g)
					done[sl] |= 1 << j
					continue
				}
				paidGap, skip, repeat := d.paidGapProbe(len(units) != 0, boundedAny, sl, j, ab)
				if skip {
					continue
				}
				if !open && !potentialCovers(&others, &ab.Cost.Mana) {
					continue
				}
				if !s.censusTapXClaim(k, p, sl, j, ab, &cen) {
					// the tap-two candidates are already spent by an earlier
					// activation of this walk: no unit, no gap (censusTapXClaim)
					done[sl] |= 1 << j
					continue
				}
				if paidGap {
					s.bookPaidGap(k, d, sl, j, ab, &cen, repeat) // mana_xboundgap.go
				}
				done[sl] |= 1 << j
				progressed = true
				// the ability's own activation pain: when it reaches the
				// life its production is no census unit, the fold keeps it
				// (a pain-only source makes nothing there, a paid face
				// beside a normal one keeps the normal face's colours alone)
				abPain := abPainOf(ab)
				cen.pain = uint8(min(abPain, 255)) // the units below carry it (potUnitsAvail's cap)
				abDrop, addAb := cen.painGate(abPain, life, uint8(ab.ManaColors), add, fold)
				if s.storeBatteryPotential(sl, ab, d, &cen, abDrop, addAb, fold, &open) {
					continue
				}
				if ab.ManaIdentityPotential {
					open = s.identityPotentialAdd(k, sl, ab, d, open, addAb)
					continue
				}
				if s.censusDynamicAb(k, sl, ab, &cen, fold, abDrop) {
					continue
				}
				// anyAdd is add, with the census's one-mana any-colour
				// production of a source already holding a tap unit merged
				// into that unit (anyAddAb): the source taps once, the
				// production widens the unit's colours instead of adding a
				// second mana (CR 605.3b; gorge's census makes one mana per
				// tap). The over-bound pool keeps the fold.
				anyAdd := func(sl Slot, mask, n uint8) {
					cen.anyAddAb(sl, ab, mask, n, abDrop, addAb, fold)
				}
				if !s.addAnyDifferentOrOpen(k, p, sl, ab, boundedAny, len(units) != 0, anyAdd, &anyFree, &open) {
					s.addOrMergeRepeat(&cen, sl, ab, d, abDrop, addAb, fold)
				}
				open = s.sacAnyRepeatUnits(k, p, sl, ab, abDrop, &cen, fold) || open
			}
		}
	}
	s.potentialBuildAvail(a, &pot, open, boundedAny, anyFree)
	if len(units) != 0 {
		*units[0] = cen.units
	}
	return open
}

// sacAnyRepeatUnits is availPotentialFor's census arm for a live sacrifice-any
// mana source (Phyrexian Altar's "Sacrifice a creature: Add one mana of any
// color", no {T}): it activates once per candidate in the same payment. gorge's
// potential fold prices the open any-colour production unbounded in every
// colour (potential.go producedOpen), so the bound admits the play and its
// planner's prefix search funds it; the per-tap census keeps the repeats the
// sacrifice can fund as cands units, one per candidate
// (pay.ManaSacrificeCandidates) — the planner-shaped bound of the play (CR
// 605.3b, 601.2g). With one candidate the one-unit record stands exact: the
// sweep still refuses a play that one activation cannot fund, as gorge's
// planner does.
//
// The unrecord takes back the pain the ordinary unit was recorded with
// (cen.unitPain: abPain plus the tap's foreign damage the census carries for
// the source); the replacement units carry cen.pain alone — the sacrifice
// activations never tap the source, so the Taps watchers' damage prices no
// repeat. It reports the production's open fold.
func (s *State) sacAnyRepeatUnits(k *prep, p uint8, sl Slot, ab *kop.Ability, abDrop uint8, cen *potCensus, fold func(Slot, uint8, uint8)) (open bool) {
	if !cen.on || !kop.SacAnyManaAb(ab) {
		return false
	}
	var cs [MaxPerm]Slot
	if n := s.sacXCands(k, p, sl, ab, &cs); n >= 2 {
		// Only the any-colour production is open in gorge's fold. A
		// fixed-colour one (Krark-Clan Ironworks' {C}{C}) counts once in
		// PotentialMana, so its repeats are neither open nor folded: the
		// bound stays gorge's and a cast only the repeats fund is no play
		// (pair 67: Grab the Reins' entwined cast beside two artifacts).
		anyCol := ab.ManaColors == kop.ColAny
		open = anyCol
		if m := uint8(ab.ManaColors) &^ abDrop; m != 0 {
			unrecordPotUnit(&cen.units, &cen.n, m, 1, cen.unitPain(), false)
			recordPotUnit(&cen.units, &cen.n, m, uint8(min(n, 255)), cen.pain, false)
			if anyCol {
				fold(sl, m, uint8(min(n, 255)))
			}
		}
	}
	return open
}

// potentialTapExtra folds a TapsForMana carrier's extra (Heartbeat of Spring:
// one more mana of the type the land made) into the over-bound pool: it is
// part of the tap's production in gorge's potential pool, so one Forest funds
// a repeatable filter's {2} fee (CR 605.3a, 106.1). The per-tap census records
// it too (cen): gorge's planner prices the tap at its mana plus the extra
// (pay addTapExtra), so a Mountain under Keeper of Progenitus is one unit of
// {R}{R} (CR 605.3b, 106.1) and an X-bound cast's sweep reads it. A fixed
// several-colour tap's reflected extra grows the tap unit as a same-pick
// record (growTap): one tap makes the grown mana of the pick alone. The
// caller has checked k.gen.anyTapExtra, the lethal guard's refusal and the
// pain-only face.
func (s *State) potentialTapExtra(k *prep, sl Slot, pc *pcard, mask uint8, fold func(Slot, uint8, uint8), cen *potCensus) {
	var xv [kop.NumManaKinds]uint8
	if pc.painCol != 0 {
		// a painful source's unit here is its normal ability's: the extra of
		// that tap (srcExtraPain's xc); the painful pick's is priced where
		// that alternative is (a stall counted when it cannot be)
		var ok bool
		if xv, _, _, ok = s.srcExtraPain(k, sl, pc); !ok {
			return
		}
	} else {
		xv = s.srcExtra(k, sl, pc)
	}
	for c, v := range xv {
		// srcExtra holds the source's own all-at-once rest (pcard.manaXv),
		// which the tap's colour mask already priced; only the carrier's part
		// is folded here (a stall returns nothing, so the subtraction floors)
		if own := pc.manaXv[c]; v > own {
			fold(sl, 1<<c, v-own)
			cen.rec(1<<c, v-own)
		}
	}
	if r := s.srcReflectPick(k, sl, pc); r > 0 {
		fold(sl, mask, r)
		cen.growTap(sl, r)
	}
}

// potentialGrantedExtra admits the mana abilities granted to sl beside its own
// one (an aura's "{T}: Add two mana of any one color"): each counts on its own,
// as gorge's walk admits each of a source's abilities. A source with no usable
// grant leaves open and anyFree alone.
func (s *State) potentialGrantedExtra(k *prep, sl Slot, d *payDesc, boundedAny bool, add func(Slot, uint8, uint8), open *bool, anyFree *int) {
	if k.gen == nil || len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || !s.ownManaUsable(k, sl) {
		return
	}
	var gi [maxGrants]int16
	for i, ng := 0, s.potentialGrants(k, sl, &gi, d); i < ng; i++ {
		gv := s.grantView(k.gen, gi[i], sl)
		if gv.manaCol == kop.ColAny && !gv.manaChosen && !gv.manaMulti {
			*open = s.potentialGrantedUnit(gv, sl, boundedAny, add, anyFree) || *open
			continue
		}
		add(sl, s.srcMaskAt(gv, sl), gv.manaN)
	}
}

// potentialGrants lists the mana abilities granted to sl that a payment d
// may spend: a restricted grant d refuses makes no potential mana (CR 106.6;
// gorge potentialRestrictionAdmits). A nil d is no payment, nothing refused.
//
//go:noinline
func (s *State) potentialGrants(k *prep, sl Slot, gi *[maxGrants]int16, d *payDesc) int {
	ng := s.grantIdxs(k, sl, gi)
	if d == nil {
		return ng
	}
	return s.grantsAdmitted(k, sl, gi, ng, d)
}

// anyTapP reports whether ab is a further fixed any-colour tap production the
// census may merge into its source's already-recorded tap unit (the anyAdd
// add availPotentialFor passes on): a source taps once, so its second {T}
// ability (Mirrodin's Core's "{T}, Remove a charge counter: Add one mana of
// any color" beside its {T}: {C}) widens that unit's colours instead of
// adding a second mana (CR 605.3b; gorge's census makes one mana per tap).
func (c *potCensus) anyTapP(ab *kop.Ability) bool {
	return c.on && ab.ManaColors == kop.ColAny &&
		ab.AFlags&(kop.AfManaDynamic|kop.AfManaEachColor|kop.AfManaChoice) == 0 && !anyDifferentTapAb(ab)
}

// anyAddAb is the add the anyAdd closure in availPotentialFor passes on: a
// further {T} ability of a source already holding a tap unit (Mirrodin's
// Core's "{T}, Remove a charge counter: Add one mana of any color" beside its
// {T}: {C}) widens that unit's colours instead of adding a second mana, the
// source tapping once (CR 605.3b; gorge's census makes one mana per tap). The
// over-bound pool keeps the fold. drop is the widening colours a lethal-guard
// refusal prices at nothing (cenPainDrop's).
func (c *potCensus) anyAddAb(sl Slot, ab *kop.Ability, mask, n, drop uint8, add, fold func(Slot, uint8, uint8)) {
	if c.anyTapP(ab) && c.mergeTap(sl, ab, drop) {
		fold(sl, mask, n)
		return
	}
	add(sl, mask, n)
}

// addOrMerge adds ab's fixed production as a unit of its own, or, when the census
// merges a repeat tap source into one already counted (mergeTap), folds it into
// the pool alone, so it counts once as gorge's census does. drop is the colours
// a lethal-guard refusal prices at nothing (cenPainDrop's).
func (c *potCensus) addOrMerge(sl Slot, ab *kop.Ability, drop uint8, add, fold func(Slot, uint8, uint8)) {
	if ab.ManaEach {
		// An all-at-once production makes manaN mana of EACH colour in its
		// mask (addFiltUnits' split), so the census records one unit per
		// colour. One union unit would price the tap at ONE mana of one of
		// its colours, the choose-one semantics a Combo dual carries, and
		// strand an X-bound announcement the source funds (Golgari Signet's
		// {1},{T}: Add {B}{G} under Repeal's cmcEQX target). The tap note
		// stays empty: one unit cannot widen to a per-colour vector, so a
		// further tap ability records its own unit (potentialTapUnit's
		// all-at-once branch).
		if c.on {
			if t := c.tap[sl]; t.mask != 0 {
				unrecordPotUnit(&c.units, &c.n, t.mask, t.n, t.pain, t.pick)
				c.tap[sl] = potUnit{}
			}
			for mk, n := range filtEachVec(ab) {
				if n != 0 {
					c.recDrop(sl, uint8(1<<uint(mk)), n, drop)
				}
			}
		}
		fold(sl, uint8(ab.ManaColors), max(ab.ManaN, 1))
		return
	}
	if c.mergeTap(sl, ab, drop) {
		fold(sl, uint8(ab.ManaColors), max(ab.ManaN, 1))
		return
	}
	if c.on && ab.ManaEach {
		// an all-at-once production (Simic Signet's {G}{U}) makes every
		// colour it names in one activation (CR 106.3): one unit a colour,
		// not one unit of either (gorge ProducedCounts sums each colour)
		for col, n := range filtEachVec(ab) {
			if n != 0 {
				c.recDrop(sl, 1<<col, n, drop)
			}
		}
	} else {
		c.recDrop(sl, uint8(ab.ManaColors), max(ab.ManaN, 1), drop)
	}
	fold(sl, uint8(ab.ManaColors), max(ab.ManaN, 1))
}

// identityPotentialAdd prices a Produced$ ColorIdentity source a rider or a cost
// beyond {T} kept out of the planner's census (Path of Ancestry, Opal Palace): a
// payment's bound prices it at the controller's commander identity (gorge
// potential.go addPotentialIdentityColours), while the plain over-bound prices the
// ColorIdentity word unbounded in every colour (CR 903.4). No commander reads
// nothing, which adds no unit either way (gorge potentialIdentityEmpty). It
// returns the walk's open flag.
func (s *State) identityPotentialAdd(k *prep, sl Slot, ab *kop.Ability, d *payDesc, open bool, add func(Slot, uint8, uint8)) bool {
	if m := s.identityColours(k, s.perms[sl].ctrl); m != 0 {
		if d == nil {
			return true
		}
		add(sl, uint8(m), max(ab.ManaN, 1))
	}
	return open
}

// identityPotentialAbility is the source at sl's Produced$ ColorIdentity
// ability a rider or a cost beyond {T} kept out of the planner's census
// (kop.Ability.ManaIdentityPotential), and its index on the card.
func (s *State) identityPotentialAbility(k *prep, sl Slot) (*kop.Ability, int, bool) {
	if k.gen == nil {
		return nil, 0, false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if j < 16 && a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.ManaIdentityPotential {
			return a, j, true
		}
	}
	return nil, 0, false
}

// identityPotentialView is the payment window's view of the source at sl when
// its identity-potential ability is a plain {T} mana ability: one mana of each
// colour in its controller's commander identity (CR 903.4). Every other cost
// shape stays out of the manual window; the potential gate still offers its
// bound, and a payment it cannot fund declines (CR 118.3), as any over-bound
// offer does.
func (s *State) identityPotentialView(k *prep, sl Slot) (*pcard, bool) {
	a, j, ok := s.identityPotentialAbility(k, sl)
	if !ok || a.Cost.Flags != kop.CostTap || a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 {
		return nil, false
	}
	m := s.identityColours(k, s.perms[sl].ctrl)
	if m == 0 {
		return nil, false
	}
	return &pcard{manaAb: int8(j), manaCol: m, manaN: max(a.ManaN, 1), manaTap: true}, true
}

// identityPotentialSource reports whether the source at sl has a Produced$
// ColorIdentity ability a rider or a cost beyond {T} kept out of the planner's
// census (kop.Ability.ManaIdentityPotential) that the potential walk can use
// for payment d now: its cost payable and a commander identity to read (no
// commander reads nothing, CR 903.4f). Gorge's payment-bound fold prices such a
// source at the identity colours (rules potential.go
// addPotentialIdentityColours), so a trigger's own cost must see it as a source
// even though the planner lists no unit for it.
func (s *State) identityPotentialSource(k *prep, p uint8, sl Slot, d *payDesc) bool {
	if s.identityColours(k, s.perms[sl].ctrl) == 0 {
		return false
	}
	a, j, ok := s.identityPotentialAbility(k, sl)
	return ok && s.potentialAdmits(k, p, sl, j, a, d)
}

// potentialDynamicSource reports whether sl's view is an unbounded dynamic
// amount, marking the abilities the walk must not admit again.
func (s *State) potentialDynamicSource(k *prep, sl Slot, pc *pcard, done *[MaxPerm]uint16) bool {
	if k.gen == nil {
		return false
	}
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfManaVar == 0 || s.filtFeeOwed(k, sl, c) {
		return false
	}
	// a gated or reflected source's view already holds its abilities
	done[sl] = ^uint16(0)
	if k.gen.anyFiltPip {
		done[sl] &^= s.filtPipBit(k, sl) // a one-pip filter joins below once the pool pays it (mana_filterpip.go)
	}
	cd := &k.t.Cards[c]
	return pc.manaAb >= 0 && int(pc.manaAb) < len(cd.Abilities) && cd.Abilities[pc.manaAb].AFlags&kop.AfManaDynamic != 0 ||
		k.gen.flags[c]&gfManaDynPair != 0 // gorge's potentialAmount reads the second ability's Count$ as indeterminate: unbounded (mana_dynpair.go)
}

// potentialAnyColourUnit adds a fixed any-colour source in bounded mode as one
// unit of any colour; it reports the unbounded case.
func (s *State) potentialAnyColourUnit(k *prep, sl Slot, pc *pcard, boundedAny bool, add func(Slot, uint8, uint8), anyFree *int) bool {
	if !boundedAny {
		return true
	}
	mask, n := s.srcMaskAt(pc, sl), s.srcYieldAt(k, sl, pc)
	if mask == uint8(kop.ColAny) {
		*anyFree += int(n)
	}
	add(sl, mask, n)
	return false
}

// potentialOpenTap records an open source's tap production in the potential
// walk: the census unit(s) when the per-tap view is requested, then the
// bounded any-colour unit. A tap-two source whose candidates an earlier
// activation spent contributes nothing (censusTapXClaim). It returns the
// walk's open flag.
func (s *State) potentialOpenTap(k *prep, p uint8, sl Slot, pc *pcard, boundedAny bool, add, fold func(Slot, uint8, uint8), painDrop uint8, anyFree *int, cen *potCensus) bool {
	if cen.on && !s.censusOpenTap(k, p, sl, pc, boundedAny, add, fold, painDrop, cen) {
		return false
	}
	anyAdd := add
	if s.censusPickOwns(k, sl, pc, cen) {
		anyAdd = fold
	}
	return s.potentialAnyColourUnit(k, sl, pc, boundedAny, anyAdd, anyFree)
}

// potentialOpenProduction reports whether a mana view's production is open:
// any colour, or a chosen colour only where the bound bounds (no payment, the
// choice not fixed), the shape availPotentialFor prices whole-pool open.
func potentialOpenProduction(pc *pcard, payment bool) bool {
	return pc.manaCol == kop.ColAny && !pc.manaChosen || pc.manaChosen && !payment && !pc.manaFixedChosen
}

// potentialRiderUnit is a rider source's unit in the potential walk.
func (s *State) potentialRiderUnit(a *kop.Ability, sl Slot, boundedAny bool, add func(Slot, uint8, uint8), anyFree *int) bool {
	if !(a.ManaEach || uint8(a.ManaColors) == uint8(kop.ColAny) || a.AFlags&kop.AfManaChoice != 0) {
		add(sl, uint8(a.ManaColors), max(a.ManaN, 1))
		return false
	}
	if boundedAny && !a.ManaEach && a.AFlags&kop.AfManaChoice == 0 && uint8(a.ManaColors) == uint8(kop.ColAny) {
		n := int(max(a.ManaN, 1))
		*anyFree += n
		add(sl, uint8(kop.ColAny), uint8(n))
		return false
	}
	return true
}

// potentialGrantedUnit is a granted any-colour unit in the potential walk.
func (s *State) potentialGrantedUnit(gv *pcard, sl Slot, boundedAny bool, add func(Slot, uint8, uint8), anyFree *int) bool {
	if !boundedAny {
		return true
	}
	*anyFree += int(gv.manaN)
	add(sl, uint8(kop.ColAny), gv.manaN)
	return false
}

// potentialExtraUnit is a source's further mana ability in the potential walk.
func (s *State) potentialExtraUnit(ab *kop.Ability, sl Slot, boundedAny bool, add func(Slot, uint8, uint8), anyFree *int) bool {
	if !boundedAny || ab.AFlags&(kop.AfManaDynamic|kop.AfManaEachColor) != 0 || ab.ManaColors != kop.ColAny {
		return true
	}
	n := max(ab.ManaN, 1)
	*anyFree += int(n)
	add(sl, uint8(kop.ColAny), n)
	return false
}

// potentialBuildAvail turns the per-colour potential into a's units. In bounded
// mode the wildcard units are removed from every colour and added once.
func (s *State) potentialBuildAvail(a *avail, pot *[kop.NumManaKinds]int, open, boundedAny bool, anyFree int) {
	a.total, a.n = 0, 0
	for c, n := range pot {
		if open {
			n = potentialOpenUnits
		} else if boundedAny && anyFree > 0 && c < 5 && uint8(kop.ColAny)&(1<<c) != 0 {
			n -= anyFree
		}
		if n > 0 {
			a.addUnits(1<<c, uint8(min(n, 255)))
		}
	}
	if boundedAny && !open && anyFree > 0 {
		a.addUnits(uint8(kop.ColAny), uint8(min(anyFree, 255)))
	}
}

// addAnyDifferentOrOpen is the potential walk's admission rule for a further
// ability: Combo AnyDifferent adds one unit of each colour (the bounded
// per-colour cap), an any-colour or each-colour production is open, and a
// dynamic one is unbounded in the faces it produces (CR 107.4a); every other
// ability is the caller's ordinary add. It reports whether it handled the
// ability. On the census (units) a fixed any-colour production joins modal
// once, the same bound the walk prices an open source's own tap at, while an
// each-colour one is unbounded to the census and joins nothing (a dynamic
// one's faces join unbounded) -- except on the X-bound sweep's per-tap
// census (boundedAny, kgen's EachColorAmong production): gorge's planner
// census misses it and prices the play only through paymentPlanRelaxProof's
// relaxation of the missed ability, one activation making one mana of each
// colour among its permanents (Bloom Tender's shape, doc.go), so the census
// prices the production there too, one unit per colour. An omitted one made
// the record's refusal a proof where the relaxation pays: Thrive's {G} off
// Faeburrow Elder ("{T}: add one mana of each colour among permanents you
// control") was refused and the cast dropped from the priority options
// (tail-07 pair 144, decision 335).
func (s *State) addAnyDifferentOrOpen(k *prep, p uint8, sl Slot, ab *kop.Ability, boundedAny, census bool, add func(Slot, uint8, uint8), anyFree *int, open *bool) bool {
	if anyDifferentTapAb(ab) {
		add(sl, 0x1f, 1)
		return true
	}
	if ab.ManaColors == kop.ColAny || ab.AFlags&kop.AfManaEachColor != 0 {
		if census && !boundedAny && ab.ManaColors == kop.ColAny &&
			ab.AFlags&(kop.AfManaDynamic|kop.AfManaEachColor|kop.AfManaChoice) == 0 {
			add(sl, uint8(ab.ManaColors), max(ab.ManaN, 1))
		}
		if census && boundedAny && ab.AFlags&kop.AfManaEachColor != 0 {
			// the relaxation's units (eachColorOut's colours, CR 118.3c's one
			// mana per tap per colour; addOrMerge's all-at-once split): one
			// per-colour unit, never a union one, so an announcement the one
			// tap funds is priced and one it cannot fund is not
			if n := max(ab.ManaN, 1); n != 0 {
				for m := uint8(s.eachColorOut(k, p, sl)); m != 0; m &= m - 1 {
					add(sl, m&-m, n)
				}
			}
		}
		*open = s.potentialExtraUnit(ab, sl, boundedAny, add, anyFree) || *open
		return true
	}
	if ab.AFlags&kop.AfManaDynamic != 0 {
		// an indeterminate Amount$ is unbounded in the produced faces it
		// names (CR 107.4a; rules potential.go addIndeterminatePotentialMana),
		// never in a colour the ability does not make
		add(sl, uint8(ab.ManaColors), potentialOpenUnits)
		return true
	}
	return false
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
	if ab.Cost.Flags&^(kop.CostTap|kop.CostSacSelf|kop.CostPayLife|kop.CostOncePerTurn|kop.CostOncePerGame) != 0 && !s.potentialGapCost(k, p, sl, j, ab) || pp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.genLost(k, sl) {
		return false
	}
	if ab.Cost.Flags&kop.CostOncePerTurn != 0 && j < 8 && pp.actUsed&(1<<j) != 0 {
		return false // Gravestone Strider, used this turn
	}
	if sacCtrManaAb(ab) && pp.ctrN(s, int(ab.ManaV.N)) <= 0 {
		return false // a counter-scaled sacrifice source with no counter adds nothing (mana_sacctr.go; gorge manaAbilityYieldsNothing)
	}
	if ab.Cost.Flags&kop.CostOncePerGame != 0 && s.onceBlocked(k, sl, ab) {
		return false // Loot, the Pathfinder's exhaust mana ability
	}
	if ab.Cost.Flags&kop.CostPayLife != 0 && int(s.pl[SeatIx(p)].life) < lifeCostOwed(&ab.Cost) {
		return false
	}
	if k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	if ab.Cost.Flags&kop.CostTap != 0 && (pp.flags&pfTapped != 0 || s.activationSick(k, sl)) {
		return false
	}
	if ab.Trig.Cond != 0 && !s.condEval(k, ab.Trig.Cond, evalCtx{src: pp.obj, srcSl: sl, ctrl: pp.ctrl}) {
		return false
	}
	if col, ok := fixedPipFilt(ab); ok && k.gen.anyFiltPip && s.filtPipHeld(k, pp.ctrl, sl, col) < 1 {
		return false // gorge lists the granted filter only while the live pool pays its pip (mana_filterpip.go)
	}
	return d == nil || ab.ManaRestrict == 0 || s.restrSrcAdmits(k, p, sl, ab, d)
}

// potentialGapCost reports whether mana ability j of the source at sl, whose cost goes
// beyond a tap, a self-sacrifice and life, is one gorge's potential-mana walk
// admits while its cost can be paid: a tap-another-permanent source with a permanent to
// tap (Springleaf Drum, Baylen's tokens), or a counter cost put on its source once each
// turn (Wall of Roots) or with its {T} (Weather Maker's second ability). They are the shapes kop.ManaAbilityHonoured lists.
func (s *State) potentialGapCost(k *prep, p uint8, sl Slot, j int, ab *kop.Ability) bool {
	c := s.cardAt(sl)
	switch {
	case int(k.ctrAb[c]) == j && (kop.CtrManaAb(ab) || kop.CtrTapSecondManaAb(ab)):
		return ab.Cost.Flags&kop.CostRemoveCtr == 0 || s.perms[sl].ctrN(s, int(ab.Cost.Ctr)) >= int(ab.Cost.N)
	case ab.ManaRider == kop.ManaRiderDeplete && ab.Cost.Flags == kop.CostTap|kop.CostRemoveCtr:
		// a depletion land (Sandstone Needle, Peat Bog): gorge's potentialManaFor counts its
		// whole production while it still holds the counter its cost removes
		return s.perms[sl].ctrN(s, int(ab.Cost.Ctr)) >= int(ab.Cost.N)
	case int(k.ctrAb[c]) == j && kop.StoreManaAb(ab):
		// a storage battery (Mage-Ring Network, Dwarven Hold): its cost
		// announces X ("remove any number"), so gorge's walk prices the
		// activation payable (the fixpoint's StoreManaAb arm reads its mana)
		return true
	case k.tapMana && kop.TapXMana(ab) && int(k.tapXAb[c]) == j: // the first mana ability or a second beside a plain one (mana_tapanother.go tapXAbOf)
		var cs [MaxPerm]Slot
		return s.tapXCands(k, p, sl, &cs) >= int(ab.Cost.N)
	case ab.Cost.Flags&kop.CostSac != 0 && ab.Cost.N > 0:
		// a printed sacrifice-mana ability ("Sacrifice a creature: Add {C}{C}",
		// Ashnod's Altar): gorge's potential walk admits it while the
		// sacrifice has its candidates, priced against the real board like
		// every non-mana cost part (manaAbilityPayablePool, CR 605.1a, CR
		// 601.2g's window census keeps the shape out, so only the potential
		// walk's answer is over-bound here)
		return s.countCostSac(k, s.perms[sl].ctrl, &ab.Cost, sl) >= int(ab.Cost.N)
	}
	return false
}

// potentialCovers reports whether the pool others covers cost's pips and total.
// A hybrid pip ({B/R}) needs one of its two colours beside the coloured pips
// (gorge pays it from the pool's colours, never from a generic mana), so a
// Plains does not fund Graven Cairns' {B/R} fee. A twobrid or Phyrexian cost
// keeps the total-only read.
func potentialCovers(others *[kop.NumManaKinds]int, cost *kop.ManaCost) bool {
	total := 0
	for c, n := range others {
		if int(cost.Pips[c]) > n {
			return false
		}
		total += n
	}
	if total < cost.MV() {
		return false
	}
	if cost.Hyb[0] == 0 || cost.Hyb[0] == kop.HybTwobrid {
		return true
	}
	left := *others
	for c := range left {
		left[c] -= int(cost.Pips[c])
	}
	return hybridCovered(&left, cost.Hyb[:])
}

// hybridCovered assigns each hybrid pip (mask of its two colours) one mana of a
// colour it names, trying both faces of a pip.
func hybridCovered(left *[kop.NumManaKinds]int, hyb []kop.ColorMask) bool {
	if len(hyb) == 0 || hyb[0] == 0 {
		return true
	}
	for m := hyb[0]; m != 0; m &= m - 1 {
		c := bits.TrailingZeros8(uint8(m))
		if left[c] == 0 {
			continue
		}
		left[c]--
		ok := hybridCovered(left, hyb[1:])
		left[c]++
		if ok {
			return true
		}
	}
	return false
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
	grantFirst = grantFirst || grants && s.eachBesideGrant(k, p)  // an each-colour gap among the grants (mana_granteach.go)
	grantFirst = grantFirst || grants && s.wideBesideGrant(k, p)  // a wide-only gap among the grants (mana_grantwide.go)
	// a planner-deferred gap among the grants: the one search runs for a cast
	// too — gorge's census lists neither a deferred source nor a grant, so a
	// cast's prefix search activates it in battlefield order as it does an
	// activation's (mana_granttapsdefer.go)
	grantFirst = grantFirst || grants && s.grantFirstGaps(k, p)
	pref, tapX := s.runPrefixes(k, a, p, grants, grantFirst)
	if s.sp > sp0 || s.npend != 0 {
		empty := s.playNeedsEmptyStack(k, a)
		if s.sp > sp0 && empty || s.npend != 0 {
			s.resolvePrefixPlay(k, a, p, sp0, empty) // the prefix's triggers placed (and, sorcery-speed, resolved) before the play
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
			s.turnFaceCast(k, Obj(a.Source()), uint8(a.Target())) // code-ok: a face cast's face (face_castface.go)
		}
		if a.Target() == castImpTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyImpending {
			s.g.impCast = true // the impending cost (kw_impending.go)
		}
		if a.Target() == castEvokeTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyEvoke {
			s.g.impCast = true // the evoke cost (kw_evoke.go): beginCast marks the entry evoked
		}
		if a.Target() == castDashTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyDash {
			s.g.impCast = true // the dash cost (kw_dash.go): beginCast marks the entry dashed
		}
		if a.Target() == castForetellTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyForetell {
			s.g.impCast = true // the foretell cost (kw_foretell.go): beginCast marks the entry foretold
		}
		if a.Target() == castOverloadTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyOverload {
			s.g.impCast = true // the overload cost (kw_overload.go): beginCast marks the entry overloaded
		}
		if a.Target() == castCleaveTgt && a.Mode() == 0 && k.gen != nil && k.gen.anyCleave {
			s.g.impCast = true // the cleave cost (kw_cleave.go): beginCast puts the cleave ability on the entry
		}
		if a.Target() == castSurgeTgt && a.Mode() == 0 && k.gen != nil && k.gen.anySurge {
			s.g.impCast = true // the surge cost (kw_surge.go): beginCast marks the entry surged
		}
		if a.Target() == castAltTgt && a.Mode() == 0 && k.gen != nil && k.gen.altFree != nil {
			s.g.altCast = true // the card's own mana alternative cost (cast_altfree.go)
		}
		if a.Mode()&castWarp != 0 && a.Mode() != castAirbend && k.gen != nil && k.gen.anyWarpGrant {
			// the warp instance the action names (kw_warpgrant.go): a grant's
			// marks the entry for the cost builder
			wi := s.warpInstances(k, s.prio, Obj(a.Source()))
			s.g.warpGrant = a.Target() < TCode(wi.n) && wi.grant[a.Target()] // code-ok: a warp instance index
		}
		if a.Mode() == 2 && k.gen != nil && k.gen.anyBeam {
			s.noteBeamCast(k, a) // kw_beammeup.go
		}
		if a.Mode()&2 != 0 && a.Target() == TCode(castEscapeTgt) && k.gen != nil && k.gen.anyEscapeGrant {
			s.g.escapeCast = true // the escape cast the action names (kw_escape.go)
		}
		if pref {
			s.payCastPrefixRest(k, a, p) // the cast's untapped prefix part, before the push (mana_resolutionpay.go)
		}
		s.beginCast(k, Obj(a.Source()), a.Mode(), uint8(a.Target())) // code-ok: the target field names a retrace cast (kw_retrace.go)
		if face {
			s.lowerFaceCast(k, p, Obj(a.Source())) // cast_faceroute.go
		}
	case AkCastConspire:
		s.beginConspireCast(k, Obj(a.Source())) // CR 702.78a; kw_conspire.go
	case AkCastCasualty:
		s.beginCasualtyCast(k, Obj(a.Source()), a.Mode())
	case AkActivate:
		if k.gen != nil && s.activateGen(k, a) {
			break
		}
		if pref {
			s.payPrefixRest(k, a, p, tapX) // the prefix's untapped cost part, before the push (mana_resolutionpay.go)
		}
		s.beginActivate(k, Obj(a.Source()), a.Mode())
	case AkTurnFaceUp:
		s.turnFaceUp(k, Obj(a.Source()), p, a.Mode()) // CR 708.6; mode = procedure
	case AkPlot:
		if a.Mode() == plotForetell {
			s.foretellAction(k, Obj(a.Source()), p) // CR 702.143a; kw_foretell.go
			break
		}
		if a.Mode() == plotModeSuspend {
			s.suspendAction(k, Obj(a.Source()), p) // CR 702.62a; kw_suspend.go
			break
		}
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
	if sk == kop.StatCastWithFlash {
		m = s.statM // a Caster$ Player grant reaches every player; per static below
	}
	for m.any() {
		src := m.first()
		m = m.rest()
		if s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || k.gen != nil && s.ovrLoses(k, src) {
			continue // the source lost its abilities (Witness Protection on High Fae Trickster)
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == sk && (sk != kop.StatCastWithFlash || !cd.SelfFlash && (s.ctrlM[p].has(src) || a.Static.StFlags&kop.StFlashAnyCaster != 0)) && s.matchCard(k, a.Static.Filter, c, p, p) &&
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
