package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"math/bits"
)

// Putting a spell or ability on the stack (CR 601.2 / 602.2, gorge
// cast_begin.go continueCast then cast_commit.go payCast), one program per
// entry kind (buildProgram): the entry goes on the stack first (pushCast /
// pushActivation), then a spell's modes and either-or cost, the tap-X
// election, X, the hybrid pips, the targets and their division, the mana
// abilities (the planner's sources tapped into the pool, mana.go), the cost
// objects (sacrifice, counter removal, discard), and payment. A triggered
// ability is pushed first and then asks its targets (gorge trigger_queue.go
// pushTrigger).

// Face-down casts: AkCast mode bit 7 (warp uses 5 and 6). The family is
// read from the card; the stack entry stores kop's family values in mode
// bits 0..1 with seFaceDown, and ab = 255 (no printed spell ability).
const castFaceDown uint8 = 1 << 7

func (s *State) beginCast(k *prep, o Obj, mode uint8) {
	ob := &s.objs[o]
	p := s.prio
	c := ob.card
	var fl uint8
	if mode&1 != 0 {
		fl |= sfKicked
	}
	mayhem := mayhemCast(k, c, mode) // a graveyard cast that is no flashback (kw_mayhem.go)
	if mode&2 != 0 && !mayhem {
		fl |= sfFlashback
		if two, _, g := s.fbInstances(k, p, o); two && g {
			fl |= sfFbGranted // the granted instance (fbInstances)
		}
		if k.gen != nil && s.g.altCast && (k.cards[c].kw&kop.KwFlashback != 0 || k.t.Cards[c].BeamMeUp) {
			fl |= sfFbGranted // a beam me up cast's mana cost, or a flashback grant's
		}
	}
	e := stackEntry{kind: seSpell, obj: o, srcVer: ob.ver, card: c, ab: spellAb(k, c), ctrl: p, flags: fl, trigObj: NoObj}
	if k.gen != nil && k.gen.anyCastBF && k.t.Cards[c].CastBF != 0 {
		// Count$LastStateBattlefieldWithFallback:
		// freeze the count the moment the spell reaches the stack, before
		// targets, divisions and costs (CR 601.2a; gorge castBattlefieldEvent):
		// a permanent a cost then sacrifices still counts (value_castbattlefield.go)
		e.trigMV = s.castBFLiveCount(k, p, NoSlot, k.t.Cards[c].CastBF)
	}
	if mayhem {
		e.trigPl = trigPlMayhem
	}
	if mode&castFaceDown != 0 {
		e.ab = 255
		fam := k.t.Cards[c].MorphFamily
		if fam == 0 {
			fam = kop.MorphMorph
		}
		e.mode = seFaceDown | fam
	}
	if k.gen != nil {
		// warp provenance (CR 702.185a; kw_warp.go): a hand warp is
		// exiled at the next end step, a recast is not warped again
		if mode == castAirbend {
			mode, e.trigPl = castWarpRecast, trigPlAirbend // act_airbend.go
		}
		if mode&castWarp != 0 {
			e.mode |= modeWarped
			if s.g.warpGrant {
				e.trigPl = trigPlWarpG // the grant's warp instance (kw_warpgrant.go)
			}
		}
		s.g.warpGrant = false
		if mode&castWarpRecast != 0 {
			e.mode |= modeWarpRecast
		}
		if s.g.impCast {
			if k.t.Cards[c].HasOverload {
				e.mode |= modeImpended // the overloaded cast (kw_overload.go: the bit reads as overloaded on a HasOverload card)
			} else if k.t.Cards[c].HasEvoke {
				e.spc |= spcEvoked // the evoked cast (kw_evoke.go)
			} else if k.t.Cards[c].HasDash {
				e.mode |= modeImpended // the dashed cast (kw_dash.go: the bit reads as dashed on a HasDash card)
			} else {
				e.mode |= modeImpended // kw_impending.go
			}
			s.g.impCast = false
		}
		if k.anyRoom {
			if mode&castFace != 0 && roomDef(k, c) {
				e.mode |= modeRoomAlt // its reversal turns the card back (face_rooms.go)
			}
			roomCastStat(k, c, mode)
		}
	}
	*s.bld() = build{e: e}
	if k.gen != nil && s.g.altCast {
		s.g.altCast = false
		if mode&2 == 0 {
			s.bld().altAdd = buildAltCost // cast_altfree.go
		} else if !k.t.Cards[c].BeamMeUp || s.g.beamGrant {
			s.bld().altAdd = buildBeamGrant // kw_beammeup.go
		}
		s.g.beamGrant = false
	}
	s.bld().e.tgt = noTargets
	s.bld().stage = stageFirst[seSpell]
	s.ctl = ctlBuild
	if k.gen != nil && k.gen.anyConvoke && mode&3 == 0 && s.tapPays(k, p, c, o, ob.zone == kop.ZoneHand) {
		if _, planned := s.plannedCastCostOf(k, p, o, mode, false, false); planned {
			s.bld().bfl |= bflTargetPaid
		}
	}
	// a cast only a typed permission allows (Muldrotha's per-type statics,
	// gorge's MayPlayPerm) has no planned payment: gorge always lowers it
	// (rules payment_plan.go: "a typed MayPlayText$ permission carries a
	// MayPlayPerm and stays withheld"). Read before the permission is spent.
	typed := s.cfg&(cfgGorgeLowering|cfgGorgeAutopay) != 0 && mode&castMayPlay != 0 && mode&castFree == 0 && !s.FeatMayPlayUntyped(p, o)
	if k.gen != nil {
		s.bld().fbGrants = s.fbGrants(k, o)
		s.castFlagsGen(k, o, mode)
		if k.gen.anyMayRaise && s.mayRaiseLifeOwed(k) {
			typed = true // a life raise is no cost the planner prices ("cost:life"): lowered like a typed permission (cost_mayplayraise.go)
		}
	}
	n0 := s.npend
	if fl&sfKicked != 0 && k.t.Cards[c].Cost.X == 0 && k.t.Cards[c].OptWaterbend == 0 {
		// (an optional waterbend cast taps nothing here: its window pays after
		// the picks, manaStep's waterbendCastWindow, as gorge's convokeAsk
		// precedes its manual window)
		// gorge has no planned kicked cast: the oracle lowers it by tapping
		// the witness's sources at priority, then casts from the floating
		// pool; the mana is spent at payment as usual. The witness is
		// gorge's composed-cost plan (potential_plan.go planComposedCost),
		// which plans with PlannedCast{}: its hand demand (rank key 6)
		// counts the card being cast too, so nothing is excluded.
		cost := s.buildCost(k)
		if s.rbAny(p) {
			// restricted mana the spell may spend (Secluded Courtyard's
			// {R} for a Goblin, tapped by the play's script prefix) is in
			// the witness's pool: tap only for the rest
			cost = s.restrShort(k, p, c, s.restrOriginOfObj(o), cost)
		}
		s.tapFor(k, p, &cost, 0, NoObj)
	} else if mode&castWarp != 0 || k.gen != nil && (k.gen.anyImpending && s.bld().e.impended() || k.gen.anyEvoke && s.bld().e.evoked() || k.gen.anyDash && s.bld().e.dashed(k) || k.gen.anyOverload && s.bld().e.overloaded(k)) {
		// a hand warp is one of gorge's potential plays (the "warped" cast
		// option carries Potential: true), so its plan is the composed cost
		// and the oracle always lowers it, like the kicked cast; so is an
		// impended cast ("impended", kw_impending.go)
		s.bld().bfl |= bflLowered
		if !s.poolRPays(k, p, c, o) {
			s.lowerWitness(k, p)
		}
	} else if !(k.gen != nil && s.contribPays(k, p, c, o, ob.zone == kop.ZoneHand)) && mode&(castFace|castFaceDown|castWarpRecast) == 0 && (s.cfg&cfgGorgeLowering != 0 && mode&3 == 0 && mode != castFree && ob.zone == kop.ZoneHand ||
		s.cfg&(cfgGorgeLowering|cfgGorgeAutopay) != 0 && (mode&castMayPlay != 0 && mode&castFree == 0 || mode&3 == 2)) &&
		(typed || fl&sfFbGranted != 0 || !s.plannedUnderRider(p, o, mode&3)) {
		// the oracle lowers a cast from the hand gorge's cast planner never
		// prices (Config.GorgeLowering), at the price with no target chosen,
		// or at the cheapest legal target when no source funds that
		// (witnessAtCheapest; a free cast -- Etali's -- is no potential play);
		// a cast through a permission (Muldrotha's) under either surface
		// (gorgeOffersMay); a flashback cast the cast planner declines (Luminous
		// Rebuke's target-conditional reduction, or the granted instance beside a
		// printed one: read from fl, since castFlagsGen above has ended
		// the grant FeatPlannedCast reads; pair 87192) under either
		// surface too
		s.bld().bfl |= bflLowered
		// a pool that pays the printed cost leaves no witness to tap, and the
		// cast still has no plan: an either-or part chosen after it goes
		// through the window (bsPay)
		pm := mode & 3
		if s.g.bargainCast {
			pm |= 4 // the bargained price (FeatPoolPays mode bit 2)
		}
		if s.bld().altAdd != buildBeamGrant && !s.FeatPoolPays(p, o, pm) {
			s.lowerWitness(k, p)
		}
	} else if mode&castMayPlay != 0 && mode&castFree != 0 && s.cfg&(cfgGorgeLowering|cfgGorgeAutopay) != 0 && s.freeAltAddLowers(k, p, c, o) {
		// a free may-play cast still owing an either-or part only its mana
		// pays (Silvergill Mentor from Maralen's exile) is priced by gorge's
		// witness at that part (potentialAltAddPlan), so the oracle lowers it
		s.bld().bfl |= bflLowered
		s.lowerWitness(k, p)
	} else if s.cfg&cfgGorgeAutopay != 0 && s.cfg&cfgGorgeLowering == 0 && mode == 0 && ob.zone == kop.ZoneHand && k.t.Cards[c].Cost.X != 0 {
		// the autopay surface's plain {X} cast is one the floating pool alone
		// pays at X = 0 (gorge's planner prices no {X} spell, FeatPlannedCast):
		// the seat chooses X and gorge pays the rest through the manual window,
		// the first useful source in battlefield order, not a witness plan
		s.bld().bfl |= bflLowered
	}
	if s.npend > n0 {
		s.resolveLowerCounterTriggers(k, n0, k.cards[c].instant || s.castFlash(k, p, c))
	}
}

// plannedUnderRider is FeatPlannedCast for the cast being built, whose flags hold the
// rider of a permission it has already spent.
func (s *State) plannedUnderRider(p uint8, o Obj, mode uint8) bool {
	s.g.mayPlayAny = s.bld().e.flags&sfAnyMana != 0
	planned := s.FeatPlannedCast(p, o, mode)
	s.g.mayPlayAny = false
	return planned
}

// tapRiderPlan: gorge's cast planner funds a may-play cast's pips as generic
// under its mana rider (rules payment_plan.go planCastPaymentChecked,
// pay.PlanManaUnderRider), so the sources it taps are that plan's, and payment
// spends the pool under the rider (payRider). When that mana is not affordable
// it taps nothing; the caller's tapForRider then plans the rider's cost.
func (s *State) tapRiderPlan(k *prep, e stackEntry) {
	fl, bf, fg := s.bld().e.flags, s.bld().bfl, s.bld().fixedGen
	s.bld().e.flags &^= sfAnyMana
	s.bld().bfl &^= bflAnyColor | bflCostFixed
	pc := planUnderRider(s.buildCost(k), fl&sfAnyMana != 0, bf&bflAnyColor != 0)
	s.bld().e.flags, s.bld().bfl, s.bld().fixedGen = fl, bf, fg
	var av avail
	s.availFor(k, e.ctrl, &av)
	pcx := pc.AtX(int(e.x))
	if pc.Hyb[0] == 0 && av.affords(&pcx, pipMaskOf(&pcx), 0) {
		s.tapFor(k, e.ctrl, &pc, int(e.x), e.obj)
	}
}

// pushBuild puts the entry being built on the stack: a spell's card leaves
// its zone for the stack.
func (s *State) pushBuild(k *prep) {
	e := &s.bld().e
	if e.kind == seSpell {
		ob := &s.objs[e.obj]
		z := ob.zone
		copyCast := k.gen != nil && k.gen.anyCopyCast && s.copyCastFrom(e.obj)
		if copyCast {
			z = kop.ZoneLibrary // a Play CopyCard$ copy is cast from gorge's holding zone (cast_copycast.go)
		}
		s.bld().origin = z
		if k.gen != nil && k.gen.anyCastProv || copyCast {
			e.prov = uint8(z) // trig_gravedrawn.go, trig_castprovenance.go
		}
		if k.gen != nil && k.gen.castRingOn {
			s.noteCast(e.obj, e.ctrl, z) // CR 601.2a: the push is the cast's ledger entry
			if s.bld().castN != 0 {
				if cd := &k.t.Cards[ob.card]; cd.FaceMode == kop.FaceAdventure && cd.Face == 1 {
					s.castPushes(e.ctrl)[s.bld().castN-1].prov |= castAdventurePush
				}
			}
		} else if k.gen != nil && k.gen.anyParadigm {
			s.noteParadigmCast(k, e.obj, e.ctrl, z) // kw_paradigm.go
		}
		if k.gen != nil && k.gen.anyPrepare && ob.of&ofCopy != 0 {
			s.unprepareCast(e.obj) // CR 722.3c (face_prepare.go)
		}
		s.bld().pushNP = uint8(s.npend)
		if ob.zone == kop.ZoneLibrary {
			s.bld().libPos = s.pl[ob.owner()].libIndex(e.obj)
		}
		s.detach(e.obj)
		s.touch(e.obj)
		if s.g.on {
			s.stashClr(k, e.obj) // it leaves exile: gorge's stash counter goes with the move
		}
		ob.zone = kop.ZoneStack
		ob.ver++
		if s.g.enteredOn && z == kop.ZoneGraveyard {
			s.noteCastFromGrave(e.obj) // cast_leftgraveyard.go
		}
		if e.faceDown() {
			ob.slot = slotFaceDownSpell // CR 708.4 (faceDownAt reads it without a stack walk)
		}
		e.srcVer = ob.ver
	}
	if e.kind == seAbility {
		// CR 733.1: the pending mark for the reversal's drop (unpushActivation),
		// as a spell's push records its own (unpushSpell)
		s.bld().pushNP = uint8(s.npend)
		// gorge's activation census (the view's ActivatedThisTurn) counts
		// the activation as it is pushed (CR 602.2a: AbilityPush comes
		// first), for a source on the battlefield, so a source the cost
		// then sacrifices keeps the count
		if k.gen != nil && k.gen.actRingOn {
			s.noteAct(e.obj, e.ctrl) // CR 602.2a: the push is the activation ledger entry
		}
		if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield {
			if ob.activations() < objActsMask {
				s.touch(e.obj)
				ob.acts++
			} else {
				s.capFull(StallActs)
			}
		}
	}
	if e.kind != seSpell && k.gen != nil && (k.gen.anyStackTgt || k.gen.anyWard || k.gen.anyWardGrant) {
		s.stackSid(e) // a stack-ability target's identity (tgt_stackability.go); a ward trigger's warded ability (wardedIndex)
	}
	s.push(*e)
	s.bld().bfl |= bflPushed
}

// proposalPendHi is the end of the pending triggers a reversed spell's own
// proposal queued (its push, its targets, its dilemma): every one before its
// mana step began, the rest being its mana abilities'.
func (s *State) proposalPendHi() uint8 {
	if m := s.bld().manaNP; m != 0 && m-1 < s.npend {
		return m - 1
	}
	return s.npend
}

// unpushSpell reverses a spell pushed first (CR 601.2a) whose cast is
// undone (CR 733.1): its stack entry is popped and the card returns from the
// stack to the end of the zone it was cast from, as gorge's reversal moves it
// (a hand, a graveyard for a flashback cast, exile).
func (s *State) unpushSpell(k *prep) {
	e := &s.bld().e
	s.sp--
	s.bld().bfl &^= bflPushed
	o := e.obj
	ob := &s.objs[o]
	pl := &s.pl[ob.owner()]
	s.touch(o)
	if k.gen != nil && k.gen.castRingOn {
		s.noteReversed(o, e.ctrl)
	}
	if k.gen != nil && k.gen.anyCopyCast && ob.of&(ofCopy|ofToken) == ofCopy && s.bld().origin == kop.ZoneLibrary {
		// CR 733.1 reverses the copy cast; the copy was never a card, so it
		// ceases (CR 704.5d). gorge's reversal parks it in the library
		// holding zone and the next state-based sweep ceases it: the copy
		// has no library membership, and the mana the window tapped stays
		// tapped and in the pool (loweredWindowOK; CR 733.1 lets the player
		// leave legal mana abilities reversed).
		if s.npend > s.bld().pushNP {
			s.dropPendingLimits(k, s.bld().pushNP)
		}
		ob.zone = kop.ZoneNone
		return
	}
	if hi := s.proposalPendHi(); hi > s.bld().pushNP {
		// CR 733.1: the move off the zone it was cast from never happened, so
		// its triggers (a card leaving a graveyard) are dropped, and the
		// once-per-turn records they consumed come back (gorge abortCast's
		// proposalLimits: an undone action triggered nothing, so a
		// once-each-turn trigger it queued has not used its turn). A mana
		// ability the payment activated is not undone (CR 733.1 lets it
		// stand), so what it queued stays: a creature Springleaf Drum
		// tapped still "becomes tapped" (gorge drops only
		// pendingCast.proposalTriggers).
		s.dropPendingRange(k, s.bld().pushNP, hi)
	}
	ob.zone = s.bld().origin
	ob.slot = NoSlot // off slotFaceDownSpell
	switch s.bld().origin {
	case kop.ZoneHand:
		pl.hand[pl.handN] = o
		ob.of |= ofInHand
		pl.handN++
	case kop.ZoneGraveyard:
		pl.grave[pl.graveN] = o
		pl.graveN++
		if s.g.enteredOn {
			s.unnoteCastFromGrave(o) // CR 733.1: gorge truncates its zone-add list to the push (cast_leftgraveyard.go)
		}
		if k.gen != nil && e.flags&sfFlashback != 0 && (k.cards[e.card].kw&kop.KwFlashback == 0 && !k.t.Cards[e.card].Harmonize || e.flags&sfFbGranted != 0) {
			s.setFBGrants(k, o, s.bld().fbGrants) // gorge keeps the granted flashback, once per grant
		}
	case kop.ZoneExile:
		pl.exile[pl.exileN] = o
		pl.exileN++
	case kop.ZoneLibrary:
		pl.libInsert(o, s.bld().libPos) // CR 733.2: back where it was, not on top
	case kop.ZoneSideboard:
		// outside the game: no list, the object's zone is its place
	case kop.ZoneCommand:
		// the command zone is a place: no list to return the card to
	default:
		panic("kernel: unpushSpell from an unexpected zone")
	}
}

// syncBuild copies the entry being built onto its stack slot. A target
// answer is recorded as it is chosen, for a spell and an activated ability
// alike (CR 601.2c, before the 601.2h costs), so the slot shows it from the
// answer on.
func (s *State) syncBuild() {
	if s.bld().bfl&bflPushed != 0 {
		st := &s.stack[s.sp-1]
		*st = s.bld().e
	}
}

// buildProgram is the CR 601.2 / 602.2 sequence of build stages for each
// entry kind (seSpell, seAbility, seTrigger), in gorge's order
// (cast_begin.go continueCast, then cast_commit.go payCast; a trigger:
// trigger_queue.go pushTrigger). Following a gorge reorder is an edit of
// these rows. A triggered ability is pushed by takePending before its
// program starts.
var buildProgram = [3][]uint8{
	seSpell:   {bsPush, bsModes, bsAltAdd, bsGift, bsCasualty, bsTapCost, bsConspire, bsConvoke, bsX, bsDelve, bsPips, bsTargets, bsDivide, bsEvidence, bsMana, bsCosts, bsPay},
	seAbility: {bsPush, bsModes, bsTapCost, bsX, bsPips, bsTargets, bsDivide, bsEvidence, bsMana, bsCosts, bsPay},
	seTrigger: {bsModes, bsPips, bsTargets, bsDivide, bsPay},
}

// stageNext[kind][stage] is the stage after stage in kind's program;
// stageFirst[kind] its first stage. Both are built once from buildProgram.
var stageNext, stageFirst = func() (nx [3][nBuildStages]uint8, first [3]uint8) {
	for kind, prog := range buildProgram {
		for i := range nx[kind] {
			nx[kind][i] = bsPay
		}
		first[kind] = prog[0]
		for i := 0; i+1 < len(prog); i++ {
			nx[kind][prog[i]] = prog[i+1]
		}
	}
	return
}()

// toStage enters build stage st, running its entry step: the targets start
// at slot 0, the division computes its points (startCastDivide).
func (s *State) toStage(k *prep, st uint8) {
	s.bld().stage = st
	switch st {
	case bsTargets:
		s.bld().ti = 0
	case bsDivide:
		s.startCastDivide(k)
	}
}

// nextStage enters the stage after the current one in the entry's program.
func (s *State) nextStage(k *prep) {
	if s.bld().stage == bsTargets {
		// gorge meets the targeting before the cast it is part of (gorge
		// cast.go proposalTriggers): the ward and BecomesTarget triggers the
		// CR 601.2c target choice queues are enqueued at the target answer,
		// before any payment -- a cost's sacrifice that dies during the
		// payment queues its own triggers behind them, and that order is
		// what the CR 603.3b ordering ask indexes. A reversed cast drops
		// them (CR 733.1: unpushSpell / unpushActivation).
		s.checkWard(k, &s.bld().e)
	}
	s.toStage(k, stageNext[s.bld().e.kind][s.bld().stage&^0x80])
}

func (s *State) beginActivate(k *prep, o Obj, ab uint8) {
	ob := &s.objs[o]
	*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: s.cardAt(ob.slot), ab: ab, ctrl: s.prio, trigObj: NoObj}}
	s.bld().e.tgt = noTargets
	s.bld().stage = stageFirst[seAbility]
	s.ctl = ctlBuild
	if s.cfg&cfgGorgeUnpriced != 0 && ab < abCascade {
		if c := &k.t.Cards[s.bld().e.card].Abilities[ab].Cost.Mana; c.Hyb[0] != 0 && !c.Phyrexian() && c.X == 0 && c.Generic == 0 && pipMaskOf(c) == 0 {
			s.lowerHybridActivation(k, c)
		}
	}
}

// buildAbility returns the ability being built (nil for a permanent spell).
func (s *State) buildAbility(k *prep) *kop.Ability {
	if s.bld().e.ab == 255 || s.bld().e.ab >= abCascade {
		return nil
	}
	return &k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab]
}

// buildingPowerUp reports whether the build in progress activates a
// Power-up ability, whose asks (X, hybrid pip faces) read the CR 702.193b
// reduced charge (gorge composes powerUpReduceCost into the activation's cost
// before any ask).
func (s *State) buildingPowerUp(k *prep) bool {
	e := &s.bld().e
	return e.kind == seAbility && k.gen != nil && k.gen.anyPowerUp && k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostPowerUp != 0
}

func (s *State) buildCost(k *prep) kop.ManaCost {
	return s.buildCostResolve(k, false)
}

// buildCostCharge is buildCost for the CR 702.193b charge: a Power-up
// ability's printed mana cost is reduced before its hybrid symbols are
// resolved, because the reduction never touches a hybrid symbol (gorge
// powerUpReduceCost reads only Generic and Colored). gorge's payment planner
// prices the UNREDUCED cost (planAbilityPayment does not call
// powerUpReduceCost) while the activation charges the reduced one, so the
// witness and the charge read different costs; this is the charge half.
func (s *State) buildCostCharge(k *prep) kop.ManaCost {
	return s.buildCostResolve(k, true)
}

func (s *State) buildCostResolve(k *prep, charge bool) kop.ManaCost {
	return s.delveApplyPicks(k, s.buildCostResolve0(k, charge)) // (the fixedGen cache holds the pre-credit generic)
}

func (s *State) buildCostResolve0(k *prep, charge bool) kop.ManaCost {
	s.bld().bfl |= bflCostComp // gorge withCostCompositionEvent: a pushed spell's own push is hidden while its cost is composed
	c, tbDone := s.buildCostPre(k, true, charge)
	if s.bld().bfl&bflCostFixed != 0 {
		c.Generic = s.bld().fixedGen
	} else {
		s.bld().fixedGen = c.Generic
	}
	s.bld().bfl &^= bflCostComp
	if c.Hyb[0] == 0 && !tbDone {
		return c
	}
	if c.Hyb[0] == kop.HybTwobrid || tbDone {
		if !tbDone {
			c = resolveTbBuild(c, s.bld().pipN, s.bld().pipC[0])
		}
		if s.bld().e.flags&sfAnyMana != 0 && s.bld().e.kind == seSpell {
			c = anyTypePips(c)
		}
		return c
	}
	for i := 0; i < int(s.bld().pipN) && c.Hyb[0] != 0; i++ {
		c = c.Resolve(s.bld().pipCol(uint8(i)))
	}
	if s.bld().e.flags&sfAnyMana != 0 && s.bld().e.kind == seSpell {
		c = anyTypePips(c)
	}
	return c
}

func (s *State) buildCost0(k *prep) kop.ManaCost {
	c, _ := s.buildCostPre(k, false, false)
	return c
}

// buildCostPre is buildCost0; with pre it folds the twobrid faces already
// announced into a spell's printed cost BEFORE the cost statics apply, as
// gorge composes the announced faces and then the modifiers (CR 601.2f:
// a reduction takes its generic off the {2} a generic face added, where the
// unresolved {2/W} would have dropped it). done reports that the faces are
// folded, so the caller resolves nothing more.
func (s *State) buildCostPre(k *prep, pre, charge bool) (kop.ManaCost, bool) {
	c, done := s.buildCostPreRaw(k, pre, charge)
	if s.cfg&cfgCommander != 0 && s.bld().e.kind == seSpell {
		// CR 903.8: the command-zone tax, after the cost statics, so a
		// reduction never takes it off
		addCommanderTax(&c, s.commanderTax(s.bld().e.ctrl, s.bld().e.obj))
	}
	if s.bld().e.flags&sfAnyMana != 0 && s.bld().e.kind == seSpell {
		c = anyTypePips(c) // a hand cast under Vizier's conversion too (mana_convertcreature.go); idempotent
	}
	return c, done
}

func (s *State) buildCostPreRaw(k *prep, pre, charge bool) (kop.ManaCost, bool) {
	if s.bld().e.faceDown() {
		// CR 702.37b/702.168b: Morph and Disguise cast face down for {3};
		// Megamorph's face-down cost is its Morph cost ({3} in this pool).
		c := kop.ManaCost{Generic: 3}
		if s.statM.any() || k.gen != nil {
			c = s.adjustCost(k, s.bld().e.ctrl, s.bld().e.card, s.bld().e.obj, c)
		}
		return c, false
	}
	if k.gen != nil {
		if c, done, ok := s.buildCostGenPre(k, pre); ok {
			if k.gen.anyWaterbend && spellWaterbends(k, s.bld().e.card) {
				c = s.waterbendApplyPicks(c) // a spell's own waterbend (act_waterbend.go)
			}
			return c, done
		}
	}
	e := &s.bld().e
	switch e.kind {
	case seSpell:
		cd := &k.t.Cards[e.card]
		c := cd.Cost
		if e.flags&sfFlashback != 0 && ((cd.Keywords&kop.KwFlashback != 0 || cd.BeamMeUp) && e.flags&sfFbGranted == 0 || cd.Harmonize) {
			c = cd.AltCost // a granted flashback costs the mana cost
		} else if e.flags&sfFlashback != 0 && k.gen != nil && k.gen.anyFbGrant && !cd.BeamMeUp {
			c = s.fbGrantCost(k, e.ctrl, e.obj, s.bld().fbGrants != 0) // or a live grant's own cost (kw_fbgrant.go)
		}
		if e.mode&modeWarped != 0 && e.trigPl == trigPlWarpG {
			c = s.warpGrantCost(k, e) // a battlefield grant's warp cost (kw_warpgrant.go)
		} else if e.mode&modeWarped != 0 && cd.HasWarp {
			c = cd.WarpCost // the hand's warp cost (CR 702.185a)
		}
		if cd.Impending > 0 && e.impended() {
			c = cd.ImpendingCost // the impending cost (CR 702.176a; kw_impending.go)
		}
		if cd.HasEvoke && e.evoked() {
			c = cd.EvokeCost // the evoke cost (CR 702.74a; kw_evoke.go)
		}
		if cd.HasDash && e.impended() {
			c = cd.DashCost // the dash cost (CR 702.109a; kw_dash.go)
		}
		if cd.HasOverload && e.overloaded(k) {
			c = cd.OverloadCost // the overload cost (CR 702.96a; kw_overload.go)
		}
		if e.mode&modeBestowed != 0 && e.bestowedEntry() {
			c = cd.BestowCost // the bestow cost (CR 702.103a; kw_bestow.go)
		}
		if cd.Mayhem && e.trigPl == trigPlMayhem {
			c = cd.AltCost // the mayhem cost (CR 702.187a; kw_mayhem.go)
		}
		if e.trigPl == trigPlMadness && cd.HasMadness {
			c = cd.MadnessCost // the madness cast from exile (CR 702.35b)
		}
		if e.trigPl == trigPlMiracle && cd.HasMiracle {
			c = cd.MiracleCost // the miracle cast from hand (CR 702.94a; kw_miracle.go)
		}
		if k.gen != nil && k.gen.anySneak && e.trigPl == trigPlSneak && cd.HasSneak {
			c = cd.SneakCost // the sneak cost (CR 702.190a; kw_sneak.go)
		}
		if k.gen != nil && k.gen.anyWebSling && e.trigPl == trigPlWebSling && cd.HasWebSling {
			c = cd.WebSlingCost // the web-slinging cost (CR 702.188a; kw_websling.go)
		}
		if e.flags&sfKicked != 0 {
			c = addCost(c, cd.Kicker)
		}
		if k.gen != nil && k.gen.anyMayFlash && mayFlashEntry(e) && cd.HasMayFlash && !DebugMayFlashFree {
			c = addCost(c, cd.MayFlashCost) // the mana cost plus the extra (CR 702.8; kw_mayflash.go)
		}
		if cd.HasAltAdd && s.bld().altAdd == 1 {
			c = addCost(c, cd.AltAddCost.Mana)
		}
		if cd.HasModeCost {
			c = spreeCost(cd, e.mode, c) // kw_spree.go
		}
		// a reduction applies to the total cost, a flashback or kicked one
		// too (CR 601.2f)
		done := false
		if pre {
			c, done = s.foldTb(k, c)
		}
		if s.statM.any() || k.gen != nil {
			c = s.adjustCost(k, e.ctrl, e.card, e.obj, c)
		}
		if k.gen != nil && k.gen.anyWaterbend && spellWaterbends(k, e.card) {
			c = s.waterbendApplyPicks(c) // a spell's own waterbend (act_waterbend.go)
		}
		return c, done
	case seAbility:
		a := &k.t.Cards[e.card].Abilities[e.ab]
		c := a.Cost.Mana
		if k.gen != nil && k.gen.anyOwnReduce {
			if a.OwnReduce != (kop.ValueRef{}) {
				c = s.ownReduced(k, a, e.obj, e.ctrl)
			}
		}
		if charge && k.gen != nil && k.gen.anyPowerUp && a.Cost.Flags&kop.CostPowerUp != 0 {
			c = s.powerUpReduced(k, e.obj, c) // CR 702.193b, before the cost statics
		}
		if k.gen != nil && k.gen.anyAbReduce {
			c = s.abReducedCharge(k, a, e.obj, e.ctrl, c)
		}
		if k.gen != nil && k.gen.anyWaterbend {
			c = s.waterbendApplyPicks(c)
		}
		return c, false
	}
	return kop.ManaCost{}, false
}

func (s *State) srcSlot() Slot {
	e := &s.bld().e
	if e.kind == seSpell {
		return NoSlot
	}
	ob := &s.objs[e.obj]
	if ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
		return ob.slot
	}
	return NoSlot
}

// buildStep advances the entry being built; it returns true with d filled
// when a decision is needed.
func (s *State) buildStep(k *prep, d *Decision) bool {
	e := &s.bld().e
	if s.bld().bfl&bflWinTap != 0 {
		return s.castWinTapOptions(k, d) // the cast window waits on a tap-another pick (mana_window.go)
	}
	for {
		switch s.bld().stage {
		case bsMiracle:
			s.miracleRevealOptions(d)
			return true
		case bsPush:
			if s.bld().bfl&bflPushed == 0 {
				if k.gen != nil && k.gen.anyStation && s.stationPick(k, d) {
					return true // Station's tap pick precedes the push (ctr_kinds.go)
				}
				if (e.kind == seSpell || e.kind == seAbility && k.gen != nil && (k.gen.anyWaterbend || k.gen.anyXBoundAb)) && s.noProgress() {
					s.bld().bfl |= bflNoProg
				}
				s.pushBuild(k)
			}
			if e.faceDown() {
				s.toStage(k, bsDelve) // CR 601.2: no modes, alt cost, gift, tap, X, pips, targets or divide; a granted delve still applies, then bsMana
			} else {
				s.nextStage(k)
			}
			continue
		case bsAltAdd:
			if e.kind == seSpell && k.t.Cards[e.card].HasAltAdd {
				if !s.altAddPayable(k, 0) && !s.altAddPayable(k, 1) {
					// gorge altAddAsk: no payable part aborts the cast with
					// no ask and nothing tapped (CR 733.1)
					s.reverseCast(k)
					return false
				}
				s.altAddOptions(k, d)
				return true
			}
			s.nextStage(k)
			continue
		case bsGift:
			if k.gift && e.kind == seSpell && k.t.Cards[e.card].Gift {
				s.giftOptions(d)
				return true
			}
			s.nextStage(k)
			continue
		case bsCasualty:
			if s.bld().cas == casElected && e.kind == seSpell && s.casualtyOptions(k, d) {
				return true
			}
			s.nextStage(k)
			continue
		case bsTapCost:
			if k.gen != nil && k.gen.anyTapPower && s.tapPowerOptions(k, d) {
				return true
			}
			if s.costPickOptions(k, d, kop.CostTapX) {
				return s.ctl == ctlBuild // false: a short tap cost reversed the build (sacShort)
			}
			if k.gen != nil && k.gen.anyWaterbend && s.waterbendOptions(k, d) {
				return true
			}
			s.nextStage(k)
			continue
		case bsConspire:
			if s.bld().cas >= casConspire && s.bld().cas <= casConspHalf && e.kind == seSpell && s.conspireOptions(k, d) {
				return true
			}
			s.nextStage(k)
			continue
		case bsConvoke:
			if s.convokeOptions(k, d) {
				return true
			}
			s.nextStage(k)
			continue
		case bsDelve:
			if s.delveTbDeferred(k) {
				// a twobrid cost's resolved generic -- the ask's shortfall --
				// is not known until CR 601.2b announces the pips; the ask
				// re-runs at bsPips' tail once they are done (gorge
				// delveAsk's defer, cast_subcounter.go)
				s.nextStage(k)
				continue
			}
			if s.delveOptions(k, d) {
				return true
			}
			s.endDelveStage(k)
			continue
		case bsX, bsX | 0x80: // posing the X ask again (a re-step) poses the same ask
			if e.kind == seTrigger {
				s.nextStage(k)
				continue
			}
			cost := s.buildCost(k)
			if s.buildingPowerUp(k) {
				cost = s.buildCostCharge(k) // a power-up's X is announced against the reduced cost (CR 702.193b, gorge xAsk)
			}
			if cost.X == 0 {
				if e.kind == seAbility && k.gen != nil && k.gen.anyLoyaltyX && s.loyaltyXOptions(k, d) {
					return true // a planeswalker's [-X] (cost_loyaltyx.go)
				}
				if (k.gen == nil || !k.gen.anyLifeX || !s.lifeXBuilding(k)) && (k.gen == nil || !k.gen.anySacX || !s.sacXBuilding(k)) {
					s.nextStage(k)
					continue
				}
				// an announced PayLife<X> or Sac<X/Spec> with no {X} mana
				// asks X below (cost_paylifex.go, cost_sacx.go)
			}
			// X is announced (CR 601.2b) before the payment window opens, so it is
			// bounded by what that window can pay (CR 601.2g; gorge xAsk).
			var av avail
			s.availXWindow(k, e.ctrl, &av)
			if s.bld().pickN != 0 {
				s.excludePickedSources(k, &av) // a waterbend/convoke pick cannot also tap for mana
			}
			if e.kind == seAbility {
				s.excludeTapSource(k, &av)
				if m := s.restrActNow(k, e.ctrl); m != 0 {
					s.restrAvailAdd(e.ctrl, m, &av)
				}
			} else if s.rbAny(e.ctrl & 1) {
				// restricted mana a spell may spend (Secluded Courtyard's
				// chosen type, tapped by the play's script prefix) is in
				// gorge's AvailableFor pool, which its xAsk reads; the
				// cast was offered on it (legal.go restrAffords)
				s.restrAvail(k, e.ctrl, e.card, restrOriginOfEntry(e), &av)
			}
			mx := av.maxX(&cost, pipMaskOf(&cost))
			gorgeSurface := s.cfg&cfgGorgeSurface != 0
			if k.restrSrc || k.gen != nil && k.gen.anyMultiR {
				mx = max(mx, s.restrSrcMaxX(k, e.ctrl, &cost, &av)) // the restricted sources the payment may spend (a wide any-colour one included)
			}
			anyColor := s.bld().bfl&bflAnyColor != 0
			if anyColor {
				mx = av.maxXAnyColor(&cost)
			}
			if k.gen != nil && k.gen.anyFilt {
				mx = max(mx, s.filtMaxX(k, e.ctrl, &cost, &av, anyColor)) // CR 605.3b: a filter's fee paid in the window
			}
			if cost.XRed > 0 && gorgeSurface {
				// gorge's xAsk loop stops at pool + graveyard + convoke credit
				// + 1 + what the window's sources make, raised by the cast's
				// composed reductions (CR 601.2f: a reduction is mana the
				// announced X can spend). Its excess over the printed generic
				// is XRed, and the pool's pip demand never falls below the
				// reduction's non-excess part, so XRed is exactly the raise
				// the ask's ceiling needs: the same largest X is offered.
				mx = min(mx, av.total+s.filtBoundExtra(k, e.ctrl)+int(s.pl[SeatIx(e.ctrl)].graveN)+s.convokeXCredit(k)+1+int(cost.XRed))
			}
			if k.gen != nil && k.gen.anyLifeX {
				mx = s.lifeXClamp(k, &cost, mx) // PayLife<X> bounds the ask by life
			}
			if k.gen != nil && k.gen.anySacX {
				mx = s.sacXClamp(k, &cost, mx) // Sac<X/Spec> bounds the ask by the sacrifice candidates
			}
			d.Kind, d.Sub, d.Player = DkChoose, ChX, e.ctrl
			hi := s.xReducedCeiling(k, e, &cost, &av, mx)
			if np := s.convokeXCredit(k); np > 0 {
				hi = min(maxXAsk, max(hi, mx+np)) // each announced pick pays one generic (a harmonize pick its power) of the X total
			}
			if e.kind == seAbility && k.gen != nil && k.gen.anyLoyaltyX {
				mx, hi = s.counterXClamp(k, mx, hi) // a shared {X} / SubCounter<X/KIND> is bounded by the source's counters
			}
			if k.gen != nil && k.gen.anySacX && cost.X != 0 {
				hi = s.sacXClamp(k, &cost, hi) // (announced only, hi is the clamped mx already)
			}
			if !s.addXOptions(k, d, e, mx, hi, &av) {
				return s.ctl == ctlBuild // false: no X above the floor is payable, the cast reversed (cost_sacx.go)
			}
			s.bld().stage = bsX | 0x80 // awaiting X
			return true
		case bsCosts:
			if e.faceDown() {
				s.nextStage(k) // CR 702.37b: no additional costs on a face-down cast
				continue
			}
			if e.kind == seSpell {
				// any spell build can owe a fixed PayLife<N> the offer's cost
				// census did not price (Wishing Well's Play). A cast with no
				// target asks nothing before this stage, so the life part would
				// otherwise be paid into negative life instead of reversing
				// (CR 119.4, 601.2h, 733.1); this applies to any spell, not
				// only a free cast. A PayLife<X> is bounded by the X ask.
				// buildPickCost is the chosen either-or part: an alt-add cast
				// paying its mana part owes no life (Redirect Lightning).
				if pc := s.buildPickCost(k); pc != nil && pc.Flags&kop.CostPayLife != 0 && int(s.pl[SeatIx(e.ctrl)].life) < int(pc.N) {
					s.reverseCast(k)
					return false
				}
			}
			// gorge's order: sacrifice, the counter removals, discard
			// (the forage either-or first: gorge forageAsk; an exile
			// part is the only counted one of its cost: act_forage.go)
			if k.costObj && s.costObjOptions(k, d) || s.costPickOptions(k, d, kop.CostSac) || s.subCtrOptions(k, d) || k.subAmong && s.subAmongOptions(k, d) || s.costPickOptions(k, d, kop.CostDiscard) {
				return s.ctl == ctlBuild // false: a short sacrifice reversed the build (sacShort)
			}
			if k.gen != nil && (k.gen.anySneak && sneakEntry(e) || k.gen.anyWebSling && webSlingEntry(e)) && s.sneakReturnOptions(k, d) {
				// the sneak or web-slinging cast's return, after the other
				// cost-object picks (gorge's sac, subcounter, discard,
				// exile, then return; kw_sneak.go)
				return s.ctl == ctlBuild // false: no return candidate reversed the build
			}
			s.nextStage(k)
			continue
		case bsModes:
			a := s.buildAbility(k)
			if a == nil || a.NModes == 0 {
				s.nextStage(k)
				continue
			}
			pick := a.ModesPick
			if k.teamwork {
				if tw := s.twModes(k); tw != 0 {
					pick = tw // the teamworked cast's "choose both" (kw_teamwork.go)
				}
			}
			if k.gen != nil && k.gen.anyCharmV {
				if c, dyn := s.charmCap(k); dyn {
					pick = c // CharmNum$ over a dynamic count (modal_charmcmp.go)
					if c == 0 {
						// choose up to zero: a trigger with no mode is
						// removed (CR 603.3c), a spell announces none
						if e.kind == seTrigger {
							s.sp--
							s.ctl = ctlPlaceTriggers
							return false
						}
						s.bld().modesN = 255
						s.nextStage(k)
						continue
					}
				}
			}
			if s.bld().modesN >= max8(pick, 1) || e.kind == seAbility && s.modalActAtResolution(a, pick) {
				if k.gen != nil && k.gen.anyDilemma {
					s.buildModesChosen(k, a) // trig_dilemma.go
				}
				s.nextStage(k)
				continue
			}
			if pick > 1 || a.ModesFew != 0 || a.Repeats() { // "choose up to one" takes the multi-pick step
				switch s.charmModesStep(k, a, d) {
				case cmPosed:
					return true
				case cmGone:
					return false
				case cmAbort:
					return false
				}
				if k.gen != nil && k.gen.anyDilemma {
					s.buildModesChosen(k, a) // trig_dilemma.go
				}
				s.nextStage(k)
				continue
			}
			d.Kind, d.Player = DkModes, e.ctrl
			for m := 0; m < int(a.NModes); m++ {
				if e.mode&(1<<m) != 0 {
					continue
				}
				if !s.modeFeasible(k, a, m) {
					continue
				}
				d.add(MakeAction(AkMode, 0, 0, uint8(m)))
			}
			if d.N == 0 {
				if e.kind == seTrigger {
					// CR 603.3c: with no mode to choose, it is removed
					s.sp--
					s.ctl = ctlPlaceTriggers
					return false
				}
				s.bld().modesN = 255
				continue
			}
			return true
		case bsPips:
			if e.kind != seTrigger && s.pipOptions(k, d) {
				return true
			}
			if s.altAddPipsFirst(k) && s.bld().stage == bsPips && s.bld().altAdd <= 1 && !s.altAddPayable(k, s.bld().altAdd) {
				// the pips are answered and the part chosen cannot be paid:
				// gorge's payment window reverses the cast (CR 733.1)
				s.altAddWindowTap(k)
				s.reverseCast(k)
				return false
			}
			// a twobrid spell deferred its delve ask past the pip
			// announcements (delveTbDeferred): every pip is announced now and
			// the folded cost's generic is the ask's real shortfall
			if e.kind == seSpell && k.gen != nil && k.gen.anyDelve && s.bld().pipN > 0 && s.buildIsTb(k) {
				s.toStage(k, bsDelve)
				continue
			}
			s.nextStage(k)
			continue
		case bsTargets:
			ti := int(s.bld().ti)
			if ti >= kop.MaxTargets {
				s.nextStage(k)
				continue
			}
			if e.kind == seSpell && k.gen != nil && k.gen.anyOverload && e.overloaded(k) {
				// overload has no targets to announce: "target" reads "each"
				// (CR 702.96b; kw_overload.go)
				s.bld().ti = kop.MaxTargets
				s.nextStage(k)
				continue
			}
			if ti == 0 && k.gen != nil && k.gen.anyCharmNum && s.charmCombined(k) {
				return s.charmTargetsStep(k, d)
			}
			t := tspecE(k, e, ti)
			if e.ab == abProwess {
				t = nil
			}
			if t == nil || t.Kind == kop.TgtNone {
				s.nextStage(k)
				continue
			}
			if a := s.buildAbility(k); a != nil {
				if tm := a.TgtModes[ti]; tm != 0 && tm&a.ModeSet(e.mode) == 0 {
					s.bld().ti++
					continue
				}
				if k.gen != nil && k.gen.anyCharmNum && s.charmRootAsked(k, a, ti) {
					s.bld().ti++ // announced by the combined ask
					continue
				}
			}
			if t.TFlags&kop.TgfAtResolution != 0 {
				s.bld().ti++ // chosen mid-resolution (OpChooseTargets), as gorge asks it
				continue
			}
			if !s.tgtAsksNothing(k, t, e) && (s.convokeTargetAbort(k) || s.altAddTargetAbort(k) || s.addCostTargetAbort(k) || s.copyCastTargetWindowShort(k) || s.copyCastXTargetAbort(k, t)) {
				// Gorge's target menu (affordableTargetCandidates) probes the
				// CR 601.2g window and withholds every candidate of a convoke
				// cast the announced picks plus the pool cannot complete: a
				// required target aborts the cast (cast_begin.go's "no legal
				// target"), an up-to target asks nothing and payCast reverses.
				if !t.UpTo {
					s.reverseCast(k)
					return false
				}
				s.bld().ti = kop.MaxTargets
				s.nextStage(k)
				continue
			}
			if e.kind == seSpell && k.t.Cards[e.card].HasModeCost && !s.spreeAffordable(k) && !s.charmSeqSlot(k, ti) {
				// the same census for a Spree cast whose chosen modes cost
				// more than it can pay (kw_spree.go); a sequential charm ask
				// has none (askCharmSeqSlot reads the legal candidates)
				if !t.UpTo {
					s.reverseCast(k)
					return false
				}
				s.bld().ti = kop.MaxTargets
				s.nextStage(k)
				continue
			}
			if k.gen != nil && k.gen.anyCopyCast && s.copyCastTargetAbort(k, t) {
				// a mode-gated required target of a paid copy cast the pool
				// cannot cover (cast_copycast.go)
				s.reverseCast(k)
				return false
			}
			if k.gen != nil && k.gen.anyWaterbend && s.waterbendTargetAbort(k) {
				// the same census after a waterbend ability's picks
				// (act_waterbend.go)
				if !t.UpTo {
					s.reverseCast(k)
					return false
				}
				s.bld().ti = kop.MaxTargets
				s.nextStage(k)
				continue
			}
			if k.gen != nil && s.tapXTargetAbort(k) {
				// the same census after an ability's tap-another picks took
				// the mana sources its cost needs (cast_tapxtarget.go)
				if !t.UpTo {
					s.reverseCast(k)
					return false
				}
				s.bld().ti = kop.MaxTargets
				s.nextStage(k)
				continue
			}
			if k.gen != nil && k.gen.anyCharmNum {
				if a := s.buildAbility(k); a != nil && !s.charmSeqPrime(k, a, ti) {
					return false // a sequential charm ask (modal_charmseq.go)
				}
			}
			if k.gen != nil && multiSlot(t) {
				switch s.multiTargetStep(k, d, t, uint8(ti)) {
				case mtPosed:
					return true
				case mtAbort:
					return false
				}
				continue
			}
			var buf [MaxOptions]uint8
			n := s.targetCands(k, t, e.ctrl, s.srcSlot(), e.obj, &buf)
			if k.gen != nil {
				n = s.affordableTargets(k, &buf, n)
				if k.gen.anyUniqLater {
					n = s.uniqueChainViable(k, uint8(ti), &buf, n) // tgt_uniquechain.go
				}
				if n == 0 && t.UpTo {
					// gorge asks no zero-minimum target with no legal
					// candidate (cast_begin.go continueCast: "a zero-minimum
					// one with no legal candidate asks nothing"), a
					// trigger's or a spell's (Fiery Annihilation's "up to
					// one target Equipment attached to that creature")
					s.bld().ti++
					continue
				}
			}
			if n == 0 && !t.UpTo {
				// no legal target: a trigger is removed (CR 603.3d)
				if e.kind == seTrigger {
					s.sp-- // the pushed trigger leaves the stack
					s.ctl = ctlPlaceTriggers
					return false
				}
				if e.kind == seSpell {
					// the offer judged the targets with the card still in
					// its zone; on the stack (CR 601.2a) a target whose
					// legality read that zone can be gone (Consuming
					// Aberration's power counts the caster's graveyard, a
					// flashback cast leaves it): the proposal is reversed
					// (CR 601.2c, 733.1), as gorge's target ask aborts
					// it ("cast aborted: no legal target")
					s.reverseCast(k)
					return false
				}
				if xBoundTarget(k.t, t) {
					s.reverseCast(k) // the offer did not know its X
					return false
				}
				if _, _, ok := s.equipTargetPriced(k); ok {
					s.reverseCast(k) // no candidate's own equip price is payable (statics_gen.go)
					return false
				}
				// an activation was checked feasible and moves nothing
				// before its targets
				panic("kernel: spell/ability without a legal target")
			}
			d.Kind, d.Player = DkTarget, e.ctrl
			for i := 0; i < n; i++ {
				d.add(MakeAction(AkTarget, 0, buf[i], 0))
			}
			if t.UpTo {
				d.add(MakeAction(AkTarget, 0, NoTarget, 0))
			}
			return true
		case bsDivide:
			if s.bld().divLeft == 0 {
				s.nextStage(k)
				continue
			}
			s.castDivideOptions(k, d)
			return true
		case bsEvidence:
			if n := s.buildEvidenceN(k); n != 0 {
				if s.graveTotalMV(k, s.bld().e.ctrl) < int(n) {
					// a named amount the graveyard cannot reach: gorge aborts
					// the cast (CR 733.1; cast_targets.go evidenceAsk)
					s.reverseCast(k)
					return false
				}
				s.evidenceStageOpts(k, d, int(n))
				return true
			}
			s.nextStage(k)
			continue
		case bsMana:
			if s.manaStep(k) {
				return false
			}
			if s.winPaused() {
				return s.castWinTapOptions(k, d)
			}
			if s.parkedWaiting() {
				s.ctl = ctlExileOrder // a mana sacrifice's move; the answer floats its mana and this step goes on
				return false
			}
			s.nextStage(k)
			continue
		case bsPay:
			s.finishBuild(k)
			if s.winPaused() && s.buildLive() {
				return s.castWinTapOptions(k, d)
			}
			return false
		default:
			panic("kernel: bad build stage")
		}
	}
}

// pipOptions poses the next hybrid pip's colour (gorge rules manaAsk ->
// pay.ManaAsk, CR 601.2b): one DkChoose/ChGenPip per pip, mode = a colour
// whose face leaves the whole cost (X included, the later pips any
// colour) payable from the pool and untapped sources -- the first face
// when none does, as gorge offers. It reports whether it asked.
func (s *State) pipOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	cost := s.buildCost(k)
	if s.buildingPowerUp(k) {
		cost = s.buildCostCharge(k) // the faces are priced against the CR 702.193b charge, as gorge's pip ask reads the reduced cost
	}
	if cost.Hyb[0] == 0 {
		return false
	}
	var av avail
	// the window's sources are what the payment can spend (CR 106.6): a
	// restricted grant (Mm'menon's blue for a spell cast from anywhere other
	// than the hand) is no unit of a pip ask the building hand warp cannot
	// spend it on, as gorge's announceFeasible probes the same sources the
	// window can activate (the X ask's availXWindow keeps the same rule)
	av.xd, av.xdOn = s.payDescNow(NoObj), true
	s.availCastWindowFor(k, e.ctrl, &av)
	if e.kind == seAbility {
		s.excludeTapSource(k, &av)
	} else if s.rbAny(e.ctrl & 1) {
		s.restrAvail(k, e.ctrl, e.card, restrOriginOfEntry(e), &av) // the restricted mana the spell may spend, as the X ask reads it
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChGenPip, e.ctrl
	if cost.Hyb[0] == kop.HybTwobrid {
		s.pipOptionsTb(k, d, cost, &av)
		return true
	}
	first, second := hybColours(cost, 0)
	for _, c := range [2]int{first, second} {
		r := cost.Resolve(c)
		if e.flags&sfAnyMana != 0 && e.kind == seSpell {
			r = hybridGeneric(anyTypePips(r)) // the pips still to announce are payable by any mana too
		}
		rx := r.AtX(int(e.x))
		if s.faceReachable(k, e.ctrl, &av, &rx, 0) || hybGateMutation(s.bld().pipN) {
			d.add(MakeAction(AkChoose, 0, 0, uint8(c)))
		}
	}
	if d.N == 0 {
		d.add(MakeAction(AkChoose, 0, 0, uint8(first)))
	}
	return true
}

func max8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}

// modeFeasible: mode m's mandatory targets have candidates.
func (s *State) modeFeasible(k *prep, a *kop.Ability, m int) bool {
	if k.gen != nil && k.gen.anyCharmPick && s.charmPickBarred(a, s.srcSlot(), m) {
		return false // ChoiceRestriction$: chosen before (modal_charmpick.go)
	}
	for i := 0; i < int(a.NTargets); i++ {
		t := &a.Targets[i]
		if t.UpTo || (a.TgtModes[i] != 0 && a.TgtModes[i]&(1<<m) == 0) {
			continue
		}
		if !s.hasTarget(k, t, s.bld().e.ctrl, s.srcSlot(), s.bld().e.obj) {
			return false
		}
	}
	if s.bld().e.kind == seSpell && k.t.Cards[s.bld().e.card].HasModeCost {
		return s.spreeModeAffordable(k, m) // kw_spree.go
	}
	return true
}

func (s *State) excludeTapSource(k *prep, av *avail) {
	sl := s.srcSlot()
	if sl == NoSlot {
		return
	}
	a := &k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab]
	if a.Cost.Flags&kop.CostTap == 0 || !s.canTapForMana(k, sl) {
		return
	}
	pc := &k.cards[s.cardAt(sl)]
	if k.gen != nil {
		pc = s.manaPCGen(k, sl)
	}
	s.removeSrc(k, av, sl, pc)
}

func (s *State) applyBuild(k *prep, a Action) {
	if s.bld().bfl&bflWinTap != 0 {
		s.applyCastWinTap(k, a)
		return
	}
	if k.gen != nil && s.g.mt.active && s.bld().stage == bsTargets {
		if s.g.mt.slot == mtCharm {
			s.applyCharmTarget(k, a)
			s.syncBuild()
			return
		}
		s.applyMultiTarget(k, a)
		s.syncBuild()
		return
	}
	e := &s.bld().e
	switch a.Kind() {
	case AkChoose:
		if s.bld().stage == bsMiracle {
			s.answerMiracleReveal(k, a)
			return
		}
		if s.bld().stage == bsGift {
			s.answerGift(k, a)
			break
		}
		if s.bld().stage == bsCasualty {
			s.answerCasualty(k, a)
			break
		}
		if s.bld().stage == bsConspire {
			s.answerConspire(k, a)
			return
		}
		if s.bld().stage == bsEvidence {
			s.answerCostEvidence(k, a)
			break
		}
		if s.bld().stage == bsAltAdd {
			part := a.Target()
			s.bld().altAdd = part
			s.nextStage(k)
			if !s.altAddPayable(k, part) && !s.altAddOpenTargets(k) && !s.altAddPipsFirst(k) {
				if k.t.Cards[e.card].BeholdN != 0 && !s.requiredTarget(k) {
					// The seat chose an unpayable part of a behold-or-mana
					// cost: gorge pays through its fallback CR 601.2g
					// window first, which can pose a tap-another pick, and
					// reverses the cast when the window cannot finish it
					// (CR 733.1). manaStep runs that window, so this
					// choice only defers to it.
					s.bld().altAbort = true
					return
				}
				// gorge offers an unpayable part too (Config.GorgeAltAdd);
				// choosing it undoes the cast and returns to priority;
				// gorge reverses the cast (CR 733.1), the card moving back
				// from the stack to the end of its owner's hand
				s.altAddWindowTap(k)
				s.reverseCast(k)
			}
			return
		}
		if s.bld().stage == bsDivide {
			s.answerCastDivide(k, a.Source())
			s.syncBuild()
			return
		}
		if s.bld().stage == bsCosts && s.bld().pickK == pickSubCtr {
			s.recordSubCtr(Obj(a.Source()), a.Mode())
			return
		}
		if s.bld().stage == bsCosts && s.bld().pickK&pkExile != 0 {
			s.answerCostObj(a)
			return
		}
		if s.bld().stage == bsDelve {
			s.answerDelvePick(a)
			return
		}
		if s.bld().stage == bsConvoke {
			s.bld().pickK = convokePickMask | pickKind(a.Mode()&7)
			s.bld().pickLeft = 1
			s.recordPick(Obj(a.Source()))
			s.bld().pickK = 0
			s.convokeDebugTapAtPick(Obj(a.Source()))
			return
		}
		if s.bld().stage == bsCosts || s.bld().stage == bsTapCost || s.bld().stage == bsPush { // bsPush: Station's pick
			s.recordPick(Obj(a.Source()))
			return
		}
		if s.bld().stage == bsPips {
			if s.buildIsTb(k) {
				s.answerPipTb(a.Mode())
				return
			}
			s.bld().setPipCol(s.bld().pipN, a.Mode())
			s.bld().pipN++
			return
		}
		e.x = a.Mode() // X
		s.nextStage(k)
	case AkDone:
		if s.bld().stage == bsModes {
			s.bld().modesN = 255 // a multi-pick charm closed with its minimum met
			if k.gen != nil && k.gen.anyDilemma {
				s.buildModesChosen(k, s.buildAbility(k)) // trig_dilemma.go
			}
			s.nextStage(k)
			s.syncBuild()
			return
		}
		if s.bld().stage == bsConvoke {
			s.bld().pickK = 0
			s.nextStage(k)
			return
		}
		if s.bld().stage == bsDelve {
			s.endDelveStage(k)
			return
		}
		if s.bld().stage == bsEvidence {
			s.answerCostEvidence(k, a)
			s.syncBuild()
			return
		}
		if s.bld().stage == bsCasualty {
			s.teamworkFinish() // the Teamwork election closed (kw_teamwork.go)
			return
		}
		if s.bld().stage != bsTapCost {
			panic("kernel: bad build action")
		}
		if s.bld().pickK == pkWaterbend {
			s.waterbendDone() // a Waterbend pick closed (act_waterbend.go)
			return
		}
		s.tapPowerDone() // a CostTapPower pick closed (kw_crewsaddle.go)
	case AkMode:
		if ab := s.buildAbility(k); ab != nil && ab.Repeats() {
			e.mode = ab.ModeAdd(e.mode, int(a.Mode())) // a count per mode (kop/repeat.go)
		} else {
			e.mode |= 1 << a.Mode()
		}
		s.bld().modesN++
		if k.gen != nil && k.gen.anyCharmPick {
			if e.kind != seAbility { // an activation can still be reversed: recorded at payment
				s.charmPickRecord(k, a.Mode()) // modal_charmpick.go
			}
		}
	case AkTarget:
		ti := s.bld().ti
		e.tgt[ti] = a.Target()
		if a.Target() != NoTarget {
			if isObjTarget(a.Target()) {
				e.tver[ti] = s.objs[a.Target()].ver
			} else if isStackTarget(a.Target()) {
				s.stackTgtPicked(e, ti, a.Target()) // its stack identity (tgt_stackability.go)
			}
			if ti+1 > e.nt {
				e.nt = ti + 1
			}
			if k.gen != nil && k.gen.crimeOn {
				// the plain target step takes one pick per ask: the ask
				// completes here (act_commitcrime.go crimePick)
				s.crimePick(k, a.Target(), true)
			}
		}
		s.bld().ti++
	default:
		panic("kernel: bad build action")
	}
	s.syncBuild()
}

// payCastParts pays a spell's costs other than its mana: the planned sources when a
// discard comes first, the cost picks, life.
func (s *State) payCastParts(k *prep, ep *stackEntry, anyColor bool) {
	e := *ep
	if s.discardPicked() && s.bld().bfl&bflLowered == 0 {
		// gorge's planner picks the sources at priority, before the
		// discard is chosen: the cards discarded for the cost still
		// count toward the hand demand (HandReserve, rank key 6). A
		// lowered cast has no plan: its window pays (finishBuild)
		cost := s.buildCost(k)
		if e.flags&sfAnyMana != 0 || anyColor {
			s.tapRiderPlan(k, e)
		}
		if s.rbAny(e.ctrl) {
			// restricted mana this spell may spend pays first (payRestrSpend):
			// the sources tap for what it leaves (manaStep tapped them already
			// when the plan ran there; this keeps the retap a no-op)
			cost = s.restrShort(k, e.ctrl, e.card, restrOriginOfEntry(ep), cost)
		}
		s.tapForRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
	}
	s.payPicks(k)
	s.payBlightPicks(k)
	if e.flags&sfKicked != 0 && k.gen != nil && k.gen.anyOptCost {
		s.payOptEvidence(k) // a paid OptionalCost's evidence part (cost_optionalevidence.go)
	}
	if e.kind == seSpell && k.gen != nil && k.gen.anyEvTargets {
		s.payTargetsEvidence(k) // a spell's mandatory evidence part (cost_collectevidencex.go)
	}
	if pc := s.buildPickCost(k); pc != nil && pc.Flags&kop.CostPayLife != 0 {
		s.payLifeCost(k, e.ctrl, pc, e.x)
	}
	if k.gen != nil && k.gen.anyRaiseLife {
		s.payRaiseLife(k) // a RaiseCost PayLife<N> the chosen target owes (cost_raiselife.go)
	}
	if s.bld().payLifeMV {
		s.payMayPlayLife(k) // a may-play cast's alternative cost: life equal to its mana value (cast_mayplaypaylife.go)
	}
	if k.phyMana {
		s.payPhyrexianLife(k) // the two-life faces (mana_phyrexian.go)
	}
}

// finishBuild pays the entry (bsPay): the cost picks, life, then the mana from the pool.
func (s *State) finishBuild(k *prep) {
	e := s.bld().e
	anyColor := s.bld().bfl&bflAnyColor != 0
	if e.kind == seAbility && e.mode != 0 && k.gen != nil && k.gen.anyCharmPick {
		s.charmPickRecord(k, uint8(bits.TrailingZeros8(uint8(e.mode))))
	}
	switch e.kind {
	case seSpell:
		if s.bld().pipShort {
			s.reverseCast(k) // the pip window ran dry: nothing of the cost is paid (loweredPipWindow)
			return
		}
		if k.t.Cards[e.card].HasModeCost && !s.spreeAffordable(k) {
			s.spreeCastWindowAbort(k) // kw_spree.go
			return
		}
		// a window that went on after its tap-another pick (bflPayDone) found the cost's
		// other payments made
		payDone := s.bld().bfl&bflPayDone != 0
		_, wbSpell := s.waterbendPartFor(k, &e)
		if wbSpell && s.waterbendTargetAbort(k) {
			if s.waterbendPickCount() > 0 {
				// the picks stranded a cost a complete census prices
				// (waterbendTargetAbort's probe): gorge reverses the cast
				// before any mana ability (reverseStrandedPayment, CR 733.1)
				s.reverseCast(k)
				return
			}
			s.spreeCastWindowAbort(k) // no pick committed: the window taps as it walks, then aborts (CR 733.1)
			return
		}
		if !payDone {
			s.payCastParts(k, &e, anyColor)
		}
		cost := s.buildCost(k)
		if (e.flags&sfAnyMana != 0 || anyColor) && s.bld().bfl&bflLowered == 0 && !payDone {
			s.tapRiderPlan(k, e) // (a lowered cast has no plan: its window below)
		}
		obsPool, obsOK := s.obsWinPool(e.ctrl)
		if e.flags&sfFree != 0 && cost != (kop.ManaCost{}) || e.trigPl == trigPlMadness || e.trigPl == trigPlMiracle {
			// a free cast mid-resolution (Etali) pays the rest of its
			// cost (Eaten Alive's {3}{B} part) through gorge's mana
			// window, not the priority planner (pair 9954); a cost the
			// window cannot finish (Reenact the Crime's copy of Officious
			// Interrogation, whose per-target RaiseCost the pool lacks)
			// reverses the cast (CR 733.1)
			if !s.payCastWindow(k, e.ctrl, &cost, e.flags&sfAnyMana != 0, s.bld().bfl&bflAnyColor != 0) {
				return
			}
		} else if k.gen != nil && k.gen.anyConvoke && s.tapPaysEntry(k, &e) {
			if !s.payConvoke(k, &e, &cost, e.flags&sfAnyMana != 0, anyColor) || s.winPaused() {
				return
			}
			if k.gen.anyCastProv {
				s.bld().e.prov |= uint8(min(s.convokePickCount(), 31)) << 3 // Convoked$Amount (trig_gravedrawn.go)
			}
		} else if s.bld().bfl&bflLowered != 0 && !s.loweredPoolPays(k, &e, xFolded(cost, int(e.x)), anyColor) {
			// a lowered cast pays what its witness left unpaid (Eaten
			// Alive's {3}{B} part, chosen after it; an announced X past the
			// witness) through gorge's mana window: no payment plan exists
			// (pair 4512); the window offers only the sources still useful
			// (pairs 10163 20476)
			xc := xFolded(cost, int(e.x))
			if !s.loweredWindowOK(k, e.ctrl, &xc, e.flags&sfAnyMana != 0, s.bld().bfl&bflAnyColor != 0, cost.X != 0) || s.winPaused() {
				return // stranded: the cast was reversed; or the window waits on a tap-another pick
			}
		} else if (k.t.Cards[e.card].HasModeCost || wbSpell) && cost.X == 0 {
			// gorge's planner declines a Spree face ("shape:modal_cost"):
			// it pays through the mana window (kw_spree.go)
			if !s.payCastWindow(k, e.ctrl, &cost, e.flags&sfAnyMana != 0, s.bld().bfl&bflAnyColor != 0) {
				return
			}
		} else if k.gen != nil && s.delvePaysEntry(k, &e) {
			// gorge's planner declines a delve cast ("shape:contribution",
			// payment_plan.go), so its CR 601.2g window ran at manaStep
			// (mana_window.go branch 3) and what is left pays from the pool;
			// a pool that cannot finishes the cast reversed (CR 733.1). The
			// abort reverses the push, not the window's mana abilities: the
			// tapped sources and the floated mana stay (cast_commit.go
			// abortCast; reverseCast touches neither).
			xc := xFolded(cost, int(e.x))
			if !s.windowSpellSettled(k, e.ctrl, &xc, anyColor) {
				s.reverseCast(k)
				return
			}
			// payFromPool folds the announced X itself (XGeneric), like
			// payRider's pool arm: the folded copy above is only the
			// window's settled check
			s.payFromPoolRider(k, e.ctrl, &cost, int(e.x), anyColor)
		} else if s.rbAny(e.ctrl) {
			s.payRestrRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor) // an Angel spell spends Giada's {W}
		} else if k.gen != nil && s.delvePaysEntry(k, &e) && !s.loweredPoolPays(k, &e, xFolded(cost, int(e.x)), anyColor) {
			// a granted-delve cast's payment ran through the manual CR 601.2g
			// window in manaStep; when even that leaves the cost unpaid the
			// cast is reversed (gorge abortCast, CR 733.1). Declining the
			// delve picks is legal (CR 702.66a "If you do"), and the window's
			// sources may run out afterwards. (A lowered or restricted payment
			// has its own arm above and keeps it.)
			s.reverseCast(k)
			return
		} else {
			s.payRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
		}
		if s.winPaused() {
			return // a free or Spree cast's window waits on a tap-another pick (mana_window.go)
		}
		s.obsWinSettle(k, e.ctrl, obsPool, obsOK)
		s.payDelvePicks() // gorge payCast exiles the delved cards after the mana
		s.paySacsGen(k)
		s.bld().e.flags &^= sfFbGranted
		if wbSpell && s.bld().bfl&bflPushed != 0 {
			s.payWaterbend(k) // the announced taps, after the mana (act_waterbend.go)
		}
		s.syncBuild()
		if e.mode&modeBestowed != 0 && e.bestowedEntry() {
			s.bestowPaid(&e) // gorge's pay-time FlagBestowed (kw_bestow.go)
		}
		// a free cast with ReplaceGraveyard$ Exile carries bit 4
		// as sfFreeGY once the build bit is gone (a non-copy spell entry;
		// resolveTop tests it only on a copy)
		if s.bld().e.flags&sfFree != 0 && s.g.freeP&16 != 0 && s.bld().bfl&bflPushed != 0 {
			s.stack[s.sp-1].flags |= sfFreeGY
		}
		// ForgetPlayed$ True: this free cast now sits on the stack
		// (CR 601.2a), the point where gorge's PlayDone pass sees it and
		// drops it from the remembered set. A cast refused before the push,
		// or reversed while its cost was being paid (CR 733.1), never
		// reaches this commit and keeps its card remembered.
		if s.bld().e.flags&sfFree != 0 && s.g.freeP&128 != 0 && s.bld().bfl&bflPushed != 0 {
			s.forgetRemGen(e.obj)
		}
		if k.gen != nil && k.gen.anyHist {
			s.histCast(k, &e, &cost)
		}
		if k.gen != nil && k.gen.anyFirstCast {
			s.noteFirstCast() // cast_firstcast.go
		}
		if k.gen != nil && k.gen.anyManaHaste {
			s.manaHasteRider(k) // mana_spentrider.go
		}
		s.onCast(k, &e)
		s.conspireTrigger(k, &e)
		s.casualtyTrigger(k, &e)
		s.passes = 0
		if s.bld().bfl&bflDefeatCast != 0 {
			if s.bld().e.flags&sfDefeatPass != 0 {
				// CR 310.11: the owner cast while the other player held
				// priority (the owner had already passed), so that holder's
				// next pass resolves. When the owner held priority, the cast
				// resets the count and both must pass again.
				s.passes = 1
			}
			s.bld().bfl &^= bflDefeatCast
			s.bld().e.flags &^= sfDefeatPass
			s.prio = s.g.holdPrio
		}
		s.ctl = ctlPriority
	case seAbility:
		// gorge composes the mana cost before any cost is paid: a sacrificed
		// source (a Clue's {2}, sacrifice) must still read its reduction
		var pre kop.ManaCost
		early := k.gen != nil && k.gen.anyAbReduce
		if early {
			pre = s.buildCostCharge(k)
		}
		s.payAbilityCosts(k, &e)
		if k.phyMana {
			s.payPhyrexianLife(k) // an activation's two-life faces
		}
		cost := pre
		if !early {
			cost = s.buildCostCharge(k)
		}
		cost = s.waterbendXCharge(k, &e, cost) // a Waterbend<X> pick pays one generic of the folded X (act_waterbend.go)
		if m := s.restrActNow(k, e.ctrl); m != 0 {
			s.payRestrSpend(k, e.ctrl, m, &cost, int(e.x), NoObj, false) // an ability its restriction admits spends it first
		} else {
			s.pay(k, e.ctrl, &cost, int(e.x), NoObj)
		}
		if k.gen != nil && k.gen.anyWaterbend && s.bld().bfl&bflPushed != 0 { // a failed waterbend payment reversed the activation
			s.payWaterbend(k)
		}
		s.paySacsGen(k)
		s.syncBuild()
		if k.gen != nil && k.gen.anyAbilityCast && s.bld().bfl&bflPushed != 0 {
			s.fireAbilityCast(k, &e) // trig_abilitycast.go
		}
		s.passes = 0
		s.ctl = ctlPriority
	case seTrigger:
		s.syncBuild()
		s.ctl = ctlPlaceTriggers
	}
	if k.gen != nil && k.gen.anyBattle && s.g.nmay > 0 {
		// gorge's step() head runs startDefeatedCast after any cast or
		// activation: ask a battle defeated earlier this step first.
		s.wakeDefeat()
	}
	if s.parkedWaiting() {
		s.ctl = ctlExileOrder // a cost's parked move (repl_exilemark.go)
	}
}

func (s *State) push(e stackEntry) {
	if int(s.sp) >= MaxStack {
		panic("kernel: stack full")
	}
	s.stack[s.sp] = e
	s.sp++
}

// abProwess marks a kernel-native prowess trigger (no CardDef ability).
const abProwess uint8 = 254

// onCast fires cast triggers and prowess in one walk over the listeners
// in gorge's battlefield order (player 0's list, then player 1's): per
// permanent its SpellCast triggers (own, then granted), then one
// kernel-native prowess per granting static (gorge checks the printed
// face's triggers, then one granted keyword trigger per granting static).
func (s *State) onCast(k *prep, e *stackEntry) {
	if s.cfg&cfgCommander != 0 && s.bld().origin == kop.ZoneCommand {
		// CR 903.8 counts a cast as it is committed to the stack; a
		// reversed cast never reaches here
		if j := s.cmdSlot(e.ctrl, e.obj); j >= 0 && *s.cmdCastAt(e.ctrl, j) < 255 {
			*s.cmdCastAt(e.ctrl, j)++
		}
	}
	pc := &k.cards[e.card]
	if k.gen != nil && pc.types&(kop.TInstant|kop.TSorcery) != 0 && s.g.isCast[e.ctrl] < 255 {
		s.g.isCast[e.ctrl]++
	}
	var pm slotMask
	if !pc.creature && pc.types&kop.TLand == 0 && !e.faceDown() { // a face-down spell is a creature spell (CR 708.4)
		for m := s.ctrlM[e.ctrl].and(s.occ); m.any(); m = m.rest() {
			sl := m.first()
			// a printed prowess is the card's own SpellCast trigger; the
			// kernel-native trigger covers granted prowess only
			if k.gen != nil && k.gen.anyProwessGrant {
				if s.prowessGrantCount(k, sl) > 0 {
					pm = pm.with(sl)
				}
			} else if s.kw(k, sl)&kop.KwProwess != 0 && k.cards[s.cardAt(sl)].kw&kop.KwProwess == 0 {
				pm = pm.with(sl)
			}
		}
	}
	lm := s.subs[kop.TrigSpellCast].and(s.occ)
	if k.gen != nil && k.gen.grantTrigEv&(1<<kop.TrigSpellCast) != 0 {
		lm = s.grantTrigHosts(k, lm)
	}
	if e.ctrl == 1 && !e.faceDown() {
		// the spell's own cast trigger: gorge walks the stack after seat
		// 0's battlefield and before seat 1's (trig_selfcast.go)
		s.castFromStack(k, e)
	}
	if lm.or(pm).any() {
		var ord [MaxPerm]Slot
		n := s.listenerOrder(lm.or(pm), &ord)
		for _, sl := range ord[:n] {
			if lm.has(sl) {
				if k.gen != nil {
					// a self cast trigger (TfSelf) reads the spell cast, never
					// the permanent carrying it: skipped here, fired from the
					// stack by fireSelfCast
					// a deferred source's TriggersWhenSpent$ rider queues after the walk (fireDeferredRider)
					s.slotTriggers(k, sl, func(a *kop.Ability) bool {
						return a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags&kop.TfSelf == 0 && !s.deferRider(k, sl, a)
					}, func(l trigListener, a *kop.Ability) {
						s.castTrigGen(k, l, a, e)
					})
				} else {
					s.castTrigVanilla(k, sl, e)
				}
			}
			if pm.has(sl) {
				p := &s.perms[sl]
				n := 1
				if k.gen != nil && k.gen.anyProwessGrant {
					n = max(s.prowessGrantCount(k, sl), 1)
				}
				for i := 0; i < n; i++ {
					np := s.npend
					s.enqueue(p.obj, s.objs[p.obj].ver, s.cardAt(sl), abProwess, p.ctrl, NoObj, e.ctrl, 0)
					s.panharmKeyword(k, sl, kop.TrigSpellCast, NoObj, np)
				}
			}
		}
	}
	if e.ctrl == 0 && !e.faceDown() {
		s.castFromStack(k, e)
	}
	if k.anyEmbFlash {
		s.embFlashForget(k, e)
	}
	if k.gen != nil {
		s.castTrigGraves(k, e)
		s.eachCommandTrigger(k, kop.TrigSpellCast, func(l trigListener, a *kop.Ability) {
			s.castTrigGen(k, l, a, e)
		})
		s.fireEmblems(k, kop.TrigSpellCast, e.ctrl, e.obj, func(a *kop.Ability) bool {
			return s.matchCardGen(k, a.Trig.Filter, e.card, e.ctrl, e.ctrl, e.obj)
		})
	}
	if k.gen != nil && k.gen.anyDelaySpell && s.g.ndp != 0 {
		// gorge checkEventDelayedTriggers: after the face triggers, before
		// the cascade triggers (trig_delayedspell.go)
		s.fireDelayedSpell(k, e)
	}
	if k.gen != nil && k.gen.anyDeferred {
		s.fireDeferredRider(k, e) // gorge fireManaSpentTriggers: after the cast triggers, before cascade
	}
	if k.gen != nil && k.gen.anyCascade && !DebugIgnoreCascade {
		s.queueCascade(k, e)
	}
}

// castTrigGraves queues the SpellCast triggers that work from a graveyard
// (TriggerZones$ Graveyard, Shambling Cie'th; CR 113.6k), after the battlefield's.
func (s *State) castTrigGraves(k *prep, e *stackEntry) {
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				o := pl.grave[i]
				if s.objs[o].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind == kop.AbTriggered && a.Trig.Zone == kop.ZoneGraveyard && a.Trig.Event == kop.TrigSpellCast {
						s.castTrigGen(k, trigListener{obj: o, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}, a, e)
					}
				}
			}
		}
	}
}

// castTrigVanilla queues the SpellCast triggers of the listener at ls (a
// hand-written table) that spell e matches.
func (s *State) castTrigVanilla(k *prep, ls Slot, e *stackEntry) {
	lp := &s.perms[ls]
	cd := &k.t.Cards[s.cardAt(ls)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigSpellCast || a.Trig.Flags&kop.TfSelf != 0 {
			continue // a self cast trigger fires from the stack (fireSelfCast)
		}
		if !whoseOK(a.Trig.Whose, lp.ctrl, e.ctrl) || !s.matchCardAt(k, a.Trig.Filter, e.card, e.ctrl, lp.ctrl, e.obj) {
			continue
		}
		s.enqueue(lp.obj, s.objs[lp.obj].ver, s.cardAt(ls), uint8(i), lp.ctrl, e.obj, e.ctrl, 0)
	}
}

// targetedOne is one target code t of entry e chosen: its BecomesTarget
// triggers, then the ward of a permanent its controller does not control.
func (s *State) targetedOne(k *prep, e *stackEntry, t uint8) {
	if !isObjTarget(t) {
		return // a player, or a stack ability (tgt_stackability.go)
	}
	ob := &s.objs[t]
	if ob.zone != kop.ZoneBattlefield {
		if ob.zone == kop.ZoneStack && k.gen != nil && k.gen.btSpell && s.subs[kop.TrigBecomesTarget].any() {
			s.btSpellTarget(k, Obj(t), e) // trig_becomestarget.go
		}
		return
	}
	sl := ob.slot
	p := &s.perms[sl]
	if s.subs[kop.TrigBecomesTarget].any() {
		s.becameTarget(k, sl, e)
	}
	fd := s.faceDown(k, sl)
	disguise := fd && s.faceDownWard(k, sl) // never a manifested permanent (CR 701.40a)
	// The ward must survive the layer-6 walk: a face-down Disguise
	// permanent's ward is a base keyword, so "loses all abilities"
	// removes it (CR 702.168b, 613.1f).
	if p.ctrl == e.ctrl || s.kw(k, sl)&kop.KwWard == 0 {
		return
	}
	if k.gen != nil && k.gen.anyWardGrant {
		s.wardGranted(k, sl, ob, e, disguise) // the printed ward and each granted one (kw_ward.go)
		return
	}
	amt := wardAmt(&k.t.Cards[s.cardAt(sl)])
	if disguise {
		amt = 2 // CR 702.168b: a face-down Disguise permanent has ward {2}
	}
	np := s.npend
	s.enqueueWard(e, p.obj, ob.ver, s.cardAt(sl), p.ctrl, e.ctrl, amt)
	s.panharmKeyword(k, sl, kop.TrigBecomesTarget, e.obj, np)
}

// enqueueWard queues the ward trigger e's targeting of a permanent answers,
// carrying e's stack identity so the trigger answers that ability and not the
// topmost one of the same source at placement: a sibling ability of the same
// source can be pending in one batch (a doubler's copy), so the stack is not
// authoritative then (CR 603.3b, 701.6a). A spell has no identity stamp: its
// trigObj names it directly (wardedIndex).
func (s *State) enqueueWard(e *stackEntry, o Obj, ver uint8, c CardID, ctrl, tpl uint8, amt int16) {
	np := s.npend
	s.enqueue(o, ver, c, abWard, ctrl, e.obj, tpl, amt)
	if s.npend > np && e.kind != seSpell {
		s.pend[np].wardSid = e.sid
	}
}

// checkWard: a spell or ability an opponent controls that targets a
// permanent with ward triggers the ward (kernel-native trigger, abWard).
func (s *State) checkWard(k *prep, e *stackEntry) {
	var cs crimeSt
	crimeOn := k.gen != nil && k.gen.crimeOn
	if crimeOn {
		cs.n0 = int(s.npend)
	}
	for i := 0; i < int(e.nt); i++ {
		s.targetedOne(k, e, e.tgt[i])
		if crimeOn {
			s.crimeStep(k, e, &cs, e.tgt[i], int8(i), -1)
		}
		if k.gen != nil && e.xi != 0 {
			// a multi-object slot's other objects (Run Away Together)
			x := s.xtOf(e)
			for j := 0; j < int(x.n); j++ {
				if x.slot[j] == uint8(i) {
					s.targetedOne(k, e, x.code[j])
					if crimeOn {
						s.crimeStep(k, e, &cs, x.code[j], -1, int8(j))
					}
				}
			}
		}
	}
	if crimeOn {
		s.crimeEnd(k, e, &cs)
	}
}

const abWard uint8 = 253

// abCascade marks a kernel-native cascade trigger (CR 702.85a; no CardDef
// ability): its source is the cascade spell, trigAmt its mana value. Every
// ab at or above it is kernel-native (no Abilities entry to index). It was
// 249 until abMiracle (kw_miracle.go) took that value to stay above it.
const abCascade uint8 = 248

// abStorm marks a kernel-native storm trigger (CR 702.40a; no CardDef
// ability): its source is the spell, trigObj the same spell, trigAmt the
// number of spells cast before it this turn (kw_storm.go).
const abStorm uint8 = 250

// abDelayed marks a delayed triggered ability (OpDelayedTrigger).
const abDelayed uint8 = 252

// --- putting pending triggers on the stack (APNAP; gorge trigger_queue.go) ---

// placeTriggers puts pending triggers on the stack: the active player's
// first (they resolve last). With two or more pending for one player a
// TriggerOrder decision picks which goes on the stack next.
func (s *State) placeTriggers(k *prep, d *Decision) bool {
	if s.orderPlace > 0 {
		// an answered ordering round: push its triggers in pick order (each
		// then asks its targets), as gorge pushes the ordered group
		idx := s.ordList[0]
		copy(s.ordList[:], s.ordList[1:s.orderPlace])
		s.orderPlace--
		for i := 0; i < int(s.orderPlace); i++ {
			if s.ordList[i] > idx {
				s.ordList[i]--
			}
		}
		s.takePending(k, int(idx))
		return false
	}
	if s.npend == 0 {
		// placing triggers keeps the priority holder and the pass count
		// (gorge grantPriority emits Priority{holder, Passes})
		s.ctl = ctlPriority
		return false
	}
	if k.gen != nil && k.gen.anyStaticTrig && s.placeStaticTriggers(k, d) {
		// a Static$ True trigger resolved off the stack, before the
		// CR 603.3b ordering ask (gorge resolveStaticTriggers); the loop
		// re-enters to scan the queue again (trig_mayplaystatic.go)
		return false
	}
	p := s.active
	n := s.pendCount(p)
	if n == 0 {
		// CR 603.3b: APNAP order, each player in turn order from the
		// active player (gorge sortPendingTriggers over AliveFrom(Active))
		for {
			p = s.nextAlive(p)
			if n = s.pendCount(p); n != 0 || MaxSeats == 2 {
				break
			}
		}
	}
	if s.orderLeft == 0 && n >= 2 {
		// gorge KTriggerOrder is one Min=Max=n multi-select over the
		// group: every one of its n picks is posed, the last one too, and
		// the group is pushed only once the order is complete
		s.orderLeft = uint8(n)
		s.orderN = 0
	}
	if s.orderLeft > 0 {
		d.Kind, d.Player = DkTriggerOrder, p
		for i := 0; i < int(s.npend); i++ {
			if pt := &s.pend[i]; pt.ctrl == p && !s.ordered(uint8(i)) {
				d.add(MakeAction(AkOrder, uint8(pt.obj), uint8(i), s.occurrence(i)))
			}
		}
		return true
	}
	for i := 0; i < int(s.npend); i++ {
		if s.pend[i].ctrl == p {
			s.takePending(k, i)
			return false
		}
	}
	panic("kernel: pending trigger not found")
}

func (s *State) ordered(i uint8) bool {
	for j := 0; j < int(s.orderN); j++ {
		if s.ordList[j] == i {
			return true
		}
	}
	return false
}

func (s *State) applyTriggerOrder(k *prep, a Action) {
	s.ordList[s.orderN] = a.Target()
	s.orderN++
	s.orderLeft--
	if s.orderLeft == 0 {
		s.orderPlace = s.orderN
		s.orderN = 0
	}
}

// takePending removes pending trigger i and starts building it.
func (s *State) takePending(k *prep, i int) {
	pt := s.pend[i]
	copy(s.pend[i:s.npend-1], s.pend[i+1:s.npend])
	s.npend--
	// the trigger's push is progress: gorge's TriggerPush event clears the
	// no-progress abort counts (emit's suppression rule), so a cast reversed
	// before it may be redone after it (CR 733.2; noProgress)
	s.touchN++
	// A stale warp / unearth / destroy / exile promise, or an incarnation-tracked
	// one (Decayed), still triggers and is put on the stack (CR 603.7); it
	// resolves doing nothing to the new object (CR 400.7: resolveDelayed and
	// resolveTrig check the version), as gorge's resolveTop does.
	*s.bld() = build{e: stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card, ab: pt.ab, ctrl: pt.ctrl,
		trigObj: pt.trigObj, trigPl: pt.trigPl, trigAmt: pt.trigAmt, li: pt.li, remVer: pt.remVer, remSet: pt.skw&skwRem != 0, x: pt.x, trigMV: pt.trigMV, grp: pt.grp, skw: pt.skw &^ (skwRem | skwMany), defSeat: pt.defSeat}}
	if pt.skw&skwMany != 0 {
		s.dpManyVers(&s.bld().e)
	}
	s.bld().e.tgt = noTargets
	if pt.ab == abWard && pt.wardSid != 0 {
		// the ability this ward trigger answers, by the identity captured when
		// checkWard queued it: a sibling ward may counter it first, and a newer
		// ability of the same source is not the one it targets (CR 603.3b, 701.6a)
		s.bld().e.tver[0] = pt.wardSid
	}
	s.bld().stage = stageFirst[seTrigger]
	if pt.ab == abMiracle {
		s.bld().stage = bsMiracle // asked before it is put on the stack (kw_miracle.go)
		s.ctl = ctlBuild
		return
	}
	if k.gift {
		s.giftPushed(k, &pt)
	}
	s.pushBuild(k)
	if pt.ab >= abCascade {
		return
	}
	s.ctl = ctlBuild
}

// pickCosts are the cost flags paid by choosing objects.
const pickCosts = kop.CostDiscard | kop.CostSac | kop.CostTapX

// costPickOptions poses the next pick for the pick costs in allowed of the
// entry being built (a spell's additional costs live on its spell
// ability). gorge's order: the tap-creatures cost before X (continueCast's
// tapPermanentCostAsk), then at payment, after the mana abilities
// (costObjectAsks, CR 601.2h), sacrifice, then discard; each is one multi-select
// there and one pick per object here. A tap cost with exactly N candidates
// is paid without asking (gorge cast_asks.go:488); sacrifice and discard
// are asked even then. The picks are paid with the rest of the cost.
func (s *State) costPickOptions(k *prep, d *Decision, allowed kop.CostFlags) bool {
	e := &s.bld().e
	if e.kind == seTrigger || s.bld().pickK == pkWaterbend {
		return false
	}
	pc := s.buildPickCost(k)
	if pc == nil {
		return false
	}
	src := s.srcSlot()
	for {
		if s.bld().pickK == 0 {
			rest := pickKind(pc.Flags&allowed) &^ s.bld().donePicks
			if rest == 0 {
				return false
			}
			s.bld().pickK = rest & -rest
			s.bld().pickLeft = pc.N
			if s.bld().pickK == pkSac && pc.N == 0 {
				s.bld().pickLeft = e.x // Sac<X/Spec>: the announced X (cost_sacx.go; 0 asks nothing)
			}
		}
		if s.bld().pickLeft == 0 {
			s.bld().donePicks |= s.bld().pickK &^ pickSubCtr // a posed counter pick stepped again owes no done bit
			s.bld().pickK = 0
			continue
		}
		d.N = 0
		s.costCands(k, pc, src, d)
		if s.bld().pickK == pkSac && int(d.N) < int(s.bld().pickLeft) || s.bld().pickK == pkDiscard && d.N == 0 ||
			s.bld().pickK == pkTapX && e.kind == seSpell && int(d.N) < int(s.bld().pickLeft) {
			// no card left to discard, or too few permanents to tap: a free cast begun without
			// the offer's cost census (Wishing Well's Play, a discover's Guardian of the Great
			// Door) reverses here, as gorge's discardAsk and tapPermanentCostAsk abort (CR 733.1)
			s.sacShort(k, d)
			return true
		}
		if s.bld().pickK == pkTapX && int(d.N) == int(s.bld().pickLeft) {
			for i := 0; i < int(d.N); i++ {
				s.recordPick(Obj(d.Options[i].Source()))
			}
			d.N = 0
			continue
		}
		break
	}
	d.Kind, d.Player = DkChoose, e.ctrl
	switch s.bld().pickK {
	case pkDiscard:
		d.Sub = ChCostDiscard
	case pkSac:
		d.Sub = ChCostSacrifice
		if pc.ExileBf {
			d.Sub = ChCostExile // Exile<N/Spec> from the battlefield: gorge's exile ask (cost_exilebf.go)
		}
	default:
		d.Sub = ChCostTap
	}
	return true
}

// costCands lists the candidates for the current pick kind.
func (s *State) costCands(k *prep, c *kop.Cost, src Slot, d *Decision) {
	e := &s.bld().e
	switch s.bld().pickK {
	case pkDiscard:
		pl := &s.pl[e.ctrl]
		// a filtered discard (CostDiscardFiltered) offers only the hand
		// cards matching Cost.Filter (cost_discardfiltered.go)
		fmatch := c.Flags&kop.CostDiscardFiltered != 0
		for i := 0; i < int(pl.handN); i++ {
			if o := pl.hand[i]; !s.picked(o) && o != e.obj &&
				!(fmatch && !s.matchCardGen(k, c.Filter, s.objs[o].card, e.ctrl, e.ctrl, o)) {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	case pkSac, pkTapX:
		for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
			sl := m.first()
			o := s.perms[sl].obj
			if s.picked(o) || !s.matchPerm(k, c.Filter, sl, e.ctrl, src) || s.bld().pickK == pkSac && !c.ExileBf && s.sacBlocked(k, sl) {
				continue
			}
			if s.bld().pickK == pkTapX && (s.perms[sl].flags&pfTapped != 0 || (sl == src && c.Flags&kop.CostTap != 0)) {
				continue
			}
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
}

func (s *State) recordPick(o Obj) {
	s.bld().picks[s.bld().pickN] = o
	s.bld().pickKs[s.bld().pickN] = s.bld().pickK
	s.bld().pickN++
	s.bld().pickLeft--
}

func (s *State) picked(o Obj) bool {
	if !s.buildLive() {
		// arm B's picks are another arm's stale bytes outside a build: an
		// offer-time census (costObjPayable's returnCands) must not read a
		// finished build's picks
		return false
	}
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && s.bld().pickKs[i]&convokePickMask != convokePickMask {
			return true
		}
	}
	return false
}

// payPicks pays the chosen pick costs in gorge's payCast order: discards,
// then taps, then sacrifices (cast_commit.go).
// discardPicked reports whether the entry being built discards cards for
// its cost.
func (s *State) discardPicked() bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkDiscard {
			return true
		}
	}
	return false
}

func (s *State) payPicks(k *prep) {
	if s.bld().pickN == 0 {
		return
	}
	// the counters removed for a cost leave together at payment (gorge
	// settleSubCounterParts), the creatures keeping them through the asks
	for i := 0; i < int(s.bld().pickN); i++ {
		if f := s.bld().pickKs[i]; f&pickSubCtr != 0 {
			if ob := &s.objs[s.bld().picks[i]]; ob.zone == kop.ZoneBattlefield {
				pp := &s.perms[ob.slot]
				s.touchSl(ob.slot)
				pp.seeCtr(s)
				if c := int(uint8(f)); pp.ctrN(s, c) > 0 {
					pp.subCtrN(s, c, 1)
				}
			}
		}
	}
	for _, f := range [3]pickKind{pkDiscard, pkTapX, pkSac} {
		if f == pkTapX && k.costObj {
			s.payExilePicks() // after the discards (gorge payCast)
		}
		// the cost's tap parts are one tapping action (gorge paidcost.go's
		// BatchTap bracket; trig_tapall.go)
		tb := f == pkTapX && s.tapBatchOpen(k)
		for i := 0; i < int(s.bld().pickN); i++ {
			if s.bld().pickKs[i] != f {
				continue
			}
			o := s.bld().picks[i]
			ob := &s.objs[o]
			switch f {
			case pkDiscard:
				if ob.zone == kop.ZoneHand {
					if s.bld().e.kind == seSpell && k.gen != nil && k.gen.anyDiscRef {
						s.bld().e.trigObj = o // RefDiscarded (value_discardedref.go)
					}
					if !s.parkCost(k, o, parkDiscard, 0) {
						s.discardObj(k, o)
					}
				}
			case pkTapX:
				if ob.zone == kop.ZoneBattlefield {
					was := s.perms[ob.slot].flags & pfTapped
					s.perms[ob.slot].flags |= pfTapped
					s.touch(o)
					if was == 0 {
						if k.gen != nil && s.subs[kop.TrigTaps].any() {
							if g := k.gen; g.anyTWTap && s.bld().e.kind == seSpell && s.bld().e.flags&sfKicked != 0 && k.t.Cards[s.bld().e.card].Teamwork != 0 {
								s.fireTapsTW(k, ob.slot) // a teamwork election pick (trig_params.go)
							} else {
								s.fireTaps(k, ob.slot)
							}
						}
						s.markTapped(k, ob.slot) // only a tap that happens marks
					}
					if k.gen != nil && k.gen.crewBy != nil {
						s.noteCrewPick(k, ob.slot) // CrewedThisTurn marks (filter_combatwords.go)
					}
				}
			case pkSac:
				// gorge taps the planned sources before any cost is paid
				// (CR 601.2g-h), so every sacrifice pick waits for the
				// mana (paySacsGen): a sacrificed Elf still counts for
				// Elvish Archdruid's mana (pair 9100)
				if ob.zone == kop.ZoneBattlefield && k.gen == nil {
					if s.pickExiles(k) {
						s.leave(k, ob.slot, kop.ZoneExile, false) // Exile<N/Spec>: no sacrifice (cost_exilebf.go)
						continue
					}
					s.fireSac(k, ob.slot)
					s.leave(k, ob.slot, kop.ZoneGraveyard, false)
				}
			}
		}
		s.tapBatchClose(tb)
	}
}

// occurrence is pending trigger i's index among the pending triggers with
// the same source object (queue order): the disambiguator for two
// simultaneous triggers from one source.
func (s *State) occurrence(i int) uint8 {
	n := uint8(0)
	for j := 0; j < i; j++ {
		if s.pend[j].obj == s.pend[i].obj {
			n++
		}
	}
	return n
}

// NumPending is the number of triggered abilities waiting to be put on the
// stack.
func (s *State) NumPending() int { return int(s.npend) }

// PendingTrigger describes pending trigger i (the index an AkOrder's
// target names): its source object, the source's card, the ability index
// in that CardDef (abProwess / abWard for the kernel-native triggers), its
// controller, and its occurrence index among pending triggers from the same
// source.
func (s *State) PendingTrigger(i int) (src Obj, card CardID, ability uint8, ctrl uint8, occurrence uint8) {
	pt := &s.pend[i]
	return pt.obj, pt.card, pt.ab, pt.ctrl, s.occurrence(i)
}

// PendingOrdered is the pending triggers an answered ordering round has yet to push,
// in pick order, by PendingTrigger index.
func (s *State) PendingOrdered() []uint8 { return s.ordList[:s.orderPlace] }

// spellAddCost is card c's additional cost: its spell ability's cost
// (instants, sorceries, Auras) or CardDef.SpellCost (other permanents).
func spellAddCost(k *prep, c CardID) *kop.Cost {
	if ab := k.cards[c].spellAb; ab >= 0 {
		return &k.t.Cards[c].Abilities[ab].Cost
	}
	return &k.t.Cards[c].SpellCost
}

// buildPickCost is the cost whose pick parts the entry being built pays:
// an ability's cost, or a spell's additional cost (the either-or choice
// applied).
func (s *State) buildPickCost(k *prep) *kop.Cost {
	e := &s.bld().e
	switch e.kind {
	case seSpell:
		cd := &k.t.Cards[e.card]
		if cd.HasAltAdd && s.bld().altAdd == 1 {
			return &cd.AltAddCost
		}
		if s.bld().altAdd == buildAltCost {
			if a := &cd.Abilities[k.gen.altFree[e.card]-1]; altPickCost(&a.Cost) {
				return &a.Cost // the sacrifice / hand-exile alternative cost (Fireblast, Force of Will; cast_altfree.go)
			}
		}
		if s.bld().altAdd == buildBeamGrant {
			g, _ := s.beamGrantor(k, e.ctrl)
			return &k.t.Cards[s.cardAt(g)].FbCost // the return a granted beam me up cast pays (kw_beammeup.go)
		}
		if e.flags&sfFlashback != 0 && cd.FbCost.Flags != 0 && e.flags&sfFbGranted == 0 {
			return &cd.FbCost // a non-mana flashback cost (CR 702.34a; kgen lower_fbcost.go)
		}
		if k.gen != nil && k.gen.anyKickCost {
			if kc := kickerCostPick(k, e); kc != nil {
				return kc // a kicked cast's Sac/Return kicker part (kw_kicker.go)
			}
		}
		if k.gen != nil && k.gen.anyOptCost {
			if oc := optCostPick(k, e); oc != nil {
				return oc // a paid OptionalCost's Blight<N> part (cost_optional.go)
			}
		}
		if k.gen != nil && k.gen.anyMayRaise {
			if rc := s.mayRaisePick(k, e); rc != nil {
				return rc // a may-play cast's non-mana RaiseCost$ (cost_mayplayraise.go)
			}
		}
		return spellAddCost(k, e.card)
	case seAbility:
		if a := s.buildAbility(k); a != nil {
			return &a.Cost
		}
	}
	return nil
}

// pickCostPayable reports whether p can pay cost c's non-mana pick parts
// (and its life) for card or source src (exclude: a hand card not counted).
func (s *State) pickCostPayable(k *prep, p uint8, c *kop.Cost, exclude Obj) bool {
	f := c.Flags
	if f&kop.CostDiscard != 0 {
		// a filtered discard counts only the hand cards matching
		// Cost.Filter, the excluded source not among them
		// (cost_discardfiltered.go)
		n := int(s.pl[p].handN)
		if f&kop.CostDiscardFiltered != 0 {
			n = s.discardCandN(k, p, c.Filter, exclude)
		} else if exclude != NoObj && s.objs[exclude].zone == kop.ZoneHand {
			n--
		}
		if n < int(c.N) {
			return false
		}
	}
	if f&kop.CostSac != 0 && s.countCostSac(k, p, c, NoSlot) < int(c.N) {
		return false
	}
	if f&kop.CostTapX != 0 && s.countUntapped(k, p, c.Filter, NoSlot) < int(c.N) {
		return false
	}
	if f&kop.CostPayLife != 0 && int(s.pl[p].life) < int(c.N) {
		return false
	}
	if f&(costObjFlags|kop.CostPayEnergy) != 0 && !s.costObjPayable(k, p, c, NoObj, exclude) {
		return false
	}
	return true
}

// altAddPayable: part 0 (the spell's own additional cost) or part 1
// (CardDef.AltAddCost, whose mana is added to the spell's) is payable, the
// mana from the pool or from what the window can still activate (gorge
// altAddPartPayable, CR 601.2g).
func (s *State) altAddPayable(k *prep, part uint8) bool {
	if !s.altAddPickPayable(k, part) {
		return false
	}
	if part == 0 {
		return true
	}
	e := &s.bld().e
	var av avail
	s.availForBuilt(k, e, &av)
	if k.tapMana {
		s.availTapX(k, e.ctrl, &av) // the window activates a tap-another source (mana_window.go)
	}
	saved := s.bld().altAdd
	s.bld().altAdd = 1
	c := s.buildCost(k)
	s.bld().altAdd = saved
	// the restricted sources the payment may spend (CR 106.6): gorge's window prices them
	return av.affords(&c, pipMaskOf(&c), 0) || k.restrSrc && s.restrSrcAffords(k, e.ctrl, s.payDescNow(NoObj), &c, &av, NoSlot, 0)
}

// altAddPickPayable: the part's costs that are no mana (a discard, a sacrifice,
// life, a behold) are payable. No mana ability can pay them, so gorge's target
// menu withholds every candidate when they are not (cast_pricing.go
// affordableTargetCandidates, pay.NonManaCastable), whatever the window holds.
func (s *State) altAddPickPayable(k *prep, part uint8) bool {
	e := &s.bld().e
	if part == 0 {
		return s.pickCostPayable(k, e.ctrl, spellAddCost(k, e.card), e.obj) && s.beholdPayable(k, e.ctrl, e.card, e.obj)
	}
	return s.pickCostPayable(k, e.ctrl, &k.t.Cards[e.card].AltAddCost, e.obj)
}

// altAddOptions poses gorge's either-or additional cost ask (cast_asks.go
// altAddAsk, KChoose "altaddcost"): the payable parts first, then the
// unpayable ones, each AkChoose{target: part index (0 the spell's own
// additional cost, 1 CardDef.AltAddCost), mode: option position}.
func (s *State) altAddOptions(k *prep, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChAltAdd, s.bld().e.ctrl
	var ok [2]bool
	ok[0], ok[1] = s.altAddPayable(k, 0), s.altAddPayable(k, 1)
	pos := uint8(0)
	passes := 2
	if s.cfg&cfgGorgeAltAdd == 0 && (ok[0] || ok[1]) {
		passes = 1 // the payable parts only (Config.GorgeAltAdd)
	}
	for pass := 0; pass < passes; pass++ {
		for part := uint8(0); part < 2; part++ {
			if ok[part] == (pass == 0) {
				d.add(MakeAction(AkChoose, 0, part, pos))
				pos++
			}
		}
	}
}

func (s *State) pendCount(p uint8) int {
	n := 0
	for i := 0; i < int(s.npend); i++ {
		if s.pend[i].ctrl == p {
			n++
		}
	}
	return n
}

// noProgress reports whether nothing happened since the last undone cast
// (the same turn and step, no object written).
func (s *State) noProgress() bool {
	return s.heldTurn == uint16(s.turn) && s.heldStep == s.step && s.heldTouch == s.touchN
}

// castHeldOut: gorge holds out a card's cast after its second undo in a
// row with no progress between (cast_commit.go abortCast, CR 733.2: a
// merely reversed action may be redone once), until something happens.
func (s *State) castHeldOut(o Obj) bool {
	return (s.heldN >= 2 && s.heldObj == o || s.objs[o].of&ofHeld != 0) && s.noProgress()
}

// pickKind is a cost pick's kind as build stores it (pickK, pickKs,
// donePicks): the pickable cost flags at their kop bit (all below 1<<14,
// asserted below), so a 2-byte kind keeps State's size although
// kop.CostFlags is 32 bits wide. The cost-pick path converts at its
// boundary (costPickOptions); the bits 14 and 15 are the kernel's own.
type pickKind uint16

const (
	pkSac     = pickKind(kop.CostSac)
	pkDiscard = pickKind(kop.CostDiscard)
	pkTapX    = pickKind(kop.CostTapX)
	pkExile   = pickKind(kop.CostExileGrave)
	// pkForage: donePicks' bit for a posed forage either-or (kop.CostForage
	// is above 16 bits).
	pkForage pickKind = 0x4000
	// pkBlight marks a Blight cost pick (kop.CostBlight is above 16 bits).
	// It carries pkExile's bit so the answer routes to answerCostObj.
	pkBlight pickKind = pickKind(kop.CostAddCtr) | pkExile
	// pkPutLib marks the PutCardToLibFromGrave<1/-1/CARDNAME> self cost
	// pick (kop.Cost.PutLibSelf; a Cost byte, not a flag). It carries
	// pkExile's bit so the answer routes to answerCostObj. It shares bit 13
	// with pkWaterbend: the two cost shapes cannot coexist (PutLibSelf is a
	// graveyard ability's, waterbend is refused outside a battlefield one).
	pkPutLib pickKind = 0x2000 | pkExile
	// pickSubCtr marks a pick (pickKs) that removes one counter, of the
	// kind in its low byte, from the picked creature.
	pickSubCtr pickKind = 0x8000
)

// the pickable flags fit below pkForage
var _ = [1]struct{}{}[(pickCosts|kop.CostExileGrave)>>14]

// subCtrOptions poses the next counter to remove for Quilled Greatwurm's
// "removing six counters from among creatures you control", one counter
// at a time as gorge asks it (rules cast_subcounter.go wildcardCounterAsk):
// every (creature, counter kind) unit with a counter not yet chosen, in
// battlefield order and counter kind order; a lone unit is taken unasked.
func (s *State) subCtrOptions(k *prep, d *Decision) bool {
	if s.bld().subCtr == 0 || s.bld().e.kind != seSpell {
		return false
	}
	e := &s.bld().e
	for {
		n := 0
		for i := 0; i < int(s.bld().pickN); i++ {
			if s.bld().pickKs[i]&pickSubCtr != 0 {
				n++
			}
		}
		if n >= int(s.bld().subCtr) {
			s.bld().subCtr, s.bld().pickK = 0, 0
			return false
		}
		d.N = 0
		for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
			sl := m.first()
			if s.typesGen(k, sl)&kop.TCreature == 0 {
				continue
			}
			o := s.perms[sl].obj
			p := &s.perms[sl]
			for c := 0; c < int(kop.NumCounterKinds3); c++ {
				if !kop.IsCounterKind(c) {
					continue // CtrLevel is no counter (kw_class.go)
				}
				if p.ctrN(s, c) > s.subCtrPicked(o, uint8(c)) {
					d.add(MakeAction(AkChoose, uint8(o), 0, uint8(c)))
				}
			}
		}
		if d.N == 0 {
			// gorge aborts a cast whose counters ran out; the offer
			// (creatureCounters) never lets it start short
			s.bld().subCtr, s.bld().pickK = 0, 0
			return false
		}
		if d.N == 1 {
			s.recordSubCtr(Obj(d.Options[0].Source()), d.Options[0].Mode())
			continue
		}
		s.bld().pickK = pickSubCtr
		d.Kind, d.Sub, d.Player = DkChoose, ChCostSubCtr, e.ctrl
		return true
	}
}

// subCtrPicked is the number of counters of kind c already chosen from o.
func (s *State) subCtrPicked(o Obj, c uint8) int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && s.bld().pickKs[i] == pickSubCtr|pickKind(c) {
			n++
		}
	}
	return n
}

func (s *State) recordSubCtr(o Obj, c uint8) {
	s.bld().picks[s.bld().pickN] = o
	s.bld().pickKs[s.bld().pickN] = pickSubCtr | pickKind(c)
	s.bld().pickN++
}

// poolRPays: p's floating pool, with the restricted mana the cast being built
// may spend, pays its composed cost, so the oracle's lowering of the cast taps
// nothing: a hybrid pip is then paid by whichever face the pool covers
// (a floating {U} and {B} beside an untapped Island pay an evoked Deceit's
// {U/B}{U/B}, CR 601.2h; Flamebraider's {W}{W} floating beside an untapped Mountain pays an evoked
// Catharsis, where a witness over the pool would take the {W}{R} face first
// and tap the Mountain).
func (s *State) poolRPays(k *prep, p uint8, c CardID, o Obj) bool {
	var av avail
	s.availPool(p, &av)
	if s.rbAny(p & 1) {
		s.restrAvail(k, p, c, s.restrOriginOfObj(o), &av)
	}
	if av.total == 0 {
		return false
	}
	cost := s.buildCost(k)
	// an {X} (a warp cost's) is priced at X = 0, as gorge's witness plans it
	// before the announcement (CR 601.2b)
	return av.affords(&cost, pipMaskOf(&cost), 0)
}
