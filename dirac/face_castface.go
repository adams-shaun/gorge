package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Casting a face, cold: reached only when
// the table has a card with faces (genPrep.anyFaces).
//
// A face cast is AkCast with mode bit castFace and the face's index in the
// action's target byte: gorge's "adventure_alt" and "split_alt" (face 1,
// from the hand) and "adventure_recast" (face 0, from exile). applyPriority
// turns the face up before beginCast, as gorge's beginCast emits its
// FlipFace before the ordinary cast transaction; an undone cast turns it
// back (reverseCast, gorge pc.faceBefore, CR 733.1) and every exit from the
// stack resets it (stackExitFace, gorge events apply_zone.go).
//
// The oracle's potential planner prices these routes too
// (potentialModeCastPlan's potentialModeCastFace/offerAsFace), so a face
// cast the floating pool cannot pay is one of gorge's lowered plays. On the
// oracle surface (Config.GorgeUnpriced) the kernel offers the Adventure
// (including its exile recast), split and modal routes from the same mana
// census as an ordinary cast (the floating pool plus untapped sources) and
// taps the witness at priority as the cast begins (lowerFaceCast,
// cast_faceroute.go); Omen and Prepare keep their offer,
// and a Room's other door (room_alt) is priced the same way.
// Off the oracle surface (the default) every route is the plain rules
// census: no planner limit applies. DebugFacePoolOnly restores the old
// pool-only offer for the lockstep mutation.

// castFace is the AkCast mode bit of a face cast (target = the face).
const castFace uint8 = 1 << 2

// faceCastable reports whether p may cast object o (in its zone now) as
// card c, its face up for the cast, with the timing, prohibition, target,
// cost-object and mana gates of the hand walk. The cost is read with the
// face turned up (gorge's offerCastableAsFace probe, faceprobe.go) and put
// back before returning.
func (s *State) faceCastable(k *prep, p uint8, o Obj, c CardID, sorc bool) bool {
	pc := &k.cards[c]
	// Flash is read off the face being cast (CR 715.3a, 720.3a): Whirlwing
	// Stormbrood's front flash leaves its sorcery Omen at sorcery timing.
	if !pc.instant && !sorc && pc.kw&kop.KwFlash == 0 && !s.castFlash(k, p, c) {
		return false
	}
	if s.castHeldOut(o) {
		return false
	}
	ob := &s.objs[o]
	was := ob.card
	ob.card = c // the probe: no touch, restored below
	ok := !(k.gen.anyCantCast && s.castForbidden(k, p, c, o)) && s.spellTargetsFeasibleFrom(k, c, p, o) && s.spellCostsFeasible(k, c, p, o)
	if ok {
		// prices every face route through the potential planner
		// (potential_plan.go potentialModeCastPlan). The priced routes read
		// the same mana census an ordinary cast uses: the floating pool plus
		// untapped sources.
		var av avail
		s.availFor(k, p, &av)
		cost := s.adjustCost(k, p, c, o, pc.cost)
		// the census prices the face's granted delve credit as the hand
		// walk does (legal.go delveOfferCreditHand; gorge delveCredit, CR
		// 702.66a): Teval's grant on an Omen. The either-or check credits
		// its own total, so it takes the pre-delve cost.
		dc := s.delveOfferCredit(k, p, c, o, ob.zone == kop.ZoneHand, cost)
		ok = s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &dc, &av) && s.altAddCastable(k, c, p, o, &cost, &av)
		if ok && DebugFacePoolOnly && !s.FeatPoolPays(p, o, 0) {
			ok = false
		}
		if ok && s.cfg&cfgGorgeUnpriced != 0 && s.contribPays(k, p, c, o, ob.zone == kop.ZoneHand) && !s.FeatPoolPays(p, o, 0) {
			ok = false // a granted-delve face is a "shape:contribution" the planner declines: only the floating pool casts it
		}
		if ok && s.cfg&cfgGorgeUnpriced != 0 && facePlannerLimited(k.t.Cards[c].FaceMode) && !s.FeatPoolPays(p, o, 0) {
			if s.anySacSource(k, p) && !s.faceWitnessReaches(k, p, c, &cost) {
				// The witness may sacrifice a source (a Treasure) the
				// face's own targets need: gorge's potentialWitnessReaches
				// drops the play ("unsupported: witness_unreachable").
				ok = false
			}
		}
	}
	ob.card = was
	return ok
}

// facePlannerLimited reports whether the potential planner's limits (a refused
// restricted pool, a scripted-prefix dead end, and a witness that consumes a
// source the face needs) gate this face route's census offer. Omen's isn't.
func facePlannerLimited(fm uint8) bool {
	switch fm {
	case kop.FaceAdventure, kop.FaceSplit, kop.FaceModal, kop.FaceRoom, kop.FacePrepare:
		return true
	}
	return false
}

// offerFaceCasts offers hand card o's other castable face: an Adventure
// card's Adventure (gorge "adventure_alt", CR 715.3a; a land front's too,
// from the hand walk's land branch), a split card's right half (gorge
// "split_alt", CR 709.3; an aftermath half is cast only from the graveyard,
// face_aftermath.go) or a modal DFC's nonland back (gorge
// "modal_spell", rules legal.go modalSpellBack, CR 712.8; a land back is
// offerFaceLand's play). The caller reached o in its hand walk before the
// front face's timing gate: each face has its own (gorge
// legal_walk_hand.go places these offers above it).
func (s *State) offerFaceCasts(k *prep, d *Decision, p uint8, o Obj, sorc bool) {
	cd := &k.t.Cards[s.objs[o].card]
	if cd.FaceMode == kop.FaceRoom && len(cd.Faces) == 8 {
		s.offerRoomAlt(k, d, p, o, sorc) // face_rooms.go
		return
	}
	if cd.Face != 0 || len(cd.Faces) != 2 {
		return
	}
	switch cd.FaceMode {
	case kop.FaceAdventure, kop.FaceOmen:
	case kop.FaceSplit:
		if k.t.Cards[cd.Faces[1]].Aftermath && !DebugAftermathFromHand {
			return // cast only from the graveyard (face_aftermath.go)
		}
	case kop.FaceModal:
		if k.cards[cd.Faces[1]].land {
			return
		}
	default:
		return
	}
	if s.faceCastable(k, p, o, cd.Faces[1], sorc) {
		d.add(MakeAction(AkCast, uint8(o), 1, castFace))
	}
}

// offerAdventureRecasts offers casting the front of each of p's nonland
// cards on an adventure (ofAdventure) from exile: gorge's
// "adventure_recast" (rules legal_walk_alt.go exileCastsWalk, CR 715.4). A
// land front is never cast (CR 305.1); its play from exile is the land play
// offerAdventureLandPlays adds (CR 715.3d, gorge).
func (s *State) offerAdventureRecasts(k *prep, d *Decision, p uint8, sorc bool) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		if s.objs[o].of&ofAdventure == 0 || k.cards[s.objs[o].card].land {
			continue
		}
		if s.faceCastable(k, p, o, s.objs[o].card, sorc) {
			d.add(MakeAction(AkCast, uint8(o), 0, castFace))
		}
	}
}

// turnFaceCast turns face i of object o up for its cast. The turn is part
// of the provisional proposal, not progress for gorge's hold-out rule
// (rules cast_begin.go emitProposalFlip keeps the suppression state), so
// heldTouch moves with it when o is the held card (castFlagsGen's rule).
func (s *State) turnFaceCast(k *prep, o Obj, i uint8) {
	s.turnFace(k, o, i)
	if s.heldObj == o || s.objs[o].of&(ofAbort1|ofHeld) != 0 {
		s.heldTouch++
	}
}

// unturnFace turns a face cast's card front face up again when its cast
// is undone (reverseCast; gorge pc.faceBefore, CR 733.1). Every face cast
// starts front face up.
func (s *State) unturnFace(k *prep, o Obj, mode uint8) {
	if k.anyRoom && roomDef(k, s.objs[o].card) {
		s.roomUnturn(k, o, mode) // face_rooms.go
		return
	}
	if cd := &k.t.Cards[s.objs[o].card]; cd.Face != 0 && len(cd.Faces) > 0 && cd.FaceMode != kop.FacePrepare { // a prepared copy keeps its spell face (gorge: no faceBefore)
		s.touch(o)
		s.objs[o].card = cd.Faces[0]
	}
}

// stackExitFace runs as spell object o leaves the stack for zone z (not
// the battlefield): an Adventure, split or modal DFC card is its front
// again (gorge events apply_zone.go: CR 715.4, CR 709.4, CR 712.8a; a
// modal back that resolves into a permanent keeps its back), and
// an Adventure that leaves the stack for exile is on an adventure. It
// returns z, or exile for a resolved Adventure (gorge rules stack.go
// spellRestZone; a countered or fizzled one goes to the graveyard,
// spellFizzleZone).
func (s *State) stackExitFace(k *prep, o Obj, z kop.Zone, resolved bool) kop.Zone {
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	if cd.FaceMode == kop.FaceRoom {
		s.roomStackExit(k, o) // the face persists (face_rooms.go)
		return z
	}
	if cd.Face == 0 || len(cd.Faces) == 0 || cd.FaceMode != kop.FaceAdventure && cd.FaceMode != kop.FaceSplit && cd.FaceMode != kop.FaceModal && cd.FaceMode != kop.FaceOmen {
		return z
	}
	s.touch(o)
	ob.card = cd.Faces[0]
	if cd.FaceMode == kop.FaceOmen {
		return s.omenRest(ob, z, resolved) // face_omen.go
	}
	if cd.FaceMode == kop.FaceAdventure && ob.of&(ofCopy|ofToken) == 0 { // a copy keeps its own rest (copy_gen.go)
		if resolved && z == kop.ZoneGraveyard {
			z = kop.ZoneExile
		}
		if resolved && z == kop.ZoneExile {
			// gorge's adventureZoneAvailable: the exile move is the resolved
			// Adventure's own (the adventure_zone marker). A countered
			// Adventure -- directly to exile or redirected there by a
			// replacement -- is not on an adventure.
			ob.of |= ofAdventure
		}
	}
	return z
}

// splitMV is the mana value of object o showing card c when c is a split
// card's front and o is off the stack and the battlefield: the halves'
// sum (CR 709.4b; gorge state/mana_value.go SplitHalves). Otherwise mv.
func (s *State) splitMV(k *prep, c CardID, o Obj, mv int) int {
	cd := &k.t.Cards[c]
	if cd.FaceMode == kop.FaceRoom {
		return s.roomMV(k, c, o, mv) // face_rooms.go
	}
	if cd.FaceMode != kop.FaceSplit || cd.Face != 0 || o == NoObj || len(cd.Faces) != 2 {
		return mv
	}
	if z := s.objs[o].zone; z == kop.ZoneStack || z == kop.ZoneBattlefield {
		return mv
	}
	return int(k.cards[cd.Faces[0]].total) + int(k.cards[cd.Faces[1]].total)
}

// FeatCastFace is the AkCast mode bit of a face cast; the action's target
// is the face cast (0: an Adventure's front from exile).
const FeatCastFace = castFace

// faceWitnessReaches mirrors gorge's potentialWitnessReaches for a face
// cast whose plan sacrifices a source: the witness is paid on a copy of the
// state, and the cast is offered only while its targets are still feasible
// (a Treasure that was the only artifact for Battle Display).
//
// gorge runs the re-check only when the plan actually consumes a source
// (potential_plan.go paymentPlanConsumes: a Sacrifice or ReturnToHand
// activation), so a plan that pays with ordinary mana never withholds the
// cast. A planner that finds no witness (pk.found false) leaves the offer
// in place: the kernel's census already paid the cost, and dropping a play
// gorge reaches through a witness this planner does not model would be the
// worse divergence (kickWitness reads the same way). Only a found witness
// that consumes a source and strands the face's targets withholds the play.
func (s *State) faceWitnessReaches(k *prep, p uint8, c CardID, cost *kop.ManaCost) bool {
	tmp := s.fork()
	defer s.unfork()
	var pk planner
	pk.try = true
	tmp.tapForPK(k, p, cost, 0, NoObj, false, false, &pk)
	if !pk.found || !plannerConsumesSac(&pk) {
		return true
	}
	return tmp.spellTargetsFeasible(k, c, p)
}

// plannerConsumesSac reports whether the planner's chosen witness sacrifices
// or returns one of its sources (gorge paymentPlanConsumes).
func plannerConsumesSac(pk *planner) bool {
	for i := 0; i < pk.bestN; i++ {
		if t := &pk.types[pk.bestTy[i]]; t.sac || t.sacOnly {
			return true
		}
	}
	return false
}
