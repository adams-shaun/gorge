package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Priority options a generated table adds after the core's (before pass):
// plays through a play permission (OpMayPlay) or a play-from-zone static
// (StatMayPlayFrom), activated abilities that work from the graveyard
// (Ability.ActZone), and abilities granted by an Equipment (Static.Grant).
//
// AkCast mode bits beyond the core's (bit 0 kicked, bit 1 flashback): bit 2 a
// face cast (castFace, face_castface.go), bit 3 a cast through a
// permission or static, bit 4 without paying the mana cost.
// A granted ability is AkActivate{source: the creature, target: the
// granting permanent, mode: 0x80 | its index in the granting card}.

const (
	castMayPlay uint8 = 1 << 3
	castFree    uint8 = 1 << 4
)

// mayRecordOfferable reports whether play permission record r may offer its
// play to p: a following record reads the card's zone every time (CR 611.2c,
// cast_maypermission.go); a version-tied one requires the card unmoved since
// the grant and still in a playable zone.
func (s *State) mayRecordOfferable(r mayPlay, p uint8) bool {
	if r.ctrl != p {
		return false
	}
	ob := &s.objs[r.obj]
	if r.follows() {
		return ob.zone == r.followsZone()
	}
	return ob.ver == r.ver && (ob.zone == kop.ZoneExile || ob.zone == kop.ZoneGraveyard)
}

func (s *State) priorityGen(k *prep, d *Decision, p uint8, sorc bool, avp *avail, haveAv bool) {
	// The caller's priority walk already built the same availability
	// picture (priorityOptions); reuse it instead of re-walking the mana
	// sources for every generated option build.
	av := *avp
	if !haveAv && s.priorityGenReadsAv(k, p) {
		s.availFor(k, p, &av)
	}
	pl := &s.pl[p]
	landOK := sorc && pl.landsPlayed < s.landLimit(k, p)
	// play permissions
	var plainSeen objBits
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if !s.mayRecordOfferable(r, p) {
			continue
		}
		if k.gen.anyMayWhileSrc && s.maySrcDead(r) {
			continue // its source left the battlefield (cast_whilesource.go)
		}
		if ownWalkerRecord(r) {
			continue // a warp recast / plotted cast is offered only by its walker (kw_warp.go, kw_plot.go)
		}
		if s.freeMayDominates(k, p, i, r) {
			continue // a free record over the card offers its one cast (cast_maypermission.go)
		}
		if k.gen.anyMayKind && r.kind != 0 {
			if k.gen.anyMayLimit && s.mayLimitSpent(r) {
				continue // the grant's once-per-turn cap is reached (Flameskull)
			}
			s.offerMayKind(k, d, p, r, sorc, landOK, &av)
			if k.gen.anyZeroAlt {
				s.offerZeroAlt(k, d, p, r.obj, mayKindMode(r), mayKindPlain(r), sorc, &av) // cast_mayzeroalt.go
			}
			continue
		}
		// a Paradigm card the offer covers is cast free instead (kw_paradigm.go:
		// gorge offers the card once, the free permission dominating the grant)
		if k.gen.anyParadigm && s.paradigmOffered(k, p, r.obj, sorc) {
			if k.gen.anyZeroAlt {
				s.offerZeroAlt(k, d, p, r.obj, castMayPlay|castFree, false, sorc, &av) // unmodelled beside Paradigm: stalls when it applies (cast_mayzeroalt.go)
			}
			continue
		}
		// gorge's walk dedups its candidates per (zone, id, key)
		// (mayplay_index.go): two plain grants over the same card offer one
		// cast -- two landfall triggers while the card sits in the graveyard
		// both stand. A rider grant's key is its own, so its offer stands
		// beside a plain one.
		if plainSeen.has(r.obj) {
			continue
		}
		plainSeen.set(r.obj)
		s.offerPlay(k, d, p, r.obj, castMayPlay, sorc, landOK, false, false, false, &av)
		if k.gen.anyZeroAlt {
			s.offerZeroAlt(k, d, p, r.obj, castMayPlay, true, sorc, &av) // cast_mayzeroalt.go
		}
	}
	// play-from-zone statics: graveyards, exile, and the hand (free casts)
	if k.gen.anyMayPlay {
		s.priorityMayPlay(k, d, p, sorc, landOK, &av)
	}
	// activated abilities that work from the graveyard
	if k.gen.anyGraveAct {
		s.priorityGraveAct(k, d, p, sorc, &av)
	}
	if k.gen.anyHandAct {
		s.priorityHandAct(k, d, p, sorc, &av)
	}
	if k.anyCyclingGrant {
		s.priorityCyclingGrant(k, d, p, sorc, &av) // kw_cyclinggrant.go: a StatCyclingGrant static's granted cycling
	}
	s.priorityEmbalmGrant(k, d, p, sorc, &av)  // kw_embalmgrant.go
	s.priorityUnearthGrant(k, d, p, sorc, &av) // kw_unearthgrant.go: a StatUnearthGrant static's granted unearths
	if k.gen.anyExileGrant {
		s.priorityExileGrant(k, d, p) // cast_exilegrant.go
	}
	// granted activated abilities
	for m := s.ctrlM[p]; m.any() && s.grantsLive(k); m = m.rest() {
		sl := m.first()
		if !s.grantMay(k, sl) {
			continue
		}
		first := int(d.N)
		s.grantedAbilities(k, sl, func(c CardID, ab uint8) {
			a := &k.t.Cards[c].Abilities[ab]
			if a.Kind != kop.AbActivated {
				return
			}
			host := s.grantedHost(k, sl, c)
			if host == NoSlot && k.gen.globGrant {
				host = s.grantHostGen(k, sl, c, ab)
			}
			if host == NoSlot {
				host = s.selfGrantHost(k, sl, c, ab)
			}
			// an emblem's grant has no slot: gorge registers it under the
			// creator's object (the effect's Source, CR 114.4) and offers the
			// activation against it wherever the creator now is
			embObj := NoObj
			if host == NoSlot && k.gen.anyEmbGrant {
				if o, ok := s.embGrantHostObj(k, sl, c, ab); ok {
					embObj = o
				} else {
					return
				}
			}
			if embObj == NoObj && (host == NoSlot || a.Cost.Flags&kop.CostTapHost != 0 && s.perms[host].flags&pfTapped != 0) {
				return
			}
			if embObj == NoObj && a.Cost.Host != 0 && s.perms[host].ctrl != p {
				return // only a permanent the payer controls can be sacrificed or exiled (CR 701.21a, 118.3)
			}
			if s.cfg&cfgGorgeUnpriced != 0 && !s.poolPaysAct(k, p, &a.Cost.Mana, payDesc{class: pdAct, card: s.cardAt(sl), src: sl}) && !s.copyGrantIs(k, sl, c, ab) {
				// gorge's granted-keyword offer prices the real pool only
				// (rules.offerCastable with no hypothetical bound), the same
				// gate the Equipment-granted path above follows; a source-paid
				// price here would offer an activation gorge withholds. The
				// pool counts the restricted batches an activation may spend
				// (CR 106.6: Karn, Legacy Reforged's artifact-only {C} pays
				// the generic of Ephara's Radiance's {1}{W}), as the
				// printed-ability offer does (activateOffers: poolPaysAct).
				return
			}
			if s.canActivate(k, sl, 255, a, sorc, &av) {
				// two copies of one grantor give the same ability: the second is the
				// first's option (gorge's offer list collapses them the same way)
				hobj := embObj
				if hobj == NoObj {
					hobj = s.perms[host].obj
					if host == sl {
						// a record's grant is hosted by the object that made it (act_ovmaker.go)
						lts, lost := s.lossAt(k, sl)
						if mk := s.grantMakerObj(k, sl, c, ab, lost, lts); mk != NoObj {
							hobj = mk
						}
					}
				}
				act := MakeAction(AkActivate, TCode(s.perms[sl].obj), TCode(hobj), 0x80|ab)
				for i := first; i < int(d.N); i++ {
					if d.Options[i] == act {
						return
					}
				}
				d.add(act)
			}
		})
	}
	// gained activated abilities (GainsAbilitiesOf$, static_gainsof.go)
	s.gainedAbilities(k, d, p, sorc, &av)
}

// offerPlay offers playing card object o through a permission.
func (s *State) offerPlay(k *prep, d *Decision, p uint8, o Obj, mode uint8, sorc, landOK, anyMana, anyColor, unpriced bool, av *avail) {
	// the avail is the caller's walk picture: the cast's own delve credit
	// prices its twobrid faces and is restored after, so the next option does
	// not price with it
	saved := av.delve
	defer func() { av.delve = saved }()
	if av.dynSac {
		// A cast's census leaves a dynamic {T}, sacrifice source (Shrine of
		// Boundless Growth) out, as a hand cast's does (priorityOptions): the
		// activated-ability census the walk carries here prices it, so a
		// may-play cast offered on that unit alone was applied plainly, with
		// the source never activated, and its payment found no plan. The
		// counter-cost gap offers the cast that source funds (mana_sacctr.go).
		var cav avail
		s.availFor(k, p, &cav)
		av = &cav
	}
	c := s.objs[o].card
	pc := &k.cards[c]
	anyMana = anyMana || mode&castFree == 0 && s.convAnyType(k, p, o) // mana_convertcreature.go
	if pc.land {
		if landOK {
			s.offerLandOnce(d, o)
		}
		return
	}
	if !pc.instant && !sorc && !s.castFlash(k, p, c) {
		return
	}
	if k.gen.anyCantCast {
		var ccost *kop.ManaCost
		if mode&castFree == 0 {
			ccost = &pc.cost // a free cast fixes X at 0 (CR 107.3b)
		}
		if s.castForbiddenCost(k, p, c, o, ccost, av) || s.castHeldOut(o) {
			return // a card twice reversed for no progress is held out of every cast route (CR 733.2), not only a paid one
		}
	} else if s.castHeldOut(o) {
		return
	}
	s.offerBargainPlay(k, d, p, o, c, mode, anyMana, av) // kw_bargain.go
	if mode&castFree == 0 {
		base := pc.cost
		if k.gen.anyConvoke && s.tapPays(k, s.creditReader(p, o), c, o, false) {
			// gorge castOfferBase reads the keyword off the card's own controller (creditReader): p's Ironheart over the other seat's exiled card is no credit
			base = s.convokeCredit(k, p, c, o, base) // gorge castOfferBase: a permission cast convokes too
		}
		av.delve = s.tbDelveCredit(k, p, c, o, s.objs[o].zone == kop.ZoneHand) // the faces' credit (CR 702.66a)
		cost := s.adjustFit(k, av, p, c, o, base, true)
		if !av.tbLeafCredited(&base) {
			cost = s.delveOfferCredit(k, p, c, o, s.objs[o].zone == kop.ZoneHand, cost)
		}
		if anyMana {
			cost = anyType(cost)
		}
		if s.cfg&cfgGorgeUnpriced != 0 && s.g.availX&availWalk == 0 && s.creditOfferLive(k, p, c, o, s.objs[o].zone == kop.ZoneHand) &&
			!s.poolOfferPays(k, p, c, o, &base, &cost, anyMana, anyColor) {
			// gorge prices no potential play of a may-play cast carrying a
			// live convoke, improvise or delve credit: the potential plan
			// withholds it (potentialModeCredit's "cost:credit",
			// potential_plan.go), so only the floating pool casts it -- the
			// offer gate (mayPlayAffordable: castable over pay.AvailableFor)
			// prices the pool against the credited composed cost -- and the
			// afford read below prices the census (availFor: the pool plus the
			// untapped sources). Black Widow, Super Spy's dig grant
			// (MayPlay$ True | MayPlayIgnoreType$ True) over the exiled
			// Futurist Forge with an empty pool, two untapped sources paying
			// the rider-converted {2}, is the pair: gorge poses pass, the
			// posed enumeration drops the cast here; a potential walk keeps
			// the census read (availWalk), whose hyp bound is the one gorge's
			// own potential walk prices the play against.
			return
		}
		if anyColor {
			if !av.affordsAnyColor(&cost, 0) {
				return
			}
		} else if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) && !s.convAnyPoolPays(k, p, o, &cost) {
			return // (the any-colour conversion's pool route: mana_convertcreature.go)
		}
	} else if !s.freeCastRaisePayable(k, p, c, o, unpriced, av) {
		return
	}
	// a graveyard card cast through the permission is on the stack when its
	// targets are chosen (CR 601.2a, 601.2c): it is no candidate for them
	if !s.spellTargetsFeasibleFrom(k, c, p, o) || !s.spellCostsFeasible(k, c, p, o) {
		return
	}
	if mode&castFree != 0 && !s.altAddCastableFrom(k, c, p, o, &kop.ManaCost{}, &kop.ManaCost{}, true, anyMana, av) {
		return // a free cast still owes its either-or additional cost (CR 601.2b, 118.9d): Mudbutton Cursetosser's behold or {2}
	}
	if mode&castFree != 0 && s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.objs[o].zone == kop.ZoneHand && s.altFreePartUnpooled(k, p, c, o) {
		return // a free cast from the hand is gorge's alternative cost (Omniscience); it shares its play with the printed cast, so gorge's planner never prices it ("ambiguous"): only a pool that pays its either-or part offers it
	}
	if mode&castFree == 0 {
		cost := s.adjustFit(k, av, p, c, o, pc.cost, true)
		if anyMana {
			cost = anyType(cost)
		}
		altCost := cost // (altAddCastableFrom credits its own total)
		if !av.tbLeafCredited(&pc.cost) {
			cost = s.delveOfferCredit(k, p, c, o, s.objs[o].zone == kop.ZoneHand, cost)
		}
		s.g.mayPlayAny = anyMana
		offers := s.gorgeOffersMay(p, o, anyColor)
		s.g.mayPlayAny = false
		if !s.altAddCastableFrom(k, c, p, o, &altCost, &pc.cost, true, anyMana, av) || !offers || s.castHeldOut(o) {
			return
		}
		if cost.X != 0 && !s.spellXBoundPayable(k, c, p, &cost) {
			return // some payable X must leave a legal target, as a hand cast's (CR 601.2b, 601.2c; gorge xBoundTargetPayable)
		}
		if unpriced && !s.windowPoolPays(p, &cost, anyColor) && (s.g.availX&(availAny|availGaps) == 0 || (!anyMana && !anyColor)) {
			// (the potential walks list an any-type one: gorge's planner
			// proves it insufficient, and a scripted prefix may make the
			// pool pay it; an "ambiguous" one is never searched)
			return
		}
	}
	d.add(MakeAction(AkCast, TCode(o), 0, mode))
}

// freeCastRaisePayable reports whether a cost-waived cast of o can pay the
// cost statics' increases (CR 601.2f, 118.9d). The composed raise is the
// spell's TOTAL cost, so its own Delve prices it too (CR 702.66a: gorge's
// composedOfferCostInto prices the gate at min(generic, the graveyard's
// size); Vision, Spectral Synthezoid's once-a-turn free cast of the delve
// Temporal Trespass under Grand Arbiter Augustin IV's {1}: the ten cards in
// the graveyard delve the {1} away over an empty pool, tail-52 pair 5). A
// free cast whose raise the floating pool pays is one of gorge's ordinary
// walk's offers; one it cannot pay is still one of gorge's potential plays
// when the planner prices it: the full surface reaches it by lowering a
// potential play, whose plan taps the sources (Urza's Gelatinous Genesis
// under Thorn of Amethyst: the untapped Island pays the {1}, tail-64 pair
// 38), and on the autopay surface a planned-zone cast the census pays is one
// of those potential plays in its own list (Urza's exile-held Lash Out and
// Vibrating Sphere under Thorn: the Construct token's tap-another mana pays
// the {1}, tail-64 pair 75 -- the plain walk drops it at the census, and the
// gaps walk plus scriptOffers carry the tap-another prefix search). That
// plan covers only the graveyard and exile (mayPlayPlannedZone), so a hand
// or top-of-library permission still follows the ordinary pool-only gate --
// the potential walks and the any-mana census included (the walks' probes
// list the cast, but a hand cast's plan lowers only through the floating
// pool; Artificer's Epiphany under the same {1} over an empty pool, tail-52
// pair 5). An offer the planner leaves unpriced (two matching permissions:
// "ambiguous") is never reached by a potential play, so it keeps the
// pool-only gate too.
//
// The script walks list the free cast even when the census cannot pay the
// raise, so the prefix search can tap a source for it that the census leaves
// out: Urza, Lord High Artificer's "Tap an untapped artifact: Add {U}" beside
// Thorn of Amethyst's {1} on the free may-play cast of Battle of Wits
// (tail-64 pair 22, decision 702). The prefix's mana floats into the pool,
// which is what pays the raise there -- the same pool-only gate.
func (s *State) freeCastRaisePayable(k *prep, p uint8, c CardID, o Obj, unpriced bool, av *avail) bool {
	if !s.freeOwes(k) {
		return true
	}
	cost := s.adjustCostGen(k, p, c, o, kop.ManaCost{})
	if cost == (kop.ManaCost{}) {
		return true
	}
	cost = s.delveOfferCredit(k, p, c, o, s.objs[o].zone == kop.ZoneHand, cost)
	if cost == (kop.ManaCost{}) {
		return true
	}
	if s.g.availX&(availGaps|availAny|availWalk) != 0 && mayPlayPlannedZone(s.objs[o].zone) {
		return true
	}
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) {
		return false
	}
	if !unpriced && (s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) == 0 || mayPlayPlannedZone(s.objs[o].zone)) {
		return true
	}
	return s.windowPoolPays(p, &cost, false)
}

// poolOfferPays is the pool half of gorge's may-play offer gate (castable's
// manaFeasible over pay.AvailableFor): the floating pool pays the credited
// composed cost the offer carries, a twobrid one on some announcement face,
// the delve credit paying the generic faces (CR 702.66a, 107.4e). cost is the
// offer's composition, resolved to the face the census pays; for a twobrid
// cost that face is not the answer -- Teval, Arbiter of Virtue's granted
// delve covers a generic face an empty pool never would -- so base is
// re-composed on the pool the way offerPlay composes it on the census.
func (s *State) poolOfferPays(k *prep, p uint8, c CardID, o Obj, base, cost *kop.ManaCost, anyMana, anyColor bool) bool {
	if s.windowPoolPays(p, cost, anyColor) {
		return true
	}
	if base.Hyb[0] != kop.HybTwobrid {
		return false
	}
	fromHand := s.objs[o].zone == kop.ZoneHand
	var pav avail
	s.availPool(p, &pav)
	pav.life = s.pl[SeatIx(p)].life
	pav.delve = s.tbDelveCredit(k, p, c, o, fromHand)
	pcost := s.adjustFit(k, &pav, p, c, o, *base, true)
	if !pav.tbLeafCredited(base) {
		pcost = s.delveOfferCredit(k, p, c, o, fromHand, pcost)
	}
	if anyMana {
		pcost = anyType(pcost)
	}
	if anyColor {
		return pav.affordsAnyColor(&pcost, 0)
	}
	return pav.affords(&pcost, pipMaskOf(&pcost), 0)
}

// anyType turns coloured pips into generic (mana of any type may be spent).
func anyType(c kop.ManaCost) kop.ManaCost {
	c = anyTypePips(c)
	c.Generic += uint8(hybN(&c))
	c.Hyb = [len(kop.ManaCost{}.Hyb)]kop.ColorMask{}
	c.HybOrd = 0
	c.Tb = 0
	return c
}

// anyTypePips is anyType keeping the hybrid pips: gorge still announces
// them (pay.ManaAsk), the any-type rider making every face payable; a
// resolved face is generic again (buildCost).
func anyTypePips(c kop.ManaCost) kop.ManaCost {
	for i := range c.Pips {
		c.Generic += c.Pips[i]
		c.Pips[i] = 0
	}
	return c
}

// planUnderRider is the mana gorge's planner funds for a cast under a may-play
// rider (pay.PlanManaUnderRider): a pip the rider lets any mana pay is generic to
// the plan, a hybrid pip included (it is two colours, never {C}). An any-colour
// rider leaves {C}.
func planUnderRider(c kop.ManaCost, anyMana, anyColor bool) kop.ManaCost {
	if anyMana {
		c = anyTypePips(c)
	} else if anyColor {
		for i := 0; i < kop.C; i++ {
			c.Generic += c.Pips[i]
			c.Pips[i] = 0
		}
	}
	if anyMana || anyColor {
		c = hybridGeneric(c)
	}
	return c
}

// hybridGeneric turns each hybrid pip of c into one generic: the pips left
// after a face was chosen, paid by mana of any type, are payable by any mana.
// A twobrid or Phyrexian cost has no two-colour hybrid pip: its Hyb bytes are
// the HybTwobrid/HybPhyrexian markers, and its Tb pips stay, as gorge's
// pay.PlanManaUnderRider keeps Twobrid and Phyrexian (CR 107.4e, 107.4f).
func hybridGeneric(c kop.ManaCost) kop.ManaCost {
	if c.Hyb[0] == kop.HybTwobrid {
		return c
	}
	for i := range c.Hyb {
		if c.Hyb[i] != 0 && c.Hyb[i] != kop.HybTwobrid {
			c.Generic++
			c.Hyb[i] = 0
		}
	}
	return c
}

// castFlagsGen applies a cast's generated mode bits to the entry being
// built and consumes the once-per-turn static that permitted it.
func (s *State) castFlagsGen(k *prep, o Obj, mode uint8) { s.castFlagsGenTgt(k, o, mode, 0) }

// castFlagsGenTgt is castFlagsGen for the cast action whose target byte is
// tgt: a {0} alternative's marker (cast_mayzeroalt.go) says the cast went
// through that static, whose once-per-turn mark it spends.
func (s *State) castFlagsGenTgt(k *prep, o Obj, mode, tgt uint8) {
	s.g.raiseB = 0 // a spent surcharge record sets it again (spendMayKind)
	if s.bld().e.kind == seSpell && s.convAnyType(k, s.bld().e.ctrl, o) {
		s.bld().e.flags |= sfAnyMana // mana_convertcreature.go
	}
	// it leaves the graveyard: a granted flashback ends. The grant is part
	// of the card's view, so mark the card changed. The clear can still be
	// undone (an unpayable either-or part restores the grant), so it is not
	// progress for gorge's hold-out rule: keep heldTouch in step, or the
	// next undo would look like progress and the card would not be held out
	// (castHeldOut). This holds whichever card is held: the touch happens
	// before the push's own (uncompensated) touches and is undone by a
	// reversal (CR 733.1), so it must not clear an earlier aborted hand
	// copy's count when this flashback cast is the one that reverses
	// (pair 478, Phantom Interference).
	if s.fbGranted(k, o) {
		s.touch(o)
		s.heldTouch++
	}
	s.clearFB(k, o)
	if mode&castFree != 0 {
		s.bld().e.flags |= sfFree
	}
	if mode&castMayPlay == 0 || k.gen.altFree != nil && s.altFreeCast(k, o, mode) {
		return // (a free alternative cost spends no permission: cast_altfree.go)
	}
	s.bld().e.flags |= sfMayPlay
	p := s.bld().e.ctrl
	if mode == castMayPlay|castFree && FeatIsZeroAltTgt(tgt) && k.gen.anyZeroAlt {
		if ok, zst := s.spendZeroAlt(k, o, p); ok && tgt == zeroAltTgt {
			mode &^= castFree // the alternative beside a priced plain cast: that cast's permission is spent below (cast_mayzeroalt.go)
			if zst.a.Static.StFlags&kop.StMayPayLife != 0 {
				s.bld().payLifeMV = true // the alternative's cost is the mana value in life (Demon of Fate's Design)
			}
			if zst.a.Static.StFlags&kop.StMayPayEnergy != 0 {
				s.bld().payEnergy = uint8(zst.a.Static.A) // the alternative's cost is the static's energy (Nissa, Worldsoul Speaker)
			}
		}
	}
	// a permission record is spent; a once-per-turn static is marked
	if k.gen.anyMayKind && s.spendMayKind(k, o, p, mode) {
		return
	}
	if s.spendMayRecord(k, o, p) {
		return
	}
	lifeOnly := s.castLifeOnly(k, p, o, mode) // cast_staticpaylife.go
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		if castSkipsStatic(st, mode, lifeOnly) {
			return true // the other cast's permission (cast_staticpaylife.go)
		}
		if k.gen.anyMayRaise && !s.mayRaiseSpend(k, p, o) {
			return true // the raise can't be paid (cost_mayplayraise.go)
		}
		if st.a.Static.StFlags&kop.StRemove6Counters != 0 && s.creatureCounters(k, p) < int(st.a.Static.A) {
			return true // unpayable: another permission is the cast's (Muldrotha; priorityMayPlay, pair 20520)
		}
		if st.a.Static.StFlags&kop.StMayPayLife != 0 {
			if !s.payLifeOfferable(k, p, o) {
				return true // not the permission that offered the cast (cast_staticpaylife.go)
			}
			s.bld().payLifeMV = true
		}
		if st.a.Static.StFlags&kop.StAnyManaType != 0 {
			s.bld().e.flags |= sfAnyMana
		}
		if st.a.Static.StFlags&kop.StOncePerTurn != 0 && !s.mayPlayFolds(k, p, o) {
			u, ab, host := s.staticUseKey(k, st)
			s.markUsedBy(u, ab, host)
			s.bld().bfl |= bflUsedMark
		}
		if st.a.Static.StFlags&kop.StRemove6Counters != 0 {
			s.bld().subCtr = uint8(st.a.Static.A) // Static.A counters (Quilled 6, Dawnhand 3), chosen one counter at a time (subCtrOptions)
		}
		return false
	})
}

// spendMayRecord removes p's live permission record for o, the one a play
// of o used (CR 701.18b: a cast or a land play); false when none names it
// (a static permitted the play). The cast path and playLandNow share it.
func (s *State) spendMayRecord(k *prep, o Obj, p uint8) bool {
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.obj != o || r.ctrl != p || !(r.follows() || r.ver == s.objs[o].ver) ||
			ownWalkerRecord(r) || s.mayLimitSpent(r) || k.gen.anyMayWhileSrc && s.maySrcDead(r) {
			continue
		}
		s.stampMayExile(k, i) // repl_stackexile.go
		if r.turn&mayTurnConvC != 0 && s.buildLive() {
			s.bld().convC = true // the cast pays under the colourless-mana conversion (cast_mayconvc.go)
		}
		if !r.follows() && !(r.dur() == kop.DurWhileGrave && r.kind&kop.MayPlayLimitMask == 0) {
			// A following record outlives its own cast (gorge's continuous
			// grant); an Effect-delivered graveyard grant stays live after
			// the card leaves, and mayPersistMoved re-arms it when the card
			// returns (gorge re-reads the effect's static, CR 611.2a).
			s.removeMay(i)
		}
		return true
	}
	return false
}

// creatureCounters is the number of counters on p's creatures.
func (s *State) creatureCounters(k *prep, p uint8) int {
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesGen(k, sl)&kop.TCreature == 0 {
			continue
		}
		p := &s.perms[sl]
		var held ctrHeld
		for _, c := range held[:p.heldKinds(s, &held)] {
			if !kop.IsCounterKind(int(c)) {
				continue // CtrLevel is no counter (kw_class.go)
			}
			n += p.ctrN(s, int(c))
		}
	}
	return n
}

// buildCostGen is the mana cost of the spell being built on a generated
// table, when it differs from the core's (free, or mana of any type).
func (s *State) buildCostGen(k *prep) (kop.ManaCost, bool) {
	c, _, ok := s.buildCostGenPre(k, false)
	return c, ok
}

// buildCostGenPre is buildCostGen with buildCostPre's pre and done.
func (s *State) buildCostGenPre(k *prep, pre bool) (kop.ManaCost, bool, bool) {
	e := &s.bld().e
	if e.kind != seSpell {
		return kop.ManaCost{}, false, false
	}
	if e.flags&sfFree != 0 {
		// "without paying its mana cost" skips the mana cost only: the
		// {3}{B} part of Eaten Alive's either-or additional cost is
		// still paid (CR 118.9d; Etali's free cast, pair 9954)
		var c kop.ManaCost
		cd := &k.t.Cards[e.card]
		if cd.HasAltAdd && s.bld().altAdd == 1 {
			c = cd.AltAddCost.Mana
		}
		if cd.HasModeCost {
			c = spreeCost(cd, e.mode, c) // Spree's mode costs are additional costs (CR 702.172a)
		}
		if s.cfg&cfgCommander != 0 {
			addCommanderTax(&c, s.commanderTax(e.ctrl, e.obj)) // CR 903.8: a free cast from the command zone still pays the tax
		}
		// the cost statics apply to what is left (CR 601.2f, 118.9d): a
		// RaiseCost charges its generic, and a ReduceCost takes its generic
		// off an additional cost too (The Wind Crystal reducing a free
		// Restoration Magic's Tiered Cura {1} to {0}, pair 1268). The
		// reduction cannot make the empty cost negative, so a free cast with
		// no additional cost is charged the same nothing.
		c = s.adjustCostGen(k, e.ctrl, e.card, e.obj, c)
		return c, false, true
	}
	if e.trigPl == trigPlAirbend {
		return s.airbendCost(k, e.ctrl, e.card, e.obj), false, true // act_airbend.go
	}
	if e.trigPl == trigPlForetold {
		return s.foretellCost(k, e.ctrl, e.card, e.obj), false, true // kw_foretell.go
	}
	if s.bld().altAdd == buildAltCost {
		return s.altCostOf(k), false, true // the card's own alternative mana cost (cast_altfree.go)
	}
	if e.flags&sfMayPlay != 0 {
		cd := &k.t.Cards[e.card]
		c := cd.Cost
		if e.flags&sfKicked != 0 {
			c = s.addKickerPayments(cd, e, c)
		}
		if cd.HasAltAdd && s.bld().altAdd == 1 {
			c = addCost(c, cd.AltAddCost.Mana) // Eaten Alive's {3}{B} part (CR 601.2f)
		}
		if cd.HasModeCost {
			c = spreeCost(cd, e.mode, c)
		}
		done := false
		if pre {
			c, done = s.foldTb(k, c)
		}
		c = s.adjustCostGen(k, e.ctrl, e.card, e.obj, c)
		if e.flags&sfAnyMana != 0 {
			c = anyTypePips(c)
		}
		return c, done, true
	}
	return kop.ManaCost{}, false, false
}

// activateGen starts a graveyard or granted activation; false lets the core run
// an ordinary one.
func (s *State) activateGen(k *prep, a Action) bool {
	o := Obj(a.Source())
	ob := &s.objs[o]
	if a.Mode() == unearthGrantMode && k.anyCyclingGrant && ob.zone == kop.ZoneHand {
		// a granted cycling (kw_cyclinggrant.go): the granting permanent's
		// latent body run with the affected hand card as its source; the
		// body's discard-this cost discards that card
		host := Obj(a.Target())
		hc := s.objs[host].card
		if s.objs[host].zone == kop.ZoneBattlefield {
			hc = s.cardAt(s.objs[host].slot)
		}
		ab := cyclingGrantBody(k, hc)
		if ab < 0 {
			return false
		}
		CyclingGrantStats.Activations.Add(1)
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: hc, ab: uint8(ab), ctrl: s.prio, trigObj: NoObj}}
	} else if a.Mode() == unearthGrantMode && k.anyOffKwGrant && ob.zone == kop.ZoneGraveyard {
		// a granted unearth (kw_unearthgrant.go): the granting permanent's
		// latent body run with the affected card as its source. The mode byte
		// is a reserved granted index no ledger grant holds; the target byte
		// names the granting permanent.
		host := Obj(a.Target())
		hc := s.objs[host].card
		if s.objs[host].zone == kop.ZoneBattlefield {
			hc = s.cardAt(s.objs[host].slot)
		}
		ab := offKwGrantBody(k, hc)
		if ab < 0 {
			return false
		}
		UnearthGrantStats.Activations.Add(1) // a StatUnearthGrant activation (kw_unearthgrant.go)
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: hc, ab: uint8(ab), ctrl: s.prio, trigObj: NoObj}}
	} else if a.Mode() == EmbalmGrantMode && k.anyEmbalmGrant && ob.zone == kop.ZoneGraveyard {
		// a granted embalm (kw_embalmgrant.go): the granting card's latent
		// body run with the embalmed card as its source
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: k.embalmCard, ab: k.embalmAb, ctrl: s.prio, trigObj: NoObj}}
	} else if isGainedMode(a.Mode()) {
		// a gained ability (static_gainsof.go): the foreign card's body,
		// activated as the recipient's own ability (no granting host)
		host := Obj(a.Target())
		hc := s.objs[host].card
		if s.objs[host].zone == kop.ZoneBattlefield {
			hc = s.cardAt(s.objs[host].slot)
		}
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: hc, ab: a.Mode() &^ gainedMode, ctrl: s.prio, trigObj: NoObj}}
		s.markGainedUsed(&k.t.Cards[hc].Abilities[a.Mode()&^gainedMode], o, host, a.Mode()) // a once-each-turn one (static_gainsof.go)
	} else if a.Mode()&0x80 != 0 {
		host := Obj(a.Target())
		ab := a.Mode() &^ 0x80
		hc := s.objs[host].card
		trig := host
		if mc, ok := s.grantMakerCard(k, o, host, ab); ok {
			// a record's grant hosted by the object that made it (act_ovmaker.go):
			// the maker's body, run with the holder as its host
			hc, trig = mc, o
		} else if body := s.exileGrantCardFor(host); body != 0 && exileGrantHasAb(k, body, ab) {
			// An exile grant records the body it granted at resolution. Read it
			// on either side of the battlefield: the host may be a copy whose
			// printed card has no such ability, or a card that has left the
			// battlefield since the grant (gorge's Animate keeps the body on
			// the exiled card itself).
			hc = body
		} else if s.objs[host].zone == kop.ZoneBattlefield {
			hc = s.grantCardAt(k, s.objs[host].slot, ab)
		} else {
			// An off-battlefield grantor (an emblem's creator: the grant is
			// offered against the effect's Source wherever it now is,
			// CR 114.4). The grant names an ability of the card that granted
			// it, which a multi-face grantor may no longer show: a
			// transforming DFC is front-face-up in every zone but the
			// battlefield (CR 711.5a) while the granted body lives on the
			// face that granted it (CR 113.7a). Read the faces the way the
			// mint's fold does (events.ResolveSVarAcrossFaces): the object's
			// current card first, then the layout's face cards in index
			// order -- the first whose ability ab is an activated ability is
			// the grant's card. A one-face grantor keeps its card, as before.
			for _, fc := range k.t.Cards[hc].Faces {
				if fc != hc && len(k.t.Cards[fc].Abilities) > int(ab) && k.t.Cards[fc].Abilities[ab].Kind == kop.AbActivated {
					hc = fc
					break
				}
			}
		}
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: hc, ab: ab, ctrl: s.prio, trigObj: trig}}
	} else if ob.zone != kop.ZoneBattlefield {
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: ob.card, ab: a.Mode(), ctrl: s.prio, trigObj: NoObj}}
	} else {
		return false
	}
	s.bld().e.tgt = noTargets
	s.bld().stage = stageFirst[seAbility]
	s.ctl = ctlBuild
	if !isGainedMode(a.Mode()) && a.Mode()&0x80 == 0 {
		s.lowerHybridOnly(k, a.Mode()) // a hand or graveyard ability (cycling {U/B}): the same witness lowering as a battlefield one
	}
	return true
}

// tapHostGen pays CostTapHost: tap the permanent granting the ability.
func (s *State) tapHostGen(k *prep, e *stackEntry) {
	h := e.trigObj
	if h != NoObj && s.objs[h].zone == kop.ZoneBattlefield {
		was := s.perms[s.objs[h].slot].flags & pfTapped
		s.perms[s.objs[h].slot].flags |= pfTapped
		s.touch(h)
		if was == 0 {
			if s.subs[kop.TrigTaps].any() {
				s.fireTaps(k, s.objs[h].slot)
			}
			s.markTapped(k, s.objs[h].slot) // only a tap that happens marks
		}
	}
}

// targetableGen applies the generated keywords to targeting: hexproof from
// instants (an instant spell an opponent controls), protection from black /
// red (a black / red source).
func (s *State) targetableGen(k *prep, sl Slot, p uint8) bool {
	kw := s.kwGen(k, sl)
	var hexC kop.ColorMask
	if k.gen.anyColGrant && s.perms[sl].ctrl != p {
		hexC, _, _ = s.cgOf(sl) // hexproof from a chosen colour (static_colourgrant.go)
	}
	if k.gen.anyHexCol && s.perms[sl].ctrl != p {
		hexC |= s.hexPrinted(k, sl) // printed hexproof from a colour (kw_hexproofcolor.go)
	}
	if k.gen.anyHexOwn && s.perms[sl].ctrl != p {
		hexC |= s.hexOwnColors(k, sl) // a granted hexproof from its own colours (kw_hexproofcolor.go)
	}
	if k.gen.anyHexUB && s.perms[sl].ctrl != p {
		hexC |= s.hexUBColours(k, sl) // Veil of Summer's grant (repl_turnrepl.go)
	}
	if k.gen.anyCantTgt && s.cantTargetGen(k, sl, p) {
		return false // a printed CantTarget static (tgt_canttarget.go)
	}
	if kw&(kop.KwHexproofFromInstants|kop.KwProtBlack|kop.KwProtRed) == 0 && !(k.gen.anyProtCol && s.protPrinted(k, sl) != 0) &&
		!(k.gen.anyProtF && (k.t.Cards[s.cardAt(sl)].ProtFrom != 0 || k.gen.anyProtGrant)) && hexC == 0 {
		return true
	}
	spell, card, c, srcSl, ok := s.tgtSource(k)
	if !ok {
		return true
	}
	// an instant spell, or an ability whose source is an instant (Faebloom
	// Trick's reflexive trigger: CR 702.11d's "sources with that quality")
	if kw&kop.KwHexproofFromInstants != 0 && s.perms[sl].ctrl != p && (spell || s.curEntry() != nil) && k.cards[card].types&kop.TInstant != 0 {
		return false
	}
	if kw&kop.KwProtBlack != 0 && c&kop.ColB != 0 || kw&kop.KwProtRed != 0 && c&kop.ColR != 0 {
		return false
	}
	if hexC&c != 0 {
		return false // CR 702.11d: a source of that colour an opponent controls
	}
	if k.gen.anyProtCol && s.protPrinted(k, sl)&c != 0 {
		return false
	}
	if k.gen.anyProtF && (srcSl != NoSlot || spell || s.curEntry() != nil || s.g.probe.off) && s.protQuality(k, sl, srcSl, card) {
		return false
	}
	return true
}

// probe records which spell or ability a priority-time feasibility check
// is for, so targeting restrictions that depend on the source (hexproof
// from instants, protection from a colour) see it as the build will.
type probe struct {
	kind uint8
	src  Slot
	card CardID
	off  bool  // the ability's source is the card, off the battlefield (a graveyard activation)
	x    uint8 // the X an affordability sweep is trying, plus one; 0 is an unannounced X
	ctrl uint8 // who would cast or activate it
}

const (
	probeSpell   = 1
	probeAbility = 2
)

func (s *State) probeSpell(k *prep, c CardID, p uint8) bool {
	s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
	ok := s.spellTargetsFeasible(k, c, p)
	s.g.probe = probe{}
	return ok
}

func (s *State) probeAbility(k *prep, a *kop.Ability, p uint8, src Slot, modes uint8) bool {
	s.g.probe = probe{kind: probeAbility, src: src, ctrl: p}
	ok := s.abilityTargetsFeasible(k, a, p, src, modes)
	s.g.probe = probe{}
	return ok
}

// probeAbilityFrom is abilityTargetsFeasible for an activated ability of the
// card c that works off the battlefield (Renew from a graveyard, Channel from
// the hand): its source
// is the card, so protection from its colours bars a target (CR 115.2, 702.16b).
func (s *State) probeAbilityFrom(k *prep, a *kop.Ability, p uint8, c CardID) bool {
	if k.gen == nil || s.g.probe.kind != 0 {
		return s.abilityTargetsFeasible(k, a, p, NoSlot, 0xff)
	}
	s.g.probe = probe{kind: probeAbility, src: NoSlot, card: c, off: true, ctrl: p}
	ok := s.abilityTargetsFeasible(k, a, p, NoSlot, 0xff)
	s.g.probe = probe{}
	return ok
}

// deferSacGen: a permanent sacrificed to pay a cost that could still tap
// for mana is sacrificed after the mana is paid (CR 601.2g-h: mana
// abilities are activated before costs are paid, so a Llanowar Elves can
// tap for the ability's mana and then be sacrificed to it).
func (s *State) deferSacGen(k *prep, sl Slot) bool {
	return s.perms[sl].flags&pfTapped == 0 && s.canTapForMana(k, sl)
}

// paySacsGen sacrifices the cost picks deferSacGen held back.
func (s *State) paySacsGen(k *prep) {
	if k.gen == nil {
		return
	}
	if k.costObj {
		// gorge payCast: the returns before the sacrifices, both after the
		// planned mana sources are tapped (a returned land may pay)
		s.payReturnPicks()
	}
	if k.gen.anyBatch {
		defer s.batchClose(s.sacBatchOpen()) // gorge emitSacrifices: one sacrifice batch (act_proliferate.go)
	}
	if e := &s.bld().e; e.kind == seAbility && e.ab < uint8(len(k.t.Cards[e.card].Abilities)) {
		if f := k.t.Cards[e.card].Abilities[e.ab].Cost.Flags; f&kop.CostReturnSelf != 0 && f&kop.CostSac != 0 {
			// gorge payCast: the source's return before the sacrifices
			// (Chthonian Nightmare); the deferred call below is then a no-op
			s.returnSelfDeferred(k, e)
		}
	}
	exile := s.pickExiles(k)
	for i := 0; i < int(s.bld().pickN); i++ {
		o := s.bld().picks[i]
		if s.bld().pickKs[i] == pkSac && s.objs[o].zone == kop.ZoneBattlefield {
			s.snapSacPick(o) // RefSacrificed (CR 707.10: a copy keeps it)
			if exile {
				s.leave(k, s.objs[o].slot, kop.ZoneExile, false) // Exile<N/Spec>: no sacrifice (cost_exilebf.go)
				continue
			}
			s.fireSac(k, s.objs[o].slot)
			if !s.parkCost(k, o, parkLeave, 0) {
				s.leave(k, s.objs[o].slot, kop.ZoneGraveyard, false)
			}
		}
	}
	if s.escBfEntry(k) {
		s.payExilePicks() // after the escape cast's exiled permanent (payPicks held them back)
	}
	// a "sacrifice this" source that could tap for mana (Cathar Commando
	// under Imprisoned in the Moon)
	e := &s.bld().e
	if e.kind == seAbility && e.ab < uint8(len(k.t.Cards[e.card].Abilities)) && k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&(kop.CostReturnSelf|kop.CostExileSelf) != 0 {
		s.returnSelfDeferred(k, e)
		s.forageCastDone(k)
		return
	}
	if e.kind == seAbility && e.ab < uint8(len(k.t.Cards[e.card].Abilities)) && k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostSacSelf != 0 {
		if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer && !s.parked(e.obj) {
			s.fireSac(k, ob.slot)
			if !s.parkCost(k, e.obj, parkLeave, 0) {
				s.leave(k, ob.slot, kop.ZoneGraveyard, false)
			}
		}
	}
	s.forageCastDone(k)
}

// selfTapCounted is 1 when countUntapped counted an ability's own source
// that its {T} cost taps (so it can't also be one of the "tap N untapped
// creatures": Lathril, Blade of the Elves), else 0.
func (s *State) selfTapCounted(k *prep, a *kop.Ability, sl Slot) int {
	if a.Cost.Flags&kop.CostTap == 0 || sl == NoSlot || s.perms[sl].flags&pfTapped != 0 || !s.matchPerm(k, a.Cost.Filter, sl, s.perms[sl].ctrl, sl) {
		return 0
	}
	return 1
}

// ownActivatable: a permanent's own activated ability can be activated from
// the battlefield: not one it only grants (AfLatent: Fishing Pole's) and
// not one that works from another zone (Reassembling Skeleton's).
func ownActivatable(a *kop.Ability) bool {
	return a.AFlags&kop.AfLatent == 0 && (a.ActZone == kop.ZoneNone || a.ActZone == kop.ZoneBattlefield)
}

func (s *State) priorityMayPlay(k *prep, d *Decision, p uint8, sorc, landOK bool, av *avail) {
	pl := &s.pl[p]
	var seen objBits
	// a card a permission record already offers is listed once (gorge
	// offers one may-play cast per card: Zul Ashur's record beside
	// Muldrotha's creature static, pair 91243)
	for i := 0; i < int(s.g.nmay); i++ {
		if r := *s.mayAt(i); s.mayRecordOfferable(r, p) &&
			!ownWalkerRecord(r) &&
			!(k.gen.anyMayLimit && s.mayLimitSpent(r)) &&
			!(k.gen.anyMayWhileSrc && s.maySrcDead(r)) {
			seen.set(r.obj)
		}
	}
	// the zones some permission in force plays from (kprio): a card
	// elsewhere is asked only for its own graveyard permission
	zones := s.mayPlayZones(k, p)
	if k.gen.anyMayRaise {
		s.mayRaiseSeen(k, p, zones, &seen) // a card the raise withholds is listed as seen (cost_mayplayraise.go)
	}
	try := func(o Obj) {
		if seen.has(o) {
			return
		}
		if ob := &s.objs[o]; zones&(1<<ob.zone) == 0 &&
			(ob.zone != kop.ZoneGraveyard || ob.owner() != p || k.gen.sCls[ob.card]&sclsGyMay == 0) &&
			(ob.zone != kop.ZoneExile || ob.owner() != p || k.gen.sCls[ob.card]&sclsExMay == 0) {
			return
		}
		// the parts of every permission admitting o join into one offer; a
		// pay-life static's alternative is offered apart (mayPlayParts)
		rest, life := s.mayPlayParts(k, p, o)
		if !rest.grant && life == 0 {
			return
		}
		// pay-life statics alone offer one life alternative each (fanPayLifeAlts);
		// beside another permission only a single one is offered apart, and a
		// waived mana cost would share its mode (cast_staticpaylife.go)
		if life > payLifeAltMax || rest.grant && life > 0 && (life > 1 || rest.free) {
			s.capFull(StallPayLifeShared)
		}
		seen.set(o)
		if rest.grant {
			mode := castMayPlay
			if rest.free {
				mode |= castFree
			}
			s.offerPlay(k, d, p, o, mode, sorc, landOK, rest.anyMana, false, s.mayPlayUnpriced(k, p, o, rest.anyMana, av), av)
			if k.gen.anyZeroAlt {
				s.offerZeroAlt(k, d, p, o, mode, life == 0, sorc, av) // cast_mayzeroalt.go
			}
		}
		if life > 0 && !(rest.grant && rest.free) {
			n0 := int(d.N)
			s.offerPlay(k, d, p, o, castMayPlay|castFree, sorc, landOK, false, false, false, av) // the mana part is replaced by the life (CR 118.9)
			if life > 1 && !rest.grant && life <= payLifeAltMax {
				fanPayLifeAlts(d, n0, life)
			}
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		q := &s.pl[pp]
		for i := 0; i < int(q.graveN); i++ {
			try(q.grave[i])
		}
		for i := 0; i < int(q.exileN); i++ {
			try(q.exile[i])
		}
	}
	if zones&(1<<kop.ZoneLibrary) != 0 {
		s.tryLibTop(p, try) // cast_topoflibrary.go
	}
	if zones&(1<<kop.ZoneSideboard) != 0 && !DebugHolodeckNoSideboard {
		s.eachSideboard(p, try) // zone_sideboard.go
	}
	for i := 0; i < int(pl.handN); i++ {
		try(pl.hand[i])
	}
}

// graveOrExileAct: a printed activated ability priorityGraveAct offers, from
// the graveyard or from exile (not a granted one: priorityExileGrant's).
func graveOrExileAct(a *kop.Ability) bool {
	return a.ActZone == kop.ZoneGraveyard || a.ActZone == kop.ZoneExile && a.AFlags&kop.AfLatent == 0
}

func (s *State) priorityGraveAct(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	// activated abilities that work from the graveyard, then from exile (a
	// printed ActivationZone$ Exile ability: Torrent Elemental's; a granted
	// one is priorityExileGrant's)
	for i := 0; i < int(pl.graveN)+int(pl.exileN); i++ {
		z := kop.ZoneGraveyard
		o := Obj(0)
		if i < int(pl.graveN) {
			o = pl.grave[i]
		} else {
			z, o = kop.ZoneExile, pl.exile[i-int(pl.graveN)]
		}
		cd := &k.t.Cards[s.objs[o].card]
		for ai := range cd.Abilities {
			a := &cd.Abilities[ai]
			if a.Kind != kop.AbActivated || a.ActZone != z || a.AFlags&kop.AfLatent != 0 {
				continue
			}
			if a.Gate.Ctr == kop.CtrMaxSpeed && !s.maxSpeedGraveOK(k, p, o, a) {
				continue // kw_maxspeed.go
			}
			if a.Cost.Flags&kop.CostSorcery != 0 && !sorc {
				continue
			}
			if s.castHeldOut(o) {
				continue // CR 733.2: reversed twice in a row with no progress (Purple Pentapus), as the battlefield offer reads it
			}
			// a restricted source (Heartwood Crafter's {C}) or floating restricted
			// mana (Boommobile's activated-only units) pays an activated ability
			// from the graveyard as from the hand (CR 106.6)
			mana := &a.Cost.Mana
			var red kop.ManaCost
			if k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) {
				red = s.ownReduced(k, a, o, p) // a graveyard ability's own ReduceCost$ (cost_ownreduce.go)
				mana = &red
			}
			if z == kop.ZoneGraveyard && k.gen.anyAbReduce {
				red = s.abReducedGrave(k, a, o, p, *mana) // a graveyard-zone reduction static (Convergence of Dominion; cost_reduceability.go)
				mana = &red
			}
			gd := payDesc{class: pdAct, card: s.objs[o].card, src: NoSlot}
			if !av.affords(mana, pipMaskOf(mana), 0) &&
				!(k.restrSrc && s.restrSrcAffords(k, p, gd, mana, av, NoSlot, 0)) &&
				!s.actPoolAffordsDesc(k, p, mana, gd, av) {
				continue
			}
			if a.Trig.Cond != 0 && !s.graveActCond(k, a, p, o) {
				continue // cast_activategrave.go: Delirium / IsPresent$ PresentZone$
			}
			if a.Cost.Flags&kop.CostDiscard != 0 {
				// a filtered discard counts only the hand cards matching
				// Cost.Filter (cost_discardfiltered.go)
				dn, df, filtered := discardPart(&a.Cost)
				n := int(pl.handN)
				if filtered {
					n = s.discardCandN(k, p, df, NoObj)
				}
				if n < int(dn) {
					continue // Phantasmagorian's "Discard three cards": legal.go's battlefield check (kfix-cast)
				}
			}
			if !s.millRevealPayable(k, p, o, &a.Cost) {
				continue // cost_millcost.go: CR 701.17b's library, CR 201.5's self-reveal
			}
			if k.costObj && !s.graveActOK(k, a, p, o) {
				continue
			}
			if a.Cost.Flags&kop.CostPayEnergy != 0 && !s.energyPayable(p, &a.Cost) {
				continue // Unearth's PayEnergy<8> (CR 702.84a): costObj is not set for a pure energy cost
			}
			if f := a.Cost.Flags; f&(kop.CostSac|kop.CostTapX|kop.CostPayLife) != 0 {
				if !s.pickCostPayable(k, p, &a.Cost, o) {
					continue
				}
				if (f&kop.CostTapX != 0 || f&kop.CostSac != 0 && k.sacMana) && !s.costPartDisjoint(k, p, NoSlot, &a.Cost, mana, av, pipMaskOf(mana)) {
					continue
				}
			}
			if s.probeAbilityFrom(k, a, p, s.objs[o].card) {
				d.add(MakeAction(AkActivate, TCode(o), 0, uint8(ai)))
			}
		}
	}
}

// BuildModes is the modes already picked for the spell or ability being
// put on the stack (0 outside a build): the oracle's adapter indexes the
// remaining mode options densely, as gorge lists them. The result is a bit
// per mode chosen at least once: a repeatable charm (CanRepeatModes$) keeps a
// count per mode in the entry, which ModeSet folds to that bitmask.
func (s *State) BuildModes() uint8 {
	if s.ctl != ctlBuild {
		return 0
	}
	e := &s.bld().e
	if cards := s.k().t.Cards; int(e.card) < len(cards) && int(e.ab) < len(cards[e.card].Abilities) {
		return cards[e.card].Abilities[e.ab].ModeSet(e.mode)
	}
	return e.mode
}

// affordsFor: p can pay cost now (a ward's pay answer that can't be paid
// counters, as declining does).
func (s *State) affordsFor(k *prep, p uint8, c *kop.ManaCost) bool {
	var av avail
	s.availWindowFor(k, p, &av)
	if k.sacOnlyMana {
		s.availTappedSac(k, p, &av) // the window offers a tapped sacrifice-only source too
	}
	return av.affords(c, pipMaskOf(c), 0)
}

// tapAllForMana taps p's untapped mana sources into the pool (gorge's failed
// ward payment leaves that mana floating). A first-only filter takes its turn in
// the window's battlefield order once the pool pays its fee, for the first colour
// its ask lists (windowFiltFirst; a net change of colour, no more mana).
func (s *State) tapAllForMana(k *prep, p uint8) {
	if k.gen != nil && k.gen.anyFiltFirst {
		for s.tapAllStep(k, p) {
		}
		return
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		s.tapOneForMana(k, p, m.first())
	}
}

// tapAllStep plays the window's next activation, reporting whether there was one.
func (s *State) tapAllStep(k *prep, p uint8) bool {
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), order[:], 0)
	for _, sl := range order[:n] {
		if s.windowFiltFirstUsable(k, p, sl) {
			c := bits.TrailingZeros8(uint8(s.filtAb(k, sl).ManaColors))
			s.filtDo(k, p, filtStep{sl: sl, ans: uint8(1 + c), gap: true})
			return true
		}
		if s.tapOneForMana(k, p, sl) {
			return true
		}
	}
	return false
}

// tapOneForMana taps the source at sl into the pool, reporting whether it could.
func (s *State) tapOneForMana(k *prep, p uint8, sl Slot) bool {
	if k.gen != nil && s.anyDifferentAt(k, sl) {
		s.windowAnyDifferentTap(k, p, sl, nil, nil, false, true) // the window lists an AnyDifferent source and floats its N distinct colours
		return true
	}
	if k.gen != nil && k.gen.anyWindowGap && s.windowGap(k, p, sl) {
		var held [kop.NumManaKinds]uint8 // a restricted source's mana floats too
		s.tapGap(k, p, sl, s.windowRestr(k, p).and(slotBit(sl)), &held)
		return true
	}
	pc := s.manaPCGen(k, sl)
	sacOnly := k.sacOnlyMana && s.canSacOnlyForMana(k, sl, pc)
	// The window lists a source here only when the payment may spend it
	// (windowCanTap): a restricted grant the ward cannot spend (Mm'menon's
	// artifact blue) is no tap of gorge's failed payment.
	if !s.windowCanTap(k, sl) && !sacOnly {
		return false
	}
	mask := s.srcMaskAt(pc, sl)
	if k.painMana && pc.painCol != 0 && !pc.painOnly {
		mask |= uint8(pc.painCol) & 0x1f // the window's colour answer (windowSrcMask)
	}
	if pc.manaMulti && k.gen != nil && k.gen.anyVergeOrder {
		if c0, ok := s.vergeAbility0Col(k, sl); ok {
			mask = 1 << c0 // a Verge's ability ask answers ability 0 (mana_gate.go)
		}
	}
	// gorge's colour ask lists a source's colours in WUBRG order and
	// the failed payment takes the first (oracle payWindowAnswer)
	// A granted basic land type lists the land's own ability first (Mountain
	// under Leyline of the Guildpact: {R}, then the granted colours), and the
	// failed payment takes that one (windowCol's blgFirst).
	order := [6]int{kop.W, kop.U, kop.B, kop.R, kop.G, kop.C}
	if pc.blgFirst != 0 && int(pc.blgFirst-1) < len(order) && mask&(1<<(pc.blgFirst-1)) != 0 {
		order = [6]int{int(pc.blgFirst - 1)} // taken at once: its bit is in mask
	}
	for _, c := range order {
		if mask&(1<<c) != 0 {
			if !sacOnly {
				s.perms[sl].flags |= pfTapped
				s.touchSl(sl)
				s.manaTapped(k, sl)
			}
			s.poolAdd(p, c, int(pc.manaN))
			s.addExtraCol(k, p, sl, pc, c)
			s.artTag(k, p, sl, c)
			if k.painMana && pc.painCol != 0 && (pc.painOnly || uint8(pc.manaCol)&(1<<c) == 0) {
				s.painDamage(k, sl, p, int(pc.painN))
			}
			break
		}
	}
	if pc.manaSac || sacOnly {
		// a Treasure is sacrificed for its mana too (pair 34385)
		o := s.perms[sl].obj
		s.fireSac(k, sl)
		s.leave(k, sl, kop.ZoneGraveyard, false)
		if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
			s.detach(o) // it ceases at once, as gorge pays
		}
	}
	return true
}

// sacHostCost pays CostHostSac: sacrifice the permanent that granted the
// ability, the one candidate settled unasked.
func (s *State) sacHostCost(k *prep, e *stackEntry) {
	h := e.trigObj
	if h == NoObj || s.objs[h].zone != kop.ZoneBattlefield {
		return
	}
	sl := s.objs[h].slot
	s.snapSacPick(h)
	s.fireSac(k, sl)
	if !s.parkCost(k, h, parkLeave, 0) {
		s.leave(k, sl, kop.ZoneGraveyard, false)
	}
}

// priorityGenReadsAv reports whether priorityGen must build its own
// availability picture for p: priorityGen reads it (a play permission, a
// graveyard or hand activation, a granted ability), or availFor itself can
// stall (capFull/stall) on this table: a one-colour source census
// (addOneColour), a tap-extra rider (srcExtra), a continuous or granted mana
// ability (availGrant, manaPCGen's grantIdx), a Taps-deferred source, the
// pain/Phyrexian extension (availExt), the script census (availScript), a
// dynamic or granted-land-type source (varPC, blgOn). Without any of these,
// availFor is a pure read whose result priorityGen never uses.
func (s *State) priorityGenReadsAv(k *prep, p uint8) bool {
	g := k.gen
	return s.g.nmay != 0 || g.anyMayPlay || g.anyGraveAct || g.anyHandAct || g.blgOn || len(g.varPC) != 0 ||
		k.manaOne || g.anyTapExtra || g.anyContGrant || len(g.grantPC) != 0 || g.anyTapsDefer ||
		k.availExt || k.painMana || s.g.availX&3 != 0 ||
		g.anyGrant && s.ctrlM[p].any() && s.grantsLive(k) || g.uses.has(useGainsOf) ||
		g.uses.has(useOffKwGrant) || g.uses.has(useCyclingGrant)
}

// tgtSource is the spell or ability the current targeting is for (the entry
// being built or resolved, else the feasibility probe): spell reports a spell,
// card its card, c its colours (a source on the battlefield's current ones),
// srcSl its permanent. ok is false when no targeting is in progress.
//
// A spell probe for a card other than the resolving entry's wins over the
// entry: the cast-free ask's CR 601.2c feasibility sweep judges a graveyard
// candidate's targets while a trigger resolves, and protection reads the
// SPELL being cast (a black spell's targets), never the asking trigger
// (CR 702.16b; gorge's census judges the spell). The build's own sweep
// (cast.go) probes its spell, which reads the same as the entry's.
func (s *State) tgtSource(k *prep) (spell bool, card CardID, c kop.ColorMask, srcSl Slot, ok bool) {
	srcSl = NoSlot
	// a spell probe is the spell a feasibility check is pricing, even while
	// another entry resolves: a free cast from the resolving Sword of Once
	// and Future's trigger asks whether the candidate spell (its colours)
	// could be cast, not whether the trigger's source could target
	if s.g.probe.kind == probeSpell {
		card = s.g.probe.card
		return true, card, k.t.Cards[card].Colors, NoSlot, true
	}
	if e := s.curEntry(); e != nil {
		spell, card = e.kind == seSpell, e.card
		c = k.t.Cards[e.card].Colors
		if !spell && e.obj != NoObj && s.objs[e.obj].zone == kop.ZoneBattlefield {
			srcSl = s.objs[e.obj].slot
			c = s.colorsGen(k, srcSl)
		} else if l := s.lk(e.li); !spell && e.obj != NoObj && e.li != 0 && l.obj == e.obj {
			// a trigger of a source that has left the battlefield reads the colours
			// the source last had (CR 113.8, 608.2h, 702.16b): a Backup-granted dies
			// line on Skyship Stalker carries the granter's card, a copy's own the
			// copied card's, and the snapshot is right for both
			c = l.col
		}
		return spell, card, c, srcSl, true
	}
	switch s.g.probe.kind {
	case probeSpell:
		spell, card = true, s.g.probe.card
		c = k.t.Cards[card].Colors
	case probeAbility:
		if s.g.probe.src != NoSlot {
			srcSl = s.g.probe.src
			card = s.cardAt(srcSl)
			c = s.colorsGen(k, s.g.probe.src)
		} else if s.g.probe.off {
			card = s.g.probe.card // CR 702.16b: the card's own colours (Renew from a graveyard)
			c = k.t.Cards[card].Colors
		}
	default:
		return false, 0, 0, NoSlot, false
	}
	return spell, card, c, srcSl, true
}

// offerLandOnce adds o's land play unless d already lists it. Two permissions
// over one exiled land (Ignite the Future cast from a graveyard grants the
// plain and the free play) are one play of that land (CR 305.1), and gorge
// lists it once.
func (s *State) offerLandOnce(d *Decision, o Obj) {
	a := MakeAction(AkPlayLand, TCode(o), 0, 0)
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == a {
			return
		}
	}
	d.add(a)
}
