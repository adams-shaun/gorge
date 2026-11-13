package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Filter matching on a generated table: every kop.Filter field, the Or
// alternative chain, and the kgen predicates and flags.

// Layers a filter may be evaluated at: a static that changes a
// characteristic reads the others but not the one it changes.
const (
	layerFull uint8 = iota
	layerPT         // a P/T static: keywords and types are final
	layerKw         // a keyword static: keyword reads use printed keywords
	layerSub        // a subtype static: subtype reads use printed subtypes
)

// curEntry is the stack entry whose filters are being evaluated: the
// resolving one, else the one being built (nil outside both).
func (s *State) curEntry() *stackEntry {
	if s.res.active {
		return &s.res.e
	}
	if s.ctl == ctlBuild {
		return &s.bld().e
	}
	return nil
}

func (s *State) inSet(set *[maxRem]Obj, n uint8, o Obj) bool {
	for i := 0; i < int(n); i++ {
		if set[i] == o {
			return true
		}
	}
	return false
}

// matchPermGen reports whether battlefield slot sl matches filter f (any
// alternative of its chain) relative to a source controlled by ctrl at
// slot src.
func (s *State) matchPermGen(k *prep, f kop.FilterID, sl Slot, ctrl uint8, src Slot, layer uint8) bool {
	for f != 0 {
		if s.matchPermOne(k, &k.t.Filters[f], sl, ctrl, src, layer) {
			return true
		}
		f = k.t.Filters[f].Or
	}
	return false
}

func (s *State) matchPermOne(k *prep, fl *kop.Filter, sl Slot, ctrl uint8, src Slot, layer uint8) bool {
	p := &s.perms[sl]
	switch fl.Ctrl {
	case kop.CtrlYou:
		if p.ctrl != ctrl {
			return false
		}
	case kop.CtrlOpp:
		if p.ctrl == ctrl {
			return false
		}
	case kop.CtrlEachPl:
		if !s.ctrlEachPl(p.ctrl, ctrl) {
			return false
		}
	case kop.CtrlHitBySrc:
		if !s.combatHitBySrc(p.ctrl) {
			return false // value_turntallies.go
		}
	}
	if fl.CtrlTgt != 0 && !s.ctrlRoleTgt(k, int(fl.CtrlTgt)-1, p.ctrl) {
		return false // filter_heads.go: ControlledBy TargetedOrController
	}
	switch fl.Owner {
	case kop.CtrlYou:
		if s.objs[p.obj].owner() != ctrl {
			return false
		}
	case kop.CtrlOpp:
		if s.objs[p.obj].owner() == ctrl {
			return false
		}
	}
	if fl.Flags != 0 {
		if fl.Flags&kop.FOther != 0 && sl == src {
			return false
		}
		if fl.Flags&kop.FSelf != 0 && sl != src {
			return false
		}
		if fl.Flags&kop.FAttachedBySrc != 0 && (src == NoSlot || s.perms[src].attached != sl) {
			return false
		}
		if fl.Flags&kop.FNotAttachedBySrc != 0 && src != NoSlot && (s.perms[src].attached == sl) != DebugNotAttachedPositive {
			return false // filter_notattached.go
		}
		if fl.Flags&kop.FExiledWith != 0 && !s.exwPermFlag(p.obj, src) {
			return false // zone_exiledwithstatic.go
		}
		tok := s.objs[p.obj].of&ofToken != 0
		if fl.Flags&kop.FNontoken != 0 && tok || fl.Flags&kop.FToken != 0 && !tok {
			return false
		}
		if fl.Flags&kop.FTapped != 0 && p.flags&pfTapped == 0 || fl.Flags&kop.FUntapped != 0 && p.flags&pfTapped != 0 {
			return false
		}
		if fl.Flags&kop.FAttacking != 0 && p.flags&pfAttacking == 0 || fl.Flags&kop.FNotAttacking != 0 && p.flags&pfAttacking != 0 {
			return false
		}
		if fl.Flags&kop.FBlocking != 0 && p.flags&pfBlocking == 0 {
			return false
		}
		if fl.Flags&kop.FAttackingOrBlocking != 0 && p.flags&(pfAttacking|pfBlocking) == 0 {
			return false
		}
	}
	if fl.Types != 0 || fl.NotTypes != 0 {
		t := s.typesGen(k, sl)
		if fl.Types != 0 && t&fl.Types == 0 || t&fl.NotTypes != 0 {
			return false
		}
	}
	if fl.Colors != 0 || fl.NotColors != 0 {
		c := s.colorsGen(k, sl)
		if fl.Colors != 0 && c&fl.Colors == 0 || c&fl.NotColors != 0 {
			return false
		}
	}
	if fl.Sub != 0 || fl.NotSub != 0 {
		has := func(i uint8) bool {
			if s.faceDown(k, sl) {
				return false // CR 708.2: no subtypes
			}
			if layer == layerSub {
				return int(i) < len(k.t.Subtypes) && k.t.Cards[s.cardAt(sl)].Subtypes.Has(int(i))
			}
			return s.hasSubGen(k, sl, i)
		}
		if fl.Sub != 0 && !has(fl.Sub-1) || fl.NotSub != 0 && has(fl.NotSub-1) {
			return false
		}
		for _, n := range fl.NotSubs {
			if n != 0 && has(n-1) {
				return false
			}
		}
	}
	if fl.Flags2 != 0 && !s.flags2Perm(k, fl.Flags2, sl, src, layer) {
		return false
	}
	if fl.Pred != kop.PredNone {
		if k.w8 && fl.Pred == kop.PredWord7 && fl.PredN&0xff >= kop.W7FaceUp {
			return s.wordPerm2(k, fl, sl, ctrl, src)
		}
		if fl.Pred == kop.PredAttached {
			return s.predAttached(sl, src)
		}
		if (fl.Pred == kop.PredPowerGEV || fl.Pred == kop.PredToughnessLEV && fl.PredN == 3) && src != NoSlot {
			return s.powerGEVFrom(k, fl, sl, src) // filter_typesall.go
		}
		return s.predPermGen(k, fl, sl, layer)
	}
	return true
}

// sharesColorRem: gorge sharesColorWithOther over the remembered set -- o
// (colours mine, its printed face: a milled card) shares a colour with a
// remembered object other than itself, read from its printed face too.
// Colourless shares nothing.
func (s *State) sharesColorRem(k *prep, o Obj, mine kop.ColorMask) bool {
	if mine == 0 {
		return false
	}
	for i := 0; i < int(s.g.nrem); i++ {
		r := s.g.rem[i]
		if r != o && k.t.Cards[s.objs[r].card].Colors&mine != 0 {
			return true
		}
	}
	return false
}

// sharesNameRem: gorge sharesNameWith Remembered -- the object shares a
// printed name with a remembered object other than itself. o is the object
// (NoObj for a card read only by its CardID c); a face-down object has no
// name (CR 708.2). The remembered set holds objects only, and an object on
// the battlefield reads its layer-3 name (a rename/copy counts, CR 707.2),
// otherwise its card.
func (s *State) sharesNameRem(o Obj, c CardID, fd bool) bool {
	nm := c
	if o != NoObj {
		if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
			nm = s.nameCardAt(ob.slot)
		} else {
			nm = ob.card
		}
	}
	if fd || nm == NoCard {
		return false
	}
	for i := 0; i < int(s.g.nrem); i++ {
		r := s.g.rem[i]
		if r == o {
			continue
		}
		var rn CardID
		if s.objs[r].zone == kop.ZoneBattlefield {
			rn = s.nameCardAt(s.objs[r].slot)
		} else {
			rn = s.objs[r].card
		}
		if rn != NoCard && rn == nm {
			return true
		}
	}
	return false
}

func (s *State) flags2Perm(k *prep, f kop.FilterFlags2, sl Slot, src Slot, layer uint8) bool {
	p := &s.perms[sl]
	o := p.obj
	if f&(kop.F2Emblem|kop.F2SharesColorRem|kop.F2Unblocked) != 0 {
		// F2SharesColorRem is lowered only for the milled cards of a
		// repeat gate; on the battlefield it fails closed. F2Unblocked is
		// gorge's "unblocked": attacking with no blocker recorded (a
		// blocker that left keeps pfBlocked, gorge's BlockedBy tombstone);
		// it shares this test so a filter without it pays nothing.
		if f&(kop.F2Emblem|kop.F2SharesColorRem) != 0 || p.flags&(pfAttacking|pfBlocked) != pfAttacking {
			return false
		}
	}
	if f&kop.F2Kicked != 0 && !s.mSl(k, mechKicked, sl) {
		return false
	}
	if f&kop.F2Sneaked != 0 && !s.mSl(k, mechSneaked, sl) {
		return false
	}
	if f&kop.F2CastByYou != 0 && !s.g.cast.has(sl) {
		return false
	}
	if f&kop.F2EnteredThisTurn != 0 && !s.g.zoneTurn.has(o) {
		return false
	}
	if f&kop.F2Saddled != 0 && p.cseen&cseenSaddled == 0 {
		return false
	}
	if f&(kop.F2BlockedBySrc|kop.F2DamagedBy|kop.F2CrewedSrc|kop.F2Warped) != 0 && !s.coldF2Perm(k, f, sl, src) {
		return false // filter_combatwords.go, trig_gravedrawn.go
	}
	if f&(kop.F2TgtPlayerCtrl|kop.F2DefendingCtrl) != 0 && !s.ctrlRoleF2(k, f, p.ctrl) {
		return false // filter_heads.go
	}
	if f&(kop.F2Basic|kop.F2Legendary) != 0 && !s.superGen(k, f, sl) {
		return false
	}
	if f&(kop.F2WithFlying|kop.F2WithoutFlying) != 0 {
		var kw kop.KwMask
		if layer == layerKw {
			if !s.faceDown(k, sl) {
				kw = k.cards[s.cardAt(sl)].kw // CR 708.2: a face-down permanent has none
			}
		} else {
			kw = s.kwGen(k, sl)
		}
		fly := kw&kop.KwFlying != 0
		if f&kop.F2WithFlying != 0 && !fly || f&kop.F2WithoutFlying != 0 && fly {
			return false
		}
	}
	if f&(kop.F2ChosenType|kop.F2ChosenColor|kop.F2SameNameAsSource) != 0 {
		ch, ok := s.chosenTypeSub(src)
		if f&kop.F2ChosenType != 0 && (!ok || !s.hasSubGen(k, sl, ch) && !(outsideSub(k, ch) && s.outsideChosenHas(k, sl, src))) {
			return false
		}
		if f&kop.F2ChosenColor != 0 {
			if c, ok := s.resColour(src); ok {
				ch = c // this resolution's ChooseColor answer (static_colourgrant.go)
			} else if src == NoSlot {
				return false
			}
			if ch > 4 || s.colorsGen(k, sl)&(1<<ch) == 0 {
				return false
			}
		}
		if f&kop.F2SameNameAsSource != 0 {
			// a face-down permanent has no name (CR 708.2): nameCardAt is NoCard
			if n := s.nameCardAt(sl); src == NoSlot || n == NoCard || n != s.nameCardAt(src) {
				return false // by name: Witness Protection renames (Hare Apparent counts no Legitimate Businessperson)
			}
		}
	}
	return s.flags2Obj(f, o)
}

// flags2Obj checks the object-identity flags (remembered, chosen,
// attached to target 0, targeted) and the per-turn damage record.
func (s *State) flags2Obj(f kop.FilterFlags2, o Obj) bool {
	if f&kop.F2Remembered != 0 && !s.inSet(&s.g.rem, s.g.nrem, o) {
		return false
	}
	if f&kop.F2NotRemembered != 0 && s.inSet(&s.g.rem, s.g.nrem, o) {
		return false
	}
	if f&kop.F2WasDamaged != 0 && !s.g.dmgTurn.has(o) {
		return false // WasDealtDamageThisTurn survives a zone change (gorge events.foldDamage)
	}
	if f&kop.F2ChosenCard != 0 && !s.inSet(&s.g.chosen, s.g.nchosen, o) {
		return false
	}
	if f&kop.F2NotChosenCard != 0 && s.inSet(&s.g.chosen, s.g.nchosen, o) {
		return false
	}
	if f&(kop.F2AttachedToT0|kop.F2TargetedBy) != 0 {
		e := s.curEntry()
		if e == nil {
			return false
		}
		if f&kop.F2AttachedToT0 != 0 {
			t := e.tgt[0]
			ob := &s.objs[o]
			if isPlayerOrNone(t) || ob.zone != kop.ZoneBattlefield || s.objs[t].zone != kop.ZoneBattlefield || s.perms[ob.slot].attached != s.objs[t].slot {
				return false
			}
		}
		if f&kop.F2TargetedBy != 0 && !s.entryTargets(e, o) {
			return false
		}
	}
	if f&kop.F2NotTriggerObj != 0 && !s.notTriggerObj(o) {
		return false
	}
	return true
}

func (s *State) entryTargets(e *stackEntry, o Obj) bool {
	for i := 0; i < kop.MaxTargets; i++ {
		if e.tgt[i] == TCode(uint8(o)) {
			return true
		}
	}
	for i := 0; i < int(s.xtOf(e).n); i++ {
		if s.xtOf(e).code[i] == TCode(o) {
			return true
		}
	}
	return false
}

// xBoundPending: fl bounds by an X an offer's census has not announced. No X
// makes a target of what the rest of the filter rules out, and any other object
// may fit some X (CR 601.2c, 602.2b).
func (s *State) xBoundPending(fl *kop.Filter) bool {
	return s.g.probe.kind != 0 && s.g.probe.x == 0 && fl.PredV.K == kop.VX
}

// xBoundPayable: some X the cost can pay, from the pool plus every untapped
// source, leaves ability a a legal target for every mandatory slot (CR 601.2b,
// 601.2c, 601.2h). A cost with no {X} or a target no X bounds needs no sweep. src
// is the ability's permanent, NoSlot for spell c's own ability.
func (s *State) xBoundPayable(k *prep, a *kop.Ability, c CardID, p uint8, src Slot, cost *kop.ManaCost) bool {
	if cost.X == 0 {
		return true
	}
	bound := false
	for i := 0; i < int(a.NTargets); i++ {
		bound = bound || xBoundTarget(k.t, &a.Targets[i])
	}
	if !bound {
		return true
	}
	var pot avail
	s.availPotential(k, p, &pot)
	s.g.probe = probe{kind: probeAbility, src: src, ctrl: p}
	if src == NoSlot {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
	}
	ok := false
	for x := costXMin(&a.Cost); !ok && x < 255; x++ { // from the floor (cost_sacx.go)
		cx := cost.AtX(x)
		if !pot.affords(&cx, pipMaskOf(&cx), 0) {
			break
		}
		s.g.probe.x = uint8(x + 1)
		ok = s.abilityTargetsFeasible(k, a, p, src, 0xff)
	}
	s.g.probe = probe{}
	return ok
}

// xCountTarget: slot t takes exactly the announced X ("X target
// creatures"). X = 0 always answers it, so only the X ask reads it
// (addXOptions), not the offer's sweep (xBoundPayable).
func xCountTarget(t *kop.TargetSpec) bool {
	return t.TFlags&kop.TgfBoundsV != 0 && !t.UpTo
}

// xBoundTarget: slot t's filter is bounded by an X.
func xBoundTarget(tb *kop.Table, t *kop.TargetSpec) bool {
	for f := t.Filter; f != 0; f = tb.Filters[f].Or {
		if tb.Filters[f].PredV.K == kop.VX {
			return true
		}
	}
	return false
}

func (s *State) predPermGen(k *prep, fl *kop.Filter, sl Slot, layer uint8) bool {
	n := int(fl.PredN)
	switch fl.Pred {
	case kop.PredPowerLE, kop.PredPowerGE, kop.PredToughnessLE:
		pw, t := s.ptGen(k, sl)
		switch fl.Pred {
		case kop.PredPowerLE:
			return pw <= n
		case kop.PredPowerGE:
			return pw >= n
		}
		return t <= n
	case kop.PredCMCLE:
		return s.cmcPerm(k, sl) <= n
	case kop.PredCMCGE:
		return s.cmcPerm(k, sl) >= n
	case kop.PredCMCEQ:
		return s.cmcPerm(k, sl) == n
	case kop.PredCMCLEV:
		if s.xBoundPending(fl) {
			return true
		}
		if n != 0 {
			return s.cmcPerm(k, sl) == s.evalV(k, fl.PredV, s.ctxCur()) // PredN 1: ==
		}
		return s.cmcPerm(k, sl) <= s.evalV(k, fl.PredV, s.ctxCur())
	case kop.PredCountersGE:
		return s.perms[sl].ctrN(s, n&0xff) >= n>>8
	case kop.PredHasKeyword:
		return s.kwGen(k, sl)&(1<<uint(n)) != 0
	case kop.PredHasCounter:
		return s.perms[sl].ctrN(s, n) > 0
	case kop.PredSubtype:
		return s.hasSubGen(k, sl, uint8(n))
	case kop.PredNamed:
		// the name card (gorge sharesName reads EffectiveNames: a layer-3
		// rename counts, CR 707.2); face-down is NoCard (CR 708.2: no name)
		return int(s.nameCardAt(sl)) == n
	case kop.PredToughnessGE, kop.PredHasXCost, kop.PredPowerLTToughness:
		return s.predPermStats(k, fl, sl)
	case kop.PredMultiColor, kop.PredHistoric, kop.PredEquipped, kop.PredEnchanted:
		return s.predPermKind(k, fl, sl)
	case kop.PredActiveCtrl:
		return s.perms[sl].ctrl == s.active
	case kop.PredModified:
		return s.predModified(k, sl)
	case kop.PredHasCounters:
		return s.predHasCounters(sl)
	case kop.PredSameNameRem:
		return s.sharesNameRem(s.perms[sl].obj, 0, false)
	}
	return s.predPermCold(k, fl, sl) // filter_typesall.go: PredTypesAll and later kinds
}

// nameCardAt is the card whose name the permanent at sl has: an override's
// name card (Witness Protection), else its own. A face-down permanent has
// no name (CR 708.2), so it reads as NoCard.
func (s *State) nameCardAt(sl Slot) CardID {
	if s.perms[sl].gty&gtyFaceDown != 0 {
		return NoCard
	}
	if nc, ok := s.FeatNameCard(s.perms[sl].obj); ok {
		return nc
	}
	return s.cardAt(sl)
}

// cmcPerm is sl's mana value for a filter: a face-down permanent has none
// (CR 708.2), so 0.
func (s *State) cmcPerm(k *prep, sl Slot) int {
	if s.faceDown(k, sl) || s.cpxNoCost(k, sl) || s.copiesBackFace(k, sl) {
		return 0 // CR 712.8e: a copy of a back face has no mana cost
	}
	c := s.cardAt(sl)
	if DebugFaceTransformStats != nil && k.t.Cards[c].Face != 0 {
		DebugFaceTransformStats.PredMV++
	}
	if k.anyRoom {
		return s.roomMV(k, c, s.perms[sl].obj, int(k.cards[c].total)) // face_rooms.go
	}
	return int(k.cards[c].total)
}

// matchCardGen reports whether card c (object o, NoObj if unknown; owned
// by owner, or controlled by owner on the stack) matches f relative to a
// source controlled by ctrl.
func (s *State) matchCardGen(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj) bool {
	return s.matchCardGenAt(k, f, c, owner, ctrl, o, nil)
}

// matchCardGenAt is matchCardGen read in ctx: the source object, slot and
// controller the filter's source-relative reads (the exiled-with flag, a *V
// bound) take. A nil ctx is the resolving or building entry's (ctxCur), so
// every existing caller reads as before; a static passes its own
func (s *State) matchCardGenAt(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, ctx *evalCtx) bool {
	if ctx != nil && ctx.uncounted == o+1 {
		return false
	}
	if f == 0 {
		return true
	}
	// matchTargetLKI reads a departed target as it last was (CR 608.2h), a token copy
	// bounced to hand included: Brackish Blunder's "if it was tapped".
	if o != NoObj && s.g.tli&tliOn == 0 && s.objs[o].of&ofCopy != 0 && s.copyCeased(k, o) {
		return false
	}
	return s.matchCardChain(k, f, c, owner, ctrl, o, ctx)
}

// matchCardChain is matchCardGenAt past its ceased-copy test: f's Or chain
// against the card, nothing else.
func (s *State) matchCardChain(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, ctx *evalCtx) bool {
	for f != 0 {
		if s.matchCardOneAt(k, &k.t.Filters[f], c, owner, ctrl, o, ctx) {
			return true
		}
		f = k.t.Filters[f].Or
	}
	return false
}

// copyCeased: copy o has left the stack and the battlefield, and matches nothing (CR
// 707.10h, gorge's copyCeased), a linked prepared copy in exile aside (CR 722.3c).
//
//go:noinline
func (s *State) copyCeased(k *prep, o Obj) bool {
	switch s.objs[o].zone {
	case kop.ZoneLibrary, kop.ZoneHand, kop.ZoneGraveyard:
		return true
	case kop.ZoneExile:
		_, live := s.preparedCopyLive(k, o)
		return !live
	}
	return false
}

func (s *State) matchCardOne(k *prep, fl *kop.Filter, c CardID, owner, ctrl uint8, o Obj) bool {
	return s.matchCardOneAt(k, fl, c, owner, ctrl, o, nil)
}

func (s *State) matchCardOneAt(k *prep, fl *kop.Filter, c CardID, owner, ctrl uint8, o Obj, ctx *evalCtx) bool {
	if fl.CtrlTgt != 0 {
		// a target's controller is a battlefield role; a card matcher has
		// no live target to read, so it fails closed (kgen never lowers it
		// off the battlefield)
		return false
	}
	if fl.Flags&kop.FPlayer != 0 {
		// a target spec's player alternative (kgen filterPlayerAlt) names a
		// player, never an object; kernel castTargetsPlayer reads it
		return false
	}
	if fl.Flags2&kop.F2BasePermanent != 0 {
		// a ThisTurnEntered battlefield entry whose object has since left
		// (a live permanent is read by the perm matcher): the Permanent base
		// reads the card where it now lies, a permanent card, as gorge's
		// matchesEnteredEntry does (CR 700.14, 603.10a)
		const permanentTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker | kop.TBattle
		if k.cards[c].types&permanentTypes == 0 {
			return false
		}
	}
	for _, w := range [2]kop.CtrlSel{fl.Ctrl, fl.Owner} {
		switch w {
		case kop.CtrlYou:
			if owner != ctrl {
				return false
			}
		case kop.CtrlOpp:
			if owner == ctrl {
				return false
			}
		case kop.CtrlEachPl:
			if !s.ctrlEachPl(owner, ctrl) {
				return false // a hand or library card of the repeated player (act_repeatplayer.go)
			}
		}
	}
	pc := &k.cards[c]
	cd := &k.t.Cards[c]
	ty := pc.types
	if k.gen != nil && k.gen.anyBestow && o != NoObj && s.bestowSpellObj(o) {
		ty = ty&^kop.TCreature | kop.TAura // a bestowed Aura spell (CR 702.103b; kw_bestow.go)
	}
	fd := k.gen != nil && k.gen.anyFaceDown && s.faceDownAt(k, o)
	if fd {
		ty = kop.TCreature // CR 708.4: a face-down spell is seen only as a 2/2 creature
	}
	if fl.Types != 0 && ty&fl.Types == 0 || ty&fl.NotTypes != 0 {
		return false
	}
	cols := cd.Colors
	if fd {
		cols = 0 // CR 708.2/708.4: a face-down permanent or spell is colourless
	}
	if fl.Colors != 0 && cols&fl.Colors == 0 || cols&fl.NotColors != 0 {
		return false
	}
	if fl.Sub != 0 && (fd || int(fl.Sub-1) >= len(k.t.Subtypes) || !cd.Subtypes.Has(int(fl.Sub-1))) {
		return false
	}
	if fl.NotSub != 0 && !fd && int(fl.NotSub-1) < len(k.t.Subtypes) && cd.Subtypes.Has(int(fl.NotSub-1)) {
		return false
	}
	for _, n := range fl.NotSubs {
		if n != 0 && !fd && int(n-1) < len(k.t.Subtypes) && cd.Subtypes.Has(int(n-1)) {
			return false
		}
	}
	if fl.Zone != kop.ZoneNone && fl.Zone != kop.ZoneAny && o != NoObj && s.objs[o].zone != fl.Zone {
		// ZoneAny is the TargetsValid$ any-zone marker: a zone-free object
		// alternative matches a target in any zone (kernel castTargetsAll
		// reads it only on a SpellCast Tf2TargetsFilter2 chain)
		return false
	}
	if fl.Flags&kop.FToken != 0 && (o == NoObj || s.objs[o].of&ofToken == 0) || fl.Flags&kop.FNontoken != 0 && o != NoObj && s.objs[o].of&ofToken != 0 {
		return false
	}
	if fl.Flags&kop.FExiledWith != 0 && !s.exwFlag(o, ctx) {
		return false // zone_exiledwithstatic.go
	}
	if s.g.tli&tliOn != 0 && !s.lkiStatusOK(fl) {
		return false
	}
	if fl.Flags&(kop.FAttacking|kop.FAttackingOrBlocking) != 0 && s.g.tli&tliOn == 0 {
		return false // a card off the battlefield is attacking nothing (gorge's IsAttacking clears with the move)
	}
	if f2 := fl.Flags2; f2 != 0 {
		if f2&kop.F2ChosenType != 0 {
			// a card filter reads the resolution's mid-resolution
			// ChooseType answer (Bloodline Shaman, Elvish Soultiller); a
			// static source's choice is handled by featCardMatch before
			// the filter reaches here, so outside a resolution there is
			// no choice to read
			ch, ok := s.chosenTypeSub(NoSlot)
			if !ok {
				return false
			}
			has := int(ch) < len(k.t.Subtypes) && cd.Subtypes.Has(int(ch))
			if outsideSub(k, ch) {
				has = cd.AllCreatureTypes // a printed changeling has every type
			}
			if s.res.bad&resChosenCard != 0 {
				has = s.chosenCardTypeHas(cd.Types) // a chosen card type (Winding Way)
			}
			if !has {
				return false
			}
			f2 &^= kop.F2ChosenType
		}
		if f2&kop.F2ChosenColor != 0 {
			// a card filter reads this resolution's ChooseColor answer
			// (Addle's discard choice; static_colourgrant.go resColour)
			if ch, ok := s.resColour(NoSlot); !ok || cols&(kop.ColW<<ch) == 0 {
				return false
			}
			f2 &^= kop.F2ChosenColor
		}
		if f2&(kop.F2Emblem|kop.F2Kicked|kop.F2CastByYou|kop.F2ChosenColor|kop.F2Saddled|kop.F2Sneaked|kop.F2Unblocked|kop.F2BlockedBySrc|kop.F2DamagedBy|kop.F2CrewedSrc|kop.F2Warped) != 0 {
			return false
		}
		if f2&(kop.F2TgtPlayerCtrl|kop.F2DefendingCtrl) != 0 && !s.cardRoleF2(k, f2, owner) {
			return false // an Owner role off the battlefield (filter_referents.go)
		}
		if f2&(kop.F2WithFlying|kop.F2WithoutFlying) != 0 && !s.cardFlyingOK(f2, pc, fd) {
			return false // filter_cardflying.go
		}
		if f2&kop.F2Basic != 0 && ty&kop.TBasic == 0 || f2&kop.F2Legendary != 0 && ty&kop.TLegendary == 0 {
			return false
		}
		if o == NoObj {
			if f2&(kop.F2EnteredThisTurn|kop.F2Remembered|kop.F2ChosenCard|kop.F2AttachedToT0|kop.F2TargetedBy|kop.F2SharesColorRem|kop.F2NotTriggerObj|kop.F2WasDamaged) != 0 {
				return false
			}
		} else {
			if f2&kop.F2EnteredThisTurn != 0 && !s.g.zoneTurn.has(o) {
				return false
			}
			if f2&kop.F2SharesColorRem != 0 && !s.sharesColorRem(k, o, cols) {
				return false
			}
			if !s.flags2Obj(f2, o) {
				return false
			}
		}
	}
	n := int(fl.PredN)
	switch fl.Pred {
	case kop.PredNone:
	case kop.PredPowerLE, kop.PredPowerGE, kop.PredToughnessLE:
		pw, t := int(pc.power), int(pc.tough)
		if fd {
			pw, t = 2, 2
		}
		if k.cda && k.gen.has(c, gfCDAPT) {
			pw, t = s.cardCDAPT(k, c, owner, o)
		}
		if s.g.tli&tliOn != 0 {
			pw, t = s.lkiPT() // a moved target's last known P/T (zone_lki.go)
		}
		switch fl.Pred {
		case kop.PredPowerLE:
			return pw <= n
		case kop.PredPowerGE:
			return pw >= n
		}
		return t <= n
	case kop.PredCMCLE, kop.PredCMCGE, kop.PredCMCEQ, kop.PredCMCLEV:
		mv := s.cardMV(k, c, o, fd)
		switch fl.Pred {
		case kop.PredCMCLE:
			return mv <= n
		case kop.PredCMCGE:
			return mv >= n
		case kop.PredCMCEQ:
			return mv == n
		}
		if s.xBoundPending(fl) {
			return true
		}
		cx := s.ctxCur()
		if ctx != nil {
			cx = *ctx // a static's own source and controller (Maralen)
		}
		if n != 0 {
			return mv == s.evalV(k, fl.PredV, cx) // PredN 1: == (Repurposing Bay)
		}
		return mv <= s.evalV(k, fl.PredV, cx)
	case kop.PredCountersGE:
		// off the battlefield only stash counters exist (Tinybones)
		return kop.CounterKind(n&0xff) == kop.CtrStash && n>>8 <= 1 && o != NoObj && s.mObj(k, mechStash, o)
	case kop.PredHasKeyword:
		return pc.kw&(1<<uint(n)) != 0
	case kop.PredSubtype:
		return cd.Subtypes.Has(int(n))
	case kop.PredNamed:
		return !fd && int(c) == n // CR 708.2/708.4: a face-down card has no name
	case kop.PredToughnessGE, kop.PredHasXCost, kop.PredPowerLTToughness:
		return s.predCardStats(k, fl, c, owner, o)
	case kop.PredMultiColor, kop.PredHistoric, kop.PredEquipped, kop.PredEnchanted:
		if fd && fl.Pred == kop.PredMultiColor {
			return false // colourless, so never more than one colour
		}
		return predCardKind(k, fl, c)
	case kop.PredAttached, kop.PredActiveCtrl:
		return fl.Pred == kop.PredActiveCtrl && owner == s.active // nothing off the battlefield is attached
	case kop.PredModified, kop.PredHasCounters:
		return false // only a battlefield permanent is modified or holds counters here
	case kop.PredSameNameRem:
		return s.sharesNameRem(o, c, fd)
	default:
		if fl.Pred == kop.PredExiledWith {
			return s.exwCard(o) // zone_exiledwith.go
		}
		if fl.Pred == kop.PredTriggerObj {
			return s.isTriggerObj(o) // filter_predicates.go: the ChooseFromDefined$ TriggeredCards pool
		}
		if fl.Pred == kop.PredWasCastEnter {
			return s.wasCastPred(fl, o, ctrl) // cast_wascastenter.go
		}
		if fl.Pred == kop.PredWasCastSpell {
			return s.wasCastSpell(fl, o, ctrl) // cast_wascastspell.go
		}
		if fl.Pred == kop.PredNotF2 {
			return s.notF2Card(fl, o) // filter_negations.go
		}
		if fl.Pred == kop.PredWord7 {
			return s.wordCard(k, fl, c, o, fd, ctrl) // filter_props.go
		}
		if (fl.Pred == kop.PredPowerGEV || fl.Pred == kop.PredToughnessLEV) && fl.PredN == 1 {
			return s.cardPTEqV(k, fl, c, owner, o, fd, ctx) // filter_ptvalue.go
		}
		if fl.Pred == kop.PredToughnessLEV && fl.PredN == 3 {
			return s.cardToughnessGEV(k, fl, c, owner, o, fd, ctx) // filter_ptvalue.go
		}
		if fl.Pred == kop.PredPowerGEV {
			return s.cardPowerGEV(k, fl, c, owner, o, fd, ctx) // filter_ptvalue.go
		}
		if fl.Pred == kop.PredTypesAllCMCLE {
			return s.typesAllCMCCard(k, fl, c, o, fd) // filter_typesall.go
		}
		if fl.Pred == kop.PredSameNameT0 {
			return s.sameNameT0Card(c, o, fd) // filter_samename.go
		}
		return predCardCold(k, fl, c, fd) // filter_typesall.go: PredTypesAll and later kinds
	}
	return true
}
