package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// face-down permanents and the turn-face-up special action
// (CR 708, 702.37, 702.168). Cold: a table with no Morph/Megamorph/Disguise
// card never reaches these helpers (genPrep.anyFaceDown).

// DebugIgnoreFaceDown is the lockstep's mutation switch
// (TestFacedownMutation): with it set the kernel offers neither a face-down
// cast nor a turn-face-up, so the same games diverge from gorge.
var DebugIgnoreFaceDown bool

// DebugTurnUpNoncreature is the kmanifest mutation switch
// (TestManifestMutation): with it set a manifested or cloaked noncreature
// also offers the printed mana-cost turn-up, which gorge never offers.
var DebugTurnUpNoncreature bool

// gtyFaceDown marks a face-down permanent in perm.gty. TypeMask uses bits
// 0-14 (TLand..TVehicle); bit 15 is free, so the mark costs no State bytes.
// While face down, bits 13-14 (TToken/TVehicle) instead hold the
// manifested/cloaked provenance (face_manifest.go). Every read of
// perm.gty as types masks gtyFaceDown and both provenance bits (typesAt;
// refreshTy preserves them; static_layerwalk.go#grantSrcLost masks them).
//
// The face-down characteristics ride on state the hot accessors already
// branch on, so none of them tests face-down: a face-down permanent always
// has ovGenTy set with perm.gty its derived types (TCreature at entry,
// refreshTy keeps the slot), so typesAt takes its ovGenTy branch; it is a
// walk slot (ovr.walk, with fxNeedFaceDown making the table's fxNeed
// nonzero), so typesGen, colorsGen, hasSubGen, basePTGen (the CR 613 walk,
// charsFx/basePT7a) and kwGen (l6On, kw6) derive the face-down
// characteristics in the walk; genLost reaches lostSlow through lossSlow.
// A table without a Morph-family card never sets the bit, the walk flag
// or lossSlow for it.
const gtyFaceDown kop.TypeMask = 1 << 15

// fxNeedFaceDown (genPrep.fxNeed bit 6, after fxNeedSubSet): the table
// has a Morph-family card, so a face-down permanent's walk flag is read.
const fxNeedFaceDown uint8 = 1 << 6

// prepFaceDown sets the table gates a face-down permanent needs: the walk
// (fxNeedFaceDown), genLost's slow half (lossSlow) and the CR 506.4 check
// (tyCombat: a face-down noncreature card turned face up leaves combat).
func prepFaceDown(g *genPrep) {
	if !g.anyFaceDown {
		return
	}
	g.fxNeed |= fxNeedFaceDown
	g.lossSlow = true
	g.tyCombat = true
}

// lostSlow is genLost's slow half: a face-down permanent has no abilities
// (CR 708.2), else ovrLoses. A face-down table sets genPrep.lossSlow so
// genLost's inline gate reaches here.
func (s *State) lostSlow(k *prep, sl Slot) bool {
	return s.perms[sl].gty&gtyFaceDown != 0 || s.ovrLoses(k, sl)
}

// blankCopy reports a token copy of a face-down permanent: gtyFaceDown gives it
// the face-down characteristics, but it is face up and cannot be turned (CR 707.2).
func (s *State) blankCopy(sl Slot) bool {
	return s.objs[s.perms[sl].obj].of&ofCopy != 0
}

// slotFaceDownSpell is object.slot of a face-down spell on the stack (a
// stack object's slot is otherwise NoSlot): faceDownAt reads it without a
// stack walk. pushBuild sets it; placeFrom, unpushSpell and enterNF replace
// it.
const slotFaceDownSpell Slot = NoSlot - 1

// faceDownAt reports whether o is a face-down permanent or face-down spell:
// the objects whose name, mana value and colour are hidden (CR 708.2,
// 708.4; gorge's state.Object.FaceDownHidden).
func (s *State) faceDownAt(k *prep, o Obj) bool {
	if o == NoObj {
		return false
	}
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneBattlefield:
		return s.faceDown(k, ob.slot)
	case kop.ZoneStack:
		return ob.slot == slotFaceDownSpell
	}
	return false
}

// faceDownSlots is the battlefield's face-down permanents (refreshTy keeps
// their ovGenTy types).
func (s *State) faceDownSlots() (m slotMask) {
	for o := s.occ; o.any(); o = o.rest() {
		if sl := o.first(); s.perms[sl].gty&gtyFaceDown != 0 {
			m = m.with(sl)
		}
	}
	return
}

// clearFaceDown drops the face-down mark and the ovGenTy types and walk
// flag it carried; refreshTy rederives both if an override still applies.
func (s *State) clearFaceDown(sl Slot) {
	p := &s.perms[sl]
	p.gty = 0
	p.ovr.flags &^= ovGenTy
	p.ovr.walk = 0
	s.touchSl(sl)
}

// WardAbility is the kernel-native ward trigger's ability index (abWard),
// exported for oracle checks.
const WardAbility uint8 = abWard

// PendingTriggerAmount is pending trigger i's trigger amount (ward {N} is
// trigAmt N; ward—pay N life is -N).
func (s *State) PendingTriggerAmount(i int) int16 {
	return s.pend[i].trigAmt
}

// StackTriggerAmount is stack entry i's trigger amount (ward {N} is N).
func (s *State) StackTriggerAmount(i int) int16 {
	return s.stack[i].trigAmt
}

// enterFaceDown puts o onto the battlefield face down (CR 708.2): its
// printed abilities, counters and as-enters choices do not apply, so it is
// not subscribed. Other permanents' enters replacements and triggers still
// see it (it is a 2/2 colourless creature).
func (s *State) enterFaceDown(k *prep, o Obj, ctrl uint8, fam uint8, from ...kop.Zone) Slot {
	z := kop.ZoneStack
	if len(from) != 0 {
		z = from[0]
	}
	sl := s.enterNF(k, o, ctrl, false, z, fam)
	if s.anyETB(k) {
		s.fireObj(k, kop.TrigETB, sl, o, ctrl, 0)
	}
	return sl
}

// matchCardAt is matchCard with the object the card is (NoObj when unknown).
// The generated matcher needs the object so a face-down stack spell hides
// its printed name, mana value and colour (CR 708.4).
func (s *State) matchCardAt(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj) bool {
	if k.gen != nil {
		return s.matchCardGen(k, f, c, owner, ctrl, o)
	}
	return s.matchCard(k, f, c, owner, ctrl)
}

// offerFaceDownHand offers hand card o's face-down cast when its card has a
// Morph-family keyword and gorge's spellTimingOK allows it (sorcery timing,
// an instant, or flash).
func (s *State) offerFaceDownHand(k *prep, d *Decision, p uint8, o Obj, sorc bool) {
	c := s.objs[o].card
	fam := k.t.Cards[c].MorphFamily
	if fam == 0 || !k.gen.anyFaceDown {
		return
	}
	if sorc || k.cards[c].instant || s.castFlash(k, p, c) {
		s.offerFaceDown(k, d, p, o, fam)
	}
}

// offerFaceDown offers a hand card's face-down cast for {3}. The card is a
// permanent card with a Morph-family keyword; a land is offered even after
// the land drop is used (CR 702.37b: casting it face down is not playing a
// land). The caller applies the cast's timing gate (gorge's spellTimingOK:
// sorcery timing, an instant, or flash).
//
// The offer is priced from the floating pool alone, matching gorge's real
// offer walk: gorge's planner does not price a face-down cast
// (PotentialPaymentPlans calls the mode an unsupported shape), so a player
// must float the {3} first. Offering it from untapped sources would let the
// kernel cast what the reference engine does not offer.
func (s *State) offerFaceDown(k *prep, d *Decision, p uint8, o Obj, fam uint8) {
	if DebugIgnoreFaceDown {
		return
	}
	c := s.objs[o].card
	if k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return
	}
	cost := kop.ManaCost{Generic: 3}
	if s.statM.any() || k.gen != nil {
		cost = s.adjustCost(k, p, c, o, cost)
	}
	if !s.poolPays(p, &cost) {
		return
	}
	d.add(MakeAction(AkCast, TCode(o), 0, castFaceDown))
}

// offerTurnFaceUp offers the turn-face-up special action (CR 708.6): a
// face-down permanent you control whose Morph cost the floating pool pays.
// It is not a spell or ability and uses no stack.
func (s *State) offerTurnFaceUp(k *prep, d *Decision, p uint8) {
	if DebugIgnoreFaceDown {
		return
	}
	m := s.ctrlM[p].and(s.occ)
	for m.any() {
		sl := m.first()
		m = m.rest()
		if k.gen.anyTurnUpRepl && s.faceDown(k, sl) && s.turnUpBlocked(k, sl) {
			if DebugTurnUpReplStats {
				var tmp Decision
				s.offerTurnUpSl(k, &tmp, p, sl)
				if tmp.N != 0 {
					TurnUpBlockedHeld++
					switch turnUpWhy {
					case 1:
						TurnUpHeldCond++
					case 2:
						TurnUpHeldPlain++
					}
				}
			}
			continue // face_turnfaceup.go: gorge offers nothing
		}
		s.offerTurnUpSl(k, d, p, sl)
	}
}

// turnUpSpecial is the CR 116.2b special action a turn-up procedure pays as
// (gorge morphFaceUp.special / pay.SpecialHasProperty): the printed Morph and
// Megamorph keywords are MorphUp, a manifested card is ManifestUp, a cloaked
// one CloakUp, and its printed Disguise alternative is DisguiseUp.
func turnUpSpecial(mode uint8, family uint8) kop.SpecialMask {
	switch mode {
	case 1:
		return kop.SpmManifestUp
	case 2:
		return kop.SpmCloakUp
	case 3, 4:
		return kop.SpmMorphUp
	case 5:
		return kop.SpmDisguiseUp
	case 0:
		if family == kop.MorphDisguise {
			return kop.SpmDisguiseUp
		}
		return kop.SpmMorphUp
	}
	return 0
}

// offerTurnUpSl offers the turn-face-up action of the permanent at sl, if it
// is face down and the floating pool pays for it.
func (s *State) offerTurnUpSl(k *prep, d *Decision, p uint8, sl Slot) {
	pm := &s.perms[sl]
	cd := &k.t.Cards[s.cardAt(sl)]
	if !s.faceDown(k, sl) || s.blankCopy(sl) {
		return
	}
	if s.manifested(sl) || s.cloaked(sl) {
		// CR 701.40b/701.58b: a manifested or cloaked CREATURE CARD
		// turns up for its printed mana cost; CR 701.40c-d/701.58c-d:
		// a printed Morph/Megamorph/Disguise is an alternative.
		if (cd.Types&kop.TCreature != 0 || DebugTurnUpNoncreature) && !cd.CostEmpty {
			cost := cd.Cost
			mode := uint8(1)
			if s.cloaked(sl) {
				mode = 2
			}
			if turnUpHybOK(&cost) && s.specialAffords(k, p, &cost, turnUpSpecial(mode, cd.MorphFamily), sl, 0) {
				d.add(MakeAction(AkTurnFaceUp, TCode(pm.obj), 0, mode))
			}
		}
		if fam := cd.MorphFamily; fam != 0 {
			cost := s.morphCostAt(k, sl)
			if turnUpHybOK(&cost) && s.turnUpOffered(k, p, &cost, turnUpSpecial(fam+2, cd.MorphFamily), sl, true) {
				d.add(MakeAction(AkTurnFaceUp, TCode(pm.obj), 0, fam+2))
			}
		}
		return
	}
	if cd.MorphFamily == 0 {
		return
	}
	cost := s.morphCostAt(k, sl)
	if !turnUpHybOK(&cost) || !s.turnUpOffered(k, p, &cost, turnUpSpecial(0, cd.MorphFamily), sl, true) {
		return
	}
	d.add(MakeAction(AkTurnFaceUp, TCode(pm.obj), 0, 0))
}

// turnFaceUp performs the special action: pay the Morph cost from the
// floating pool, clear the face-down characteristics, restore the card's
// triggers and statics, refresh type-changing effects, fire TurnFaceUp,
// then put the Megamorph counter. A Disguise permanent simply turns face
// up; its ward is read from the family bits while it is face down.
func (s *State) turnFaceUp(k *prep, o Obj, p uint8, proc ...uint8) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	sl := ob.slot
	pm := &s.perms[sl]
	if pm.ctrl != p || !s.faceDown(k, sl) {
		return
	}
	if k.gen.anyTurnUpRepl && s.turnUpBlocked(k, sl) {
		return // face_turnfaceup.go: the action is illegal, nothing is paid
	}
	mode := uint8(0)
	if len(proc) != 0 {
		mode = proc[0]
	}
	cost, _, kw, ok := s.turnUpCost(k, sl, mode)
	if !ok {
		return
	}
	sp := turnUpSpecial(mode, k.t.Cards[s.cardAt(sl)].MorphFamily)
	if !turnUpHybOK(&cost) || !s.turnUpOffered(k, p, &cost, sp, sl, kw) {
		return
	}
	if kw && cost.X > 0 {
		// CR 107.3m / 702.37: the keyword's {X} is announced first (gorge
		// turnUpXAsk); the answer settles the turn-up (applyTurnUpX)
		s.g.pickP, s.g.pickZ, s.g.pickF = p, mode, uint16(o)
		s.g.pickLeft = uint8(s.turnUpMaxX(k, p, &cost, sp, sl))
		s.ctl = ctlTurnUpX
		return
	}
	s.settleTurnUp(k, o, p, mode, 0)
}

// turnUpCost prices the turn-up procedure mode at sl (the offer and the
// payment price the same way). kw: the printed Morph-family keyword, whose
// {X} is announced; a manifested/cloaked card's printed cost never is
// (CR 701.40b).
func (s *State) turnUpCost(k *prep, sl Slot, mode uint8) (cost kop.ManaCost, megamorph, kw, ok bool) {
	cd := &k.t.Cards[s.cardAt(sl)]
	prov := s.manifested(sl) || s.cloaked(sl)
	switch mode {
	case 0: // the cast family's own keyword (CR 702.37e, 702.168c, 702.169b)
		if prov || cd.MorphFamily == 0 {
			return
		}
		return s.morphCostAt(k, sl), cd.MorphFamily == kop.MorphMegamorph, true, true
	case 1, 2: // manifest/cloak: the printed mana cost (CR 701.40b, 701.58b)
		if !prov || (mode == 1) != s.manifested(sl) {
			return
		}
		if (cd.Types&kop.TCreature == 0 && !DebugTurnUpNoncreature) || cd.CostEmpty {
			return
		}
		return cd.Cost, false, false, true
	case 3, 4, 5: // the keyword alternative (CR 701.40c-d, 701.58c-d)
		fam := mode - 2
		if !prov || cd.MorphFamily != fam {
			return
		}
		return s.morphCostAt(k, sl), fam == kop.MorphMegamorph, true, true
	}
	return
}

// settleTurnUp pays the priced turn-up at X = x and turns the permanent up;
// a stale request (the permanent changed, the pool no longer pays) is a no-op.
func (s *State) settleTurnUp(k *prep, o Obj, p uint8, mode uint8, x int) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	sl := ob.slot
	pm := &s.perms[sl]
	if pm.ctrl != p || !s.faceDown(k, sl) {
		return
	}
	cost, megamorph, kw, ok := s.turnUpCost(k, sl, mode)
	if !ok || !turnUpHybOK(&cost) {
		return
	}
	sp := turnUpSpecial(mode, k.t.Cards[s.cardAt(sl)].MorphFamily)
	if !s.specialAffords(k, p, &cost, sp, sl, x) || kw && !s.turnUpLifeOK(k, p, sl) {
		return
	}
	cost = cost.AtX(x)
	if cost.Hyb[0] != 0 {
		cost = s.turnUpHybResolve(p, cost, 0) // face_turnfaceup.go
	}
	s.paySpecial(k, p, &cost, sp, sl, x)
	if kw {
		s.payTurnUpLife(k, p, sl) // face_morphlife.go
	}
	s.turnUpFlip(k, sl, o, p, uint8(x))
	if megamorph {
		s.g.lastCK = kop.CtrP1P1
		n := s.placeCounters(k, sl, int(kop.CtrP1P1), 1, pm.ctrl, ctrPutEffect)
		if n > 0 && k.anyCtrAdded {
			s.ctrNote(k, sl, int(kop.CtrP1P1))
		}
		if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			s.fireObj(k, kop.TrigCounterAddedOnce, sl, pm.obj, pm.ctrl, int16(n))
		}
	}
	s.passes = 0
}

// turnUpFlip turns the face-down permanent at sl face up once any cost is
// paid (the special action's settleTurnUp and an effect's turn-up,
// face_turnupeffect.go): its turn-up replacements, the face-up
// characteristics and subscriptions, then the TurnFaceUp triggers.
func (s *State) turnUpFlip(k *prep, sl Slot, o Obj, p uint8, x uint8) {
	c := s.cardAt(sl)
	if k.gen.anyTurnUpRepl {
		s.turnUpRepls(k, sl, o) // face_turnfaceup.go: the body runs while still face down
	}
	s.clearFaceDown(sl) // marks the slot changed (marks.go: every mutation marks)
	s.subscribe(sl, &k.cards[c], true)
	s.refreshTy(k)
	s.fireTurnFaceUp(k, sl, o, p, x)
}

// fireTurnFaceUp fires "when <filter> turns face up" (gorge Mode$
// TurnFaceUp). No slot subscription: scan the battlefield as for a gift or
// a monstrosity. The turned permanent has already cleared its face-down
// flags and been subscribed, so its own printed trigger is seen.
//
//go:noinline
func (s *State) fireTurnFaceUp(k *prep, sl Slot, o Obj, p uint8, x uint8) {
	if !k.gen.anyFaceDown {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, ls := range ord[:n] {
		s.slotTriggers(k, ls, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigTurnFaceUp }, func(l trigListener, a *kop.Ability) {
			s.objTrigGen(k, kop.TrigTurnFaceUp, l, a, sl, o, p, int16(x))
		})
	}
}
