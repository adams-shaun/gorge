package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Values, conditions and references on a generated table.

// evalCtx is where a value is evaluated: the source object (and its slot
// when it is on the battlefield as the same object), its controller, and
// the stack entry resolving or being built (nil for a static).
type evalCtx struct {
	src   Obj
	srcSl Slot
	ctrl  uint8
	// uncounted is one more than the spell a cost static is pricing, which its
	// zone counts skip: CR 601.2a has it on the stack by the time CR 601.2f
	// determines the total cost. 0 skips nothing, and NoObj+1 wraps to 0.
	uncounted Obj
	e         *stackEntry
}

// ctxCur is the context of the resolving entry, else the one being built.
func (s *State) ctxCur() evalCtx {
	e := s.curEntry()
	if e == nil {
		return s.ctxOffer()
	}
	return s.ctxEntry(e)
}

// ctxOffer is the context of the cast or activation an offer is judging: nothing
// is being built, so its controller and source come from the probe.
func (s *State) ctxOffer() evalCtx {
	c := evalCtx{src: NoObj, srcSl: s.g.probe.src, ctrl: s.g.probe.ctrl}
	if s.g.probe.kind != probeAbility {
		c.srcSl = NoSlot
	} else if c.srcSl != NoSlot {
		c.src = s.perms[c.srcSl].obj
	}
	return c
}

func (s *State) ctxEntry(e *stackEntry) evalCtx {
	c := evalCtx{src: e.obj, srcSl: NoSlot, ctrl: e.ctrl, e: e}
	ob := &s.objs[e.obj]
	if ob.zone == kop.ZoneBattlefield && (ob.ver == e.srcVer || e.kind == seSpell) {
		c.srcSl = ob.slot
	}
	return c
}

// evalV evaluates a ValueRef in ctx.
func (s *State) evalV(k *prep, v kop.ValueRef, ctx evalCtx) int {
	e := ctx.e
	switch v.K {
	case kop.VConst:
		return int(v.N)
	case kop.VX, kop.VNegX:
		x := 0
		if px := s.g.probe.x; px != 0 {
			x = int(px) - 1
		} else if e != nil {
			x = int(e.x)
		} else if ctx.src != NoObj {
			x = int(s.spellX(ctx.src)) // a self-enters condition, before its trigger binds X
		}
		if v.K == kop.VNegX {
			return -x
		}
		return x
	case kop.VLastDamage:
		return int(s.res.lastDmg)
	case kop.VTriggerAmount:
		if e == nil {
			return 0
		}
		return int(e.trigAmt)
	case kop.VCardsInHand:
		return int(s.pl[ctx.ctrl].handN)
	case kop.VLifeGainedTurn:
		return int(s.pl[ctx.ctrl].lifeGained)
	case kop.VSelfPower, kop.VSelfToughness:
		pw, t := s.objPT(k, ctx.src, ctx.srcSl)
		if ctx.srcSl == NoSlot && e != nil && e.kind != seSpell && ctx.src == e.obj && ctx.src != NoObj {
			pw, t = s.selfPTLKI(k, e, pw, t)
		}
		if v.K == kop.VSelfPower {
			return pw
		}
		return t
	case kop.VTargetPower, kop.VTargetToughness:
		if e == nil || isPlayerOrNone(e.tgt[0]) {
			return 0
		}
		o := Obj(e.tgt[0])
		sl := s.slotIfSame(o, e.tver[0])
		if sl == NoSlot && e.li != 0 && e.kind == seSpell {
			pw, t := int(s.lk(e.li).power), int(s.lk(e.li).tough)
			if v.K == kop.VTargetPower {
				return pw
			}
			return t
		}
		pw, t := s.objPT(k, o, sl)
		if v.K == kop.VTargetPower {
			return pw
		}
		return t
	case kop.VCountFilter:
		return s.countPerm(k, kop.FilterID(v.N), ctx)
	case kop.VCountersSelf:
		if ctx.srcSl == NoSlot {
			if sl := s.selfCountersLKI(e, ctx.src); sl != NoSlot {
				return s.perms[sl].ctrN(s, int(v.N)) // a departed source's last counters (CR 608.2h)
			}
			if n, ok := s.selfDepartCtr(e, ctx.src, kop.CounterKind(v.N)); ok {
				return n // its slot was reused: the departure snapshot's (ctr_selfdepart.go)
			}
			if e != nil && e.li != 0 && e.kind == seAbility {
				return s.lkiCounter(e.li, kop.CounterKind(v.N))
			}
			return 0
		}
		return s.perms[ctx.srcSl].ctrN(s, int(v.N))
	case kop.VCountersTarget:
		if e == nil || isPlayerOrNone(e.tgt[0]) {
			return 0
		}
		if sl := s.slotIfSame(Obj(e.tgt[0]), e.tver[0]); sl != NoSlot {
			return s.perms[sl].ctrN(s, int(v.N))
		}
		return 0
	case kop.VExpr:
		return s.expr(k, v.N, ctx)
	}
	panic("kernel: value kind not implemented")
}

// slotIfSame is o's battlefield slot when o is there as version ver.
func (s *State) slotIfSame(o Obj, ver uint8) Slot {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield && ob.ver == ver {
		return ob.slot
	}
	return NoSlot
}

// objPT is an object's power/toughness: derived on the battlefield, else
// its card's printed values, or its characteristic-defining ones (CR 604.3; Station
// reads a graveyard creature's CDA power) (a phased-out permanent keeps its
// characteristic-defining P/T: kw_phasing.go).
func (s *State) objPT(k *prep, o Obj, sl Slot) (int, int) {
	if sl != NoSlot {
		return s.ptGen(k, sl)
	}
	if o == NoObj {
		return 0, 0
	}
	ob := &s.objs[o]
	if k.phasing && ob.zone.Phased() && ob.slot != NoSlot {
		return s.phasedPT(k, ob.slot)
	}
	pc := &k.cards[ob.card]
	if k.cda && k.gen.has(ob.card, gfCDAPT) {
		return s.cardCDAPT(k, ob.card, ob.owner(), o) // CR 604.3: a CDA works in every zone
	}
	return int(pc.power), int(pc.tough)
}

func (s *State) countPerm(k *prep, f kop.FilterID, ctx evalCtx) int {
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		if s.matchPermGen(k, f, m.first(), ctx.ctrl, ctx.srcSl, layerFull) {
			n++
		}
	}
	return n
}

// pwTypeCount is ExCount with N 1: the number of distinct planeswalker types
// (CR 205.3j) among the battlefield permanents matching F, read off each
// match's printed subtypes when its printed type line has Planeswalker (gorge
// zoneFold's PlaneswalkerType reads the face's words after the Planeswalker
// card type; a face-down permanent's face has none). Tam, the Possibility.
func (s *State) pwTypeCount(k *prep, f kop.FilterID, ctx evalCtx) int {
	var seen kop.SubMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPermGen(k, f, sl, ctx.ctrl, ctx.srcSl, layerFull) || s.faceDown(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		if cd.Types&kop.TPlaneswalker == 0 {
			continue
		}
		subs := cd.Subtypes
		if cd.PrintedSubtypes.Any() {
			subs = cd.PrintedSubtypes // a changeling's expanded roster is not printed
		}
		seen = seen.Or(subs)
	}
	return bits.OnesCount64(seen[0]) + bits.OnesCount64(seen[1])
}

// expr evaluates Table.Exprs[i].
func (s *State) expr(k *prep, i int16, ctx evalCtx) int {
	x := &k.t.Exprs[i]
	e := ctx.e
	v := 0
	switch x.Kind {
	case kop.ExConst:
		v = int(x.N)
	case kop.ExX:
		if e != nil {
			v = int(e.x)
		} else if ctx.src != NoObj {
			v = int(s.spellX(ctx.src)) // a self-enters condition, before its trigger binds X
		}
	case kop.ExCount:
		if x.N == 1 {
			v = s.pwTypeCount(k, x.F, ctx)
		} else {
			v = s.countPerm(k, x.F, ctx)
		}
	case kop.ExCountGraveyard:
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			pl := &s.pl[pp]
			for j := 0; j < int(pl.graveN); j++ {
				o := pl.grave[j]
				if s.objs[o].of&ofToken != 0 && x.N != 1 {
					continue // a token that died waits there until the next check ceases it; it is no card (CR 111.7, 704.5d); N 1 (HasCardsInGraveyard) counts it as gorge's zone walk does
				}
				if s.matchCardGenAt(k, x.F, s.objs[o].card, pp, ctx.ctrl, o, &ctx) {
					v++
				}
			}
		}
	case kop.ExThisTurnEntered:
		v = s.thisTurnEntered(k, x, ctx)
	case kop.ExSumPower:
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if s.typesGen(k, sl)&kop.TCreature != 0 && s.matchPermGen(k, x.F, sl, ctx.ctrl, ctx.srcSl, layerFull) {
				pw, _ := s.ptGen(k, sl)
				v += pw
			}
		}
	case kop.ExDistinctCMC:
		var seen uint64
		for m := s.occ; m.any(); m = m.rest() {
			sl := m.first()
			if s.matchPermGen(k, x.F, sl, ctx.ctrl, ctx.srcSl, layerFull) {
				mv := k.cards[s.cardAt(sl)].total
				if s.faceDown(k, sl) || s.cpxNoCost(k, sl) {
					mv = 0 // CR 708.2: a face-down permanent has no mana cost (nor an embalm copy)
				}
				seen |= 1 << (mv & 63)
			}
		}
		v = bits.OnesCount64(seen)
	case kop.ExKicked:
		if e != nil && e.flags&sfKicked != 0 && !optCostUnpaid(k, e) {
			v = s.evalV(k, x.Sub[0], ctx)
		} else {
			v = s.evalV(k, x.Sub[1], ctx)
		}
	case kop.ExSneaked:
		if s.sneakProv(e) {
			v = s.evalV(k, x.Sub[0], ctx)
		} else {
			v = s.evalV(k, x.Sub[1], ctx)
		}
	case kop.ExCastFromGrave:
		if s.castFromGrave(e) {
			v = s.evalV(k, x.Sub[0], ctx)
		} else {
			v = s.evalV(k, x.Sub[1], ctx)
		}
	case kop.ExFirstCast:
		if s.firstCast(e) { // cast_firstcast.go
			v = s.evalV(k, x.Sub[0], ctx)
		} else {
			v = s.evalV(k, x.Sub[1], ctx)
		}
	case kop.ExPromisedGift:
		if s.promisedCtx(ctx) {
			v = s.evalV(k, x.Sub[0], ctx)
		} else {
			v = s.evalV(k, x.Sub[1], ctx)
		}
	case kop.ExMorbid:
		if s.g.died > 0 {
			v = s.evalV(k, x.Sub[0], ctx)
		} else {
			v = s.evalV(k, x.Sub[1], ctx)
		}
	case kop.ExAttackedThisTurn:
		v = int(s.g.attacked[ctx.ctrl])
	case kop.ExPowerOf, kop.ExToughnessOf:
		pw, t := s.refPT(k, x.Ref, ctx)
		v = pw
		if x.Kind == kop.ExToughnessOf {
			v = t
		}
	case kop.ExLifeOf:
		if p := s.refPlayer(k, x.Ref, ctx); p != 255 {
			v = int(s.pl[p].life)
		}
	case kop.ExLibrarySizeOf:
		if p := s.refPlayer(k, x.Ref, ctx); p != 255 {
			v = s.pl[p].libLen()
		}
	case kop.ExStartingLife:
		v = s.startingLife()
	case kop.ExTriggerAmount:
		if e != nil {
			v = int(e.trigAmt)
		}
	case kop.ExLifeGainedThisTurn:
		v = int(s.pl[ctx.ctrl].lifeGained)
	case kop.ExOppLifeLostThisTurn:
		v = int(s.pl[1-ctx.ctrl].lifeLost)
	case kop.ExCountersOn:
		v = s.refCounters(k, x, x.Ref, x.Ctr, ctx)
	case kop.ExCreaturesDied:
		v = int(s.g.died)
	case kop.ExRememberedCount:
		v = int(s.g.nrem) + int(s.g.remCnt)
	case kop.ExExcessDamage:
		v = int(s.g.excess)
	case kop.ExLastLifeLost:
		v = int(s.g.lostAmt)
	case kop.ExTokensCreated:
		v = int(s.g.tokMade)
	default:
		v = s.exprCold(k, x, ctx)
	}
	if x.Mul != 0 {
		v *= int(x.Mul)
	}
	if x.Div != 0 {
		v /= int(x.Div)
	}
	v += int(x.Add)
	if x.Neg {
		v = -v
	}
	return v
}

// refPT is the power/toughness of the object a ref names, with last known
// information for the triggering object and a sacrificed cost.
func (s *State) refPT(k *prep, r kop.Ref, ctx evalCtx) (int, int) {
	e := ctx.e
	switch r {
	case kop.RefSelf:
		pw, t := s.objPT(k, ctx.src, ctx.srcSl)
		if ctx.srcSl == NoSlot && e != nil && e.kind != seSpell && ctx.src == e.obj && ctx.src != NoObj {
			pw, t = s.selfPTLKI(k, e, pw, t) // CR 608.2h: the source left since the trigger fired
		}
		return pw, t
	case kop.RefTriggerObj, kop.RefTriggerSource, kop.RefSacrificed, kop.RefDelayedRemembered:
		if r == kop.RefTriggerObj && e != nil && e.trigObj != NoObj {
			o := e.trigObj
			// Gorge evaluates it at resolution: the live power
			// while the triggering object is still on the battlefield as the
			// same object (CR 400.7), the snapshot only once it left (CR
			// 608.2h; The Roaring Toeclaws, Experiment Twelve).
			ver := s.objs[o].ver
			if e.li != 0 {
				ver = s.lk(e.li).ver
			}
			if sl := s.slotIfSame(o, ver); sl != NoSlot {
				return s.ptGen(k, sl)
			}
		}
		if e != nil && e.li != 0 {
			return int(s.lk(e.li).power), int(s.lk(e.li).tough)
		}
		if e != nil && e.trigObj != NoObj {
			o := e.trigObj
			return s.objPT(k, o, s.slotIfSame(o, s.objs[o].ver))
		}
		if r == kop.RefSacrificed {
			return s.objPT(k, ctx.src, NoSlot)
		}
		return 0, 0
	}
	if r == kop.RefAttached && ctx.e == nil {
		return s.staticAttachedPT(k, ctx) // value_refheads.go
	}
	o := s.refObjCtx(r, ctx)
	if o == NoObj {
		return 0, 0
	}
	return s.objPT(k, o, s.slotIfSame(o, s.objs[o].ver))
}

func (s *State) refCounters(k *prep, x *kop.Expr, r kop.Ref, c kop.CounterKind, ctx evalCtx) int {
	e := ctx.e
	if c == kop.CtrAll {
		return s.refCountersAll(x, r, ctx) // trig_gravedrawn.go
	}
	if (r == kop.RefDelayedRemembered || r == kop.RefTriggerObj && !DebugTrigCtrLive) && e != nil && e.li != 0 {
		return s.lkiCounter(e.li, c)
	}
	var o Obj
	if r == kop.RefSelf {
		o = ctx.src
	} else {
		o = s.refObjCtx(r, ctx)
	}
	if r == kop.RefSelf {
		if sl := s.selfCountersLKI(e, o); sl != NoSlot {
			return s.perms[sl].ctrN(s, int(c))
		}
		if n, ok := s.selfDepartCtr(e, o, c); ok {
			return n // its slot was reused: the departure snapshot's (ctr_selfdepart.go)
		}
	}
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return 0
	}
	return s.perms[s.objs[o].slot].ctrN(s, int(c))
}

// refObjCtx resolves an object ref in ctx (one object; sets give their
// first).
func (s *State) refObjCtx(r kop.Ref, ctx evalCtx) Obj {
	e := ctx.e
	switch r {
	case kop.RefSelf:
		return ctx.src
	case kop.RefRemembered:
		if s.g.nrem > 0 {
			return s.g.rem[0]
		}
		return NoObj
	case kop.RefImprinted:
		if s.g.nimp > 0 {
			return s.g.imp[0]
		}
		return NoObj
	case kop.RefChosen:
		if s.g.nchosen > 0 {
			return s.g.chosen[0]
		}
		return NoObj
	case kop.RefReplaced:
		return s.g.replaced
	}
	if e == nil {
		return NoObj
	}
	if r == kop.RefDiscarded {
		return discardedObj(e) // value_discardedref.go
	}
	switch r {
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		t := e.tgt[sl]
		if e == &s.res.e {
			t = s.tgtCode(sl)
		}
		if t == NoTarget || isPlayerOrNone(t) {
			return NoObj
		}
		return Obj(t)
	case kop.RefTriggerObj, kop.RefTriggerSource, kop.RefDelayedRemembered, kop.RefHost:
		if r == kop.RefDelayedRemembered && e.remSet && e.trigObj != NoObj &&
			s.objs[e.trigObj].ver != e.remVer {
			return NoObj // a new object since registration (CR 400.7; gorge dropDelayedRememberedMoved)
		}
		return e.trigObj
	case kop.RefAttached:
		ob := &s.objs[e.obj]
		if ob.zone == kop.ZoneBattlefield {
			if a := s.perms[ob.slot].attached; a != NoSlot {
				return s.perms[a].obj
			}
			return NoObj
		}
		return s.lkiAttached(e.obj)
	}
	return NoObj
}

// refPlayer resolves a player ref in ctx (255 = none; each-player refs are
// expanded by the caller).
func (s *State) refPlayer(k *prep, r kop.Ref, ctx evalCtx) uint8 {
	switch r {
	case kop.RefController, kop.RefSelf:
		return ctx.ctrl
	case kop.RefOpponent, kop.RefEachOpponent, kop.RefDefendingPlayer:
		return 1 - ctx.ctrl
	case kop.RefEach:
		return s.g.eachPl // the player of the OpRepeatEachPlayer in progress
	}
	e := ctx.e
	if e == nil {
		return 255
	}
	switch r {
	case kop.RefTriggerPlayer:
		return e.trigPl
	case kop.RefTriggerTgtCtrl:
		// gorge TriggeredTargetController, read live (resolve.go playerRef)
		if o := e.trigObj; o != NoObj {
			return s.ctrlOfObj(o)
		}
		return e.trigPl
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4, kop.RefTargetController, kop.RefTargetOwner:
		i, _ := tgtSlot(r)
		t := e.tgt[i]
		if e == &s.res.e {
			t = s.tgtCode(i)
		}
		if t == NoTarget {
			return 255
		}
		if isPlayerOrNone(t) {
			return CodeSeat(t)
		}
		if r == kop.RefTargetOwner {
			return s.objs[t].owner()
		}
		return s.ctrlOfObj(Obj(t))
	}
	if o := s.refObjCtx(r, ctx); o != NoObj {
		return s.ctrlOfObj(o)
	}
	if r == kop.RefRememberedController {
		return s.remController() // zone_rememberparams.go
	}
	if r == kop.RefRememberedOwner {
		return s.remOwner() // zone_rememberparams.go
	}
	return 255
}

// condEval evaluates Table.Conds[id] in ctx.
func (s *State) condEval(k *prep, id kop.CondID, ctx evalCtx) bool {
	c := &k.t.Conds[id]
	if DebugCondKeysIgnore && s.condKeysIgnored(k, c) { // repl_conditions.go
		return true
	}
	r := false
	switch c.Kind {
	case kop.CondAlways:
		r = true
	case kop.CondPresent:
		r = cmp(c.Op, s.countPerm(k, c.F, ctx), s.evalV(k, c.A, ctx))
	case kop.CondValue:
		r = cmp(c.Op, s.evalV(k, c.A, ctx), s.evalV(k, c.B, ctx))
	case kop.CondKicked:
		r = ctx.e != nil && ctx.e.flags&sfKicked != 0 && !optCostUnpaid(k, ctx.e)
	case kop.CondThreshold:
		r = s.graveCards(ctx.ctrl) >= 7 // a token waiting to cease is no card (CR 111.7)
	case kop.CondYourTurn:
		r = s.active == ctx.ctrl
	case kop.CondRefMatches:
		r = cmp(c.Op, s.countRefMatches(k, c.Ref, c.F, ctx), s.evalV(k, c.A, ctx))
	case kop.CondPromised:
		r = s.promisedCtx(ctx)
	case kop.CondAnd:
		r = s.condEval(k, c.Sub[0], ctx) && s.condEval(k, c.Sub[1], ctx)
	default:
		panic("kernel: condition kind not implemented")
	}
	return r != c.Neg
}

func cmp(op kop.CmpOp, a, b int) bool {
	switch op {
	case kop.CmpGE:
		return a >= b
	case kop.CmpLE:
		return a <= b
	case kop.CmpEQ:
		return a == b
	case kop.CmpGT:
		return a > b
	case kop.CmpLT:
		return a < b
	case kop.CmpNE:
		return a != b
	}
	return a >= b
}

// countRefMatches counts the objects a ref names that match f (a moved
// card is matched as a card in its new zone).
func (s *State) countRefMatches(k *prep, r kop.Ref, f kop.FilterID, ctx evalCtx) int {
	return s.countRefMatchesN(k, r, f, ctx, false)
}

// countRefMatchesN is countRefMatches with lkiType: a Sacrificed$Valid
// Card/creature-type read (kgen marks its Expr.N 1), which takes the
// sacrifice-time type snapshot instead of the live moved card a
// ConditionDefined$ Sacrificed filter reads.
func (s *State) countRefMatchesN(k *prep, r kop.Ref, f kop.FilterID, ctx evalCtx, lkiType bool) int {
	// ceased: the object is read as the card it was moved as even if it is a
	// copy that has since left the stack and the battlefield (CR 707.10h
	// ends its existence at the next check, not within the resolution);
	// gorge's remembered-set condition matches it past that rejection.
	var ceased bool
	one := func(o Obj) int {
		if o == NoObj {
			return 0
		}
		ob := &s.objs[o]
		if ob.zone == kop.ZoneBattlefield {
			if s.matchPermGen(k, f, ob.slot, ctx.ctrl, ctx.srcSl, layerFull) {
				return 1
			}
			return 0
		}
		if ceased && f != 0 && s.g.tli&tliOn == 0 && ob.of&ofCopy != 0 {
			if s.matchCardChain(k, f, ob.card, ob.owner(), ctx.ctrl, o, nil) {
				return 1
			}
			return 0
		}
		if s.matchCardGen(k, f, ob.card, ob.owner(), ctx.ctrl, o) {
			return 1
		}
		return 0
	}
	switch r {
	case kop.RefImprinted:
		n := 0
		for i := 0; i < int(s.g.nimp); i++ {
			n += one(s.g.imp[i])
		}
		return n
	case kop.RefRemembered:
		n := 0
		ceased = true
		for i := 0; i < int(s.g.nrem); i++ {
			if o := s.g.rem[i]; o == ctx.src && o != NoObj && s.objs[o].zone != kop.ZoneBattlefield {
				n += s.matchRemSource(k, f, o, ctx) // zone_rememberparams.go
				continue
			}
			if s.g.tli&tliRemLKI != 0 {
				if m, ok := s.matchRemLKI(k, f, s.g.rem[i], ctx); ok { // zone_rememberparams.go
					n += m
					continue
				}
			}
			n += one(s.g.rem[i])
		}
		return n
	case kop.RefSelf:
		// the source as it is (a battlefield permanent, or the card being
		// cast/entering: Nine-Lives Familiar "if you cast it")
		if ctx.src == NoObj {
			return 0
		}
		if ctx.srcSl != NoSlot {
			if s.matchPermGen(k, f, ctx.srcSl, ctx.ctrl, ctx.srcSl, layerFull) {
				return 1
			}
			return 0
		}
		return one(ctx.src)
	case kop.RefSacrificed:
		o := s.sacrificedObj(ctx)
		// Sacrificed$Valid Card.IsSuspected and Card/creature-type reads
		// take the sacrifice-time LKI snapshot (gorge sacrificedValidCount).
		if o != NoObj && ctx.e != nil && ctx.e.li != 0 {
			if n, ok := s.sacrificedLKICount(k, f, ctx.e.li, lkiType); ok {
				s.kdefHit(ctx, o, n)
				return n
			}
		}
		n := one(o)
		s.kdefHit(ctx, o, n)
		return n
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		// a target that moved during this resolution is followed
		if ctx.e == nil {
			return 0
		}
		sl, _ := tgtSlot(r)
		t := ctx.e.tgt[sl]
		if t == NoTarget || isPlayerOrNone(t) {
			return 0
		}
		if s.g.tli != 0 && s.objs[Obj(t)].zone != kop.ZoneBattlefield {
			if m, ok := s.matchTargetLKI(k, f, Obj(t), ctx.ctrl); ok {
				if m {
					return 1
				}
				return 0
			}
		}
		return one(Obj(t))
	}
	if r == kop.RefTriggerObj && ctx.e != nil && ctx.e.li != 0 {
		if m, ok := s.matchTrigLKI(k, f, ctx.e.trigObj, ctx); ok { // trig_enteredlki.go
			if m {
				return 1
			}
			return 0
		}
	}
	return one(s.refObjCtx(r, ctx))
}

// objRefGen resolves the refs a generated table adds (and RefChosen, whose
// set lives in genState) for the core's objRefObj.
func (s *State) objRefGen(r kop.Ref) (Obj, bool) {
	switch r {
	case kop.RefChosen, kop.RefRemembered, kop.RefImprinted, kop.RefReplaced, kop.RefDelayedRemembered, kop.RefTriggerSource, kop.RefHost:
		return s.refObjCtx(r, s.ctxEntry(&s.res.e)), true
	case kop.RefSacrificed:
		return NoObj, true
	case kop.RefLibTop:
		return s.libTopRef() // zone_toplibrary.go
	}
	return NoObj, false
}
