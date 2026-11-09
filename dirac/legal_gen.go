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
	for i := 0; i < int(s.g.nmay); i++ {
		r := s.g.may[i]
		ob := &s.objs[r.obj]
		if r.ctrl != p || ob.ver != r.ver || (ob.zone != kop.ZoneExile && ob.zone != kop.ZoneGraveyard) {
			continue
		}
		if k.gen.anyMayWhileSrc && s.maySrcDead(r) {
			continue // its source left the battlefield (cast_whilesource.go)
		}
		if ownWalkerRecord(r) {
			continue // a warp recast / plotted cast is offered only by its walker (kw_warp.go, kw_plot.go)
		}
		if k.gen.anyMayKind && r.kind != 0 {
			if k.gen.anyMayLimit && s.mayLimitSpent(r) {
				continue // the grant's once-per-turn cap is reached (Flameskull)
			}
			s.offerMayKind(k, d, p, r, sorc, landOK, &av)
			continue
		}
		s.offerPlay(k, d, p, r.obj, castMayPlay, sorc, landOK, false, false, false, &av)
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
			if host == NoSlot || a.Cost.Flags&kop.CostTapHost != 0 && s.perms[host].flags&pfTapped != 0 {
				return
			}
			if a.Cost.Host == kop.CostHostSac && s.perms[host].ctrl != p {
				return // only a permanent the payer controls can be sacrificed (CR 701.21a)
			}
			if s.cfg&cfgGorgeUnpriced != 0 && !s.poolPays(p, &a.Cost.Mana) {
				return // gorge offers it only when the pool pays (Config.GorgeUnpriced)
			}
			if s.canActivate(k, sl, 255, a, sorc, &av) {
				// two copies of one grantor give the same ability: the second is the
				// first's option (gorge's offer list collapses them the same way)
				act := MakeAction(AkActivate, uint8(s.perms[sl].obj), uint8(s.perms[host].obj), 0x80|ab)
				for i := first; i < int(d.N); i++ {
					if d.Options[i] == act {
						return
					}
				}
				d.add(act)
			}
		})
	}
}

// offerPlay offers playing card object o through a permission.
func (s *State) offerPlay(k *prep, d *Decision, p uint8, o Obj, mode uint8, sorc, landOK, anyMana, anyColor, unpriced bool, av *avail) {
	// the avail is the caller's walk picture: the cast's own delve credit
	// prices its twobrid faces and is restored after, so the next option does
	// not price with it
	saved := av.delve
	defer func() { av.delve = saved }()
	c := s.objs[o].card
	pc := &k.cards[c]
	anyMana = anyMana || mode&castFree == 0 && s.convAnyType(k, p, o) // mana_convertcreature.go
	if pc.land {
		if landOK {
			d.add(MakeAction(AkPlayLand, uint8(o), 0, 0))
		}
		return
	}
	if !pc.instant && !sorc && !s.castFlash(k, p, c) {
		return
	}
	if k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return
	}
	if mode&castFree == 0 {
		base := pc.cost
		if k.gen.anyConvoke && s.tapPays(k, p, c, o, false) {
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
		if anyColor {
			if !av.affordsAnyColor(&cost, 0) {
				return
			}
		} else if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) {
			return
		}
	} else if s.freeOwes(k) {
		// a free cast still pays the cost statics' increases (CR 601.2f, 118.9d)
		cost := s.adjustCostGen(k, p, c, o, kop.ManaCost{})
		if cost != (kop.ManaCost{}) && (!s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) || !s.windowPoolPays(p, &cost, false)) {
			return // (a free cast is no potential play: the floating pool pays what it owes)
		}
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
		return
	}
	if mode&castFree != 0 && !s.altAddCastableFrom(k, c, p, o, &kop.ManaCost{}, &kop.ManaCost{}, true, anyMana, av) {
		return // a free cast still owes its either-or additional cost (CR 601.2b, 118.9d): Mudbutton Cursetosser's behold or {2}
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
		offers := s.gorgeOffersMay(p, o)
		s.g.mayPlayAny = false
		if !s.altAddCastableFrom(k, c, p, o, &altCost, &pc.cost, true, anyMana, av) || !offers || s.castHeldOut(o) {
			return
		}
		if unpriced && !s.windowPoolPays(p, &cost, anyColor) && (s.g.availX&(availAny|availGaps) == 0 || (!anyMana && !anyColor)) {
			// (the potential walks list an any-type one: gorge's planner
			// proves it insufficient, and a scripted prefix may make the
			// pool pay it; an "ambiguous" one is never searched)
			return
		}
	}
	d.add(MakeAction(AkCast, uint8(o), 0, mode))
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
func hybridGeneric(c kop.ManaCost) kop.ManaCost {
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
func (s *State) castFlagsGen(k *prep, o Obj, mode uint8) {
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
	// a permission record is spent; a once-per-turn static is marked
	if k.gen.anyMayKind && s.spendMayKind(k, o, p, mode) {
		return
	}
	if s.spendMayRecord(k, o, p) {
		return
	}
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
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
		if st.a.Static.StFlags&kop.StOncePerTurn != 0 && st.sl != NoSlot && !s.mayPlayFolds(k, p, o) {
			s.markUsed(st.obj, uint8(abIndex(k, s.cardAt(st.sl), st.a)))
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
		if s.g.may[i].obj == o && s.g.may[i].ctrl == p && s.g.may[i].ver == s.objs[o].ver &&
			!ownWalkerRecord(s.g.may[i]) && !s.mayLimitSpent(s.g.may[i]) && !(k.gen.anyMayWhileSrc && s.maySrcDead(s.g.may[i])) {
			s.removeMay(i)
			return true
		}
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
		for c := 0; c < int(kop.NumCounterKinds3); c++ {
			if !kop.IsCounterKind(c) {
				continue // CtrLevel is no counter (kw_class.go)
			}
			n += p.ctrN(s, c)
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
	if s.bld().altAdd == buildAltCost {
		return s.altCostOf(k), false, true // the card's own alternative mana cost (cast_altfree.go)
	}
	if e.flags&sfMayPlay != 0 {
		cd := &k.t.Cards[e.card]
		c := cd.Cost
		if e.flags&sfKicked != 0 {
			c = addCost(c, cd.Kicker)
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
	if a.Mode()&0x80 != 0 {
		host := Obj(a.Target())
		hc := s.objs[host].card
		if s.objs[host].zone == kop.ZoneBattlefield {
			hc = s.cardAt(s.objs[host].slot)
		}
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: hc, ab: a.Mode() &^ 0x80, ctrl: s.prio, trigObj: host}}
	} else if ob.zone != kop.ZoneBattlefield {
		*s.bld() = build{e: stackEntry{kind: seAbility, obj: o, srcVer: ob.ver, card: ob.card, ab: a.Mode(), ctrl: s.prio, trigObj: NoObj}}
	} else {
		return false
	}
	s.bld().e.tgt = noTargets
	s.bld().stage = stageFirst[seAbility]
	s.ctl = ctlBuild
	return true
}

// tapHostGen pays CostTapHost: tap the permanent granting the ability.
func (s *State) tapHostGen(k *prep, e *stackEntry) {
	h := e.trigObj
	if h != NoObj && s.objs[h].zone == kop.ZoneBattlefield {
		was := s.perms[s.objs[h].slot].flags & pfTapped
		s.perms[s.objs[h].slot].flags |= pfTapped
		s.touch(h)
		if was == 0 && s.subs[kop.TrigTaps].any() {
			s.fireTaps(k, s.objs[h].slot)
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
	if k.gen.anyCantTgt && s.cantTargetGen(k, sl, p) {
		return false // a printed CantTarget static (tgt_canttarget.go)
	}
	if kw&(kop.KwHexproofFromInstants|kop.KwProtBlack|kop.KwProtRed) == 0 && !(k.gen.anyProtCol && s.protPrinted(k, sl) != 0) &&
		!(k.gen.anyProtF && k.t.Cards[s.cardAt(sl)].ProtFrom != 0) && hexC == 0 {
		return true
	}
	var spell bool
	var card CardID
	var c kop.ColorMask
	srcSl := NoSlot
	if e := s.curEntry(); e != nil {
		spell, card = e.kind == seSpell, e.card
		c = k.t.Cards[e.card].Colors
		if !spell && e.obj != NoObj && s.objs[e.obj].zone == kop.ZoneBattlefield {
			srcSl = s.objs[e.obj].slot
			c = s.colorsGen(k, srcSl)
		}
	} else {
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
			return true
		}
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
	if k.gen.anyProtF && (srcSl != NoSlot || spell || s.curEntry() != nil) && s.protQuality(k, sl, srcSl, card) {
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
// card c that works off the battlefield (Renew from a graveyard): its source
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
	for i := 0; i < int(s.bld().pickN); i++ {
		o := s.bld().picks[i]
		if s.bld().pickKs[i] == pkSac && s.objs[o].zone == kop.ZoneBattlefield {
			s.snapSacPick(o) // RefSacrificed (CR 707.10: a copy keeps it)
			s.fireSac(k, s.objs[o].slot)
			if !s.parkCost(k, o, parkLeave, 0) {
				s.leave(k, s.objs[o].slot, kop.ZoneGraveyard, false)
			}
		}
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
		if r := s.g.may[i]; r.ctrl == p && s.objs[r.obj].ver == r.ver &&
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
		// the parts of every permission admitting o join into one offer: a
		// free-only static (Weftwalking) opens no zone but waives the cost of
		// the cast another static grants (CR 118.9)
		grant, free, anyMana := false, false, false
		nst, payLife := 0, false
		s.mayPlayStatics(k, p, o, func(st statSrc) bool {
			fl := st.a.Static.StFlags
			if fl&kop.StRemove6Counters != 0 && s.creatureCounters(k, p) < int(st.a.Static.A) {
				return true // the extra cost can't be paid (Quilled Greatwurm)
			}
			if fl&kop.StMayPayLife != 0 {
				if !s.payLifeOfferable(k, p, o) {
					return true // the mana value in life can't be paid (cast_staticpaylife.go)
				}
				free, payLife = true, true // the mana part is replaced by the life
			}
			nst++
			if fl&kop.StFreeOnly == 0 || s.objs[o].zone == kop.ZoneHand {
				grant = true // a free-only static opens no zone; a hand cast is already legal
			}
			free = free || fl&kop.StWithoutManaCost != 0
			anyMana = anyMana || fl&kop.StAnyManaType != 0
			return true
		})
		if !grant {
			return
		}
		if payLife && nst > 1 {
			s.capFull(StallPayLifeShared) // gorge offers the plain cast and the life alternative apart (cast_staticpaylife.go)
		}
		seen.set(o)
		mode := castMayPlay
		if free {
			mode |= castFree
		}
		// Config.GorgeUnpriced: gorge's planner funds an any-type cast's pips
		// as generic where it plans the shape (Tinybones), and prices no cast
		// two permissions offer (Muldrotha's per-type statics for an artifact
		// creature: "ambiguous"); gorge lists those only when the pool pays
		unpriced := false
		if s.cfg&cfgGorgeUnpriced != 0 {
			if anyMana {
				cost := s.adjustCostGen(k, p, s.objs[o].card, o, k.cards[s.objs[o].card].cost)
				cost = planUnderRider(cost, true, false)                                           // a hybrid pip is generic to the plan too (pay.PlanManaUnderRider)
				unpriced = !s.castAffordsR(k, p, s.objs[o].card, s.restrOriginOfObj(o), &cost, av) // (the restricted sources the planner counts price it too)
			} else {
				// Two offers are ambiguous to gorge's potential planner,
				// but its payment builder still plans the untyped
				// (MayPlayPerm=="") offer from a graveyard or exile
				// (payment_plan.go plannedMayPlayZone), so that cast is
				// priced rather than ambiguous (Storied Veteran beside
				// The Holodeck). A top-of-library cast has no such plan
				// (Boltwave beside Madame Web and Johann).
				unpriced = s.mayPlayOffers(k, p, o) >= 2 &&
					!(s.mayPlayFolds(k, p, o) && mayPlayPlannedZone(s.objs[o].zone))
			}
		}
		s.offerPlay(k, d, p, o, mode, sorc, landOK, anyMana, false, unpriced, av)
	}
	for pp := uint8(0); pp < 2; pp++ {
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

func (s *State) priorityGraveAct(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	// activated abilities that work from the graveyard
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		cd := &k.t.Cards[s.objs[o].card]
		for ai := range cd.Abilities {
			a := &cd.Abilities[ai]
			if a.Kind != kop.AbActivated || a.ActZone != kop.ZoneGraveyard {
				continue
			}
			if a.Gate.Ctr == kop.CtrMaxSpeed && !s.maxSpeedGraveOK(p, a) {
				continue // kw_maxspeed.go
			}
			if a.Cost.Flags&kop.CostSorcery != 0 && !sorc {
				continue
			}
			// a restricted source (Heartwood Crafter's {C}) pays an activated
			// ability from the graveyard as from the hand (CR 106.6)
			if !av.affords(&a.Cost.Mana, pipMaskOf(&a.Cost.Mana), 0) &&
				!(k.restrSrc && s.restrSrcAffords(k, p, payDesc{class: pdAct, card: s.objs[o].card, src: NoSlot}, &a.Cost.Mana, av, NoSlot, 0)) {
				continue
			}
			if a.Trig.Cond != 0 && !s.graveActCond(k, a, p, o) {
				continue // cast_activategrave.go: Delirium / IsPresent$ PresentZone$
			}
			if a.Cost.Flags&kop.CostDiscard != 0 {
				// a filtered discard counts only the hand cards matching
				// Cost.Filter (cost_discardfiltered.go)
				n := int(pl.handN)
				if a.Cost.Flags&kop.CostDiscardFiltered != 0 {
					n = s.discardCandN(k, p, a.Cost.Filter, NoObj)
				}
				if n < int(a.Cost.N) {
					continue // Phantasmagorian's "Discard three cards": legal.go's battlefield check (kfix-cast)
				}
			}
			if k.costObj && !s.graveActOK(k, a, p, o) {
				continue
			}
			if f := a.Cost.Flags; f&(kop.CostSac|kop.CostTapX|kop.CostPayLife) != 0 {
				if !s.pickCostPayable(k, p, &a.Cost, o) {
					continue
				}
				if (f&kop.CostTapX != 0 || f&kop.CostSac != 0 && k.sacMana) && !s.costPartDisjoint(k, p, NoSlot, &a.Cost, &a.Cost.Mana, av, pipMaskOf(&a.Cost.Mana)) {
					continue
				}
			}
			if s.probeAbilityFrom(k, a, p, s.objs[o].card) {
				d.add(MakeAction(AkActivate, uint8(o), 0, uint8(ai)))
			}
		}
	}
}

// BuildModes is the modes already picked for the spell or ability being
// put on the stack (0 outside a build): the oracle's adapter indexes the
// remaining mode options densely, as gorge lists them.
func (s *State) BuildModes() uint8 {
	if s.ctl != ctlBuild {
		return 0
	}
	return s.bld().e.mode
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
			s.pl[p].pool[c] += pc.manaN
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
		g.anyGrant && s.ctrlM[p].any() && s.grantsLive(k)
}
