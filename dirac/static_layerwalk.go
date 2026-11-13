package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The CR 613 layer walk for layers 4, 5 and 7b mirroring gorge's chars.Compute (engine/rules/chars/derive.go) and
// its layer-4 walk (engine/rules/chars/types.go TypesAndLandSubtypeSet).
// Cold code, reached from
// typesGen, colorsGen, hasSubGen and basePTGen through one predictable
// branch (k.gen.fxNeed != 0 and the slot is a walk slot); a table whose
// prep leaves fxNeed empty (no override static, no subtype-reading
// subtype grant) runs the fixed-order code unchanged.
//
// The effects on one permanent are its override records (one-shot, pool
// order = timestamp order) and the override statics in force (stamped with
// their source's layerTs), now matched by their OWN filter (an Aura's
// "enchanted creature", Maha's "creatures your opponents control"), plus,
// when fxNeed has fxSubGrant, the Continuous statics' subtype grants. They
// are merged by timestamp. Layer 4 applies them in CR 613.8 dependency
// order (gorge types.go: the timestamp-earliest effect no remaining effect
// changes the applicability of; a cycle falls back to timestamp order),
// matching each static's filter against the types built so far
// (matchPermCur). A static that has a layer-4 part is locked to the
// layer-4 verdict in layers 5 and 7b (CR 613.6); one without is matched
// against the finished layer-4 types at its own layer. Layer 6 (keywords,
// losses, grants) is walked in static_layer6.go.

// fxNeed bits (genPrep.fxNeed).
const (
	fxNeedOvr      uint8 = 1 << iota // an override static (Aura or global)
	fxNeedGlobal                     // an override static not limited to its source's host
	fxNeedSubGrant                   // a Continuous subtype grant whose filter reads types or subtypes
)

// fxNeedSubSet (bit 5, after static_layer6.go's bits): a Continuous subtype
// grant shares the table with a type or subtype setter that can reach the
// same permanent (CR 613.1d: timestamp order), so every slot an override
// reaches walks (refreshTy) and the grant is ordered with the setter.
const fxNeedSubSet uint8 = 1 << 5

// outsMax is the outside-type grants one dchars holds; a fifth sets
// nOuts = outsOver (act_choosetype.go lfxChosen).
const outsMax = 4

// outsOver marks more outside-type grants than dchars holds.
const outsOver uint8 = 255

// outGrant is one AddChosen static's outside-type grant: the granted
// type's perm.chosen code and the static's source slot+1.
type outGrant struct{ code, src uint8 }

// dchars is one permanent's characteristics as the walk builds them. A
// stack local: it never escapes.
type dchars struct {
	types  kop.TypeMask
	cols   kop.ColorMask
	all    bool // every creature type (CR 702.73a / an all-creature-types effect)
	subs   kop.SubMask
	bp, bt int
	outs   [outsMax]outGrant // AddChosen grants of types outside the matchup (act_choosetype.go)
	nOuts  uint8             // grants held in outs; outsOver = more than outsMax
}

// walk stop points.
const (
	upTo4  uint8 = iota // types and subtypes
	upTo5               // + colours
	upTo7b              // + base P/T
)

// lfx is one effect of the walk: an override (record or static) or a
// static subtype grant (o == nil).
type lfx struct {
	o    *kop.Override
	a    *kop.Ability // the static (nil: a record)
	ts   uint16
	src  Slot
	ctrl uint8
	obj  Obj
	ok   uint8 // statics with a layer-4 part: 0 undecided, 1 applied, 2 did not
	reps uint8 // a record: the identical applications it stands for (ovRec.reps)
	p, t int16 // set P/T (a record's values were fixed at application; a LivePT static's are read from its source)
	ch   uint8 // a static: its source's perm.chosen (AddChosen; act_choosetype.go)
}

// maxLfx bounds the effects one slot's layer walk collects; 24 since
// kcaps2x (was 12: thirteen Spacecraft statics overflowed it, HANDOFF-kcounters).
const maxLfx = 24

// The records loop (and ovRecs) read at most maxOvr of a slot's records;
// the walk must be able to hold them all.
var _ [maxLfx - maxOvr]struct{}

// walkSlot reports whether sl's characteristics come from the walk: two
// or more order-sensitive effects reach it, one of them an override static
// (perm.ovr.walk, kept by refreshTy), or the table has a global override
// static or a subtype lord (every slot walks). Any other slot has at most
// one effect, or records only (pool order is timestamp order), where the
// fixed-order code gives the walk's answer.
func (s *State) walkSlot(k *prep, sl Slot) bool {
	return s.perms[sl].ovr.walk&walkOn != 0 || k.gen.fxNeed&(fxNeedGlobal|fxNeedSubGrant) != 0
}

// collect gathers the effects on sl in timestamp order.
func (s *State) lfxCollect(k *prep, sl Slot, fx *[maxLfx]lfx) int {
	n := 0
	if s.fxM.has(sl) {
		c := 0
		for i := 0; i < int(s.g.nfx) && c < maxOvr; i++ {
			if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride {
				c++
				if n < maxLfx {
					fx[n] = lfx{o: &k.t.Overrides[r.r.idx-1], ts: r.r.ts, src: NoSlot, reps: uint8(r.r.reps()), p: r.r.p, t: r.r.t}
					if fx[n].o.WhileHost {
						fx[n].p, fx[n].t = int16(fx[n].o.SetP), int16(fx[n].o.SetT) // r.r.p/t hold the host
					}
					n++
				}
			}
		}
	}
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		cid := s.cardAt(src)
		sub := k.gen.has(cid, gfSubGrant) // a walked slot takes every subtype grant (the fixed path's are not read)
		if !k.gen.has(cid, gfOvrStat) && !sub {
			continue
		}
		sp := &s.perms[src]
		oneShot := sp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0
		if !k.gen.has(cid, gfGlobOvr) && !sub && (sp.attached != sl || src == sl) {
			continue // an Aura's override reaches only its host (the filter is checked below)
		}
		// Whether the source has lost all abilities (CR 613.1f), decided at
		// most once and only for a static that would not outlive the loss.
		lostKnown, lost := false, false
		cd := &k.t.Cards[cid]
		ts := s.layerTs(src)
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic {
				continue
			}
			if n >= maxLfx {
				s.lfxFull(k, sl, src, a, sub)
				continue
			}
			if !oneShot && !lostKnown && !isRemover(k.t, a) && !keptOnLoss(k.t, a) {
				lost, lostKnown = s.grantSrcLost(k, src), true
			}
			if (oneShot || lost && !isRemover(k.t, a)) && !keptOnLoss(k.t, a) {
				continue // CR 613.6: only an effect that began before layer 6 outlives the loss
			}
			var e lfx
			switch {
			case a.Static.Kind == kop.StatOverride && a.Static.Override > 0:
				if src != sl && selfOnlyFilter(k, a.Static.Filter) {
					continue // a self static (AffectedDefined$ Self) reaches its source only
				}
				e = lfx{o: &k.t.Overrides[a.Static.Override-1], a: a, ts: ts, src: src, ctrl: sp.ctrl, obj: sp.obj}
				e.p, e.t = int16(e.o.SetP), int16(e.o.SetT)
			case sub && a.Static.Kind == kop.StatContinuous && (a.Static.AddSub.Any() || a.Static.AddChosen):
				e = lfx{a: a, ts: ts, src: src, ctrl: sp.ctrl, obj: sp.obj, ch: uint8(sp.chosen)}
			default:
				continue
			}
			// merge into timestamp order (records first on a tie: they
			// were applied after the source entered)
			j := n
			for j > 0 && fx[j-1].ts > ts {
				fx[j] = fx[j-1]
				j--
			}
			fx[j] = e
			n++
		}
	}
	if k.gen.embSubGrant {
		// emblems' subtype grants (timestamp 0: oldest)
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			if s.out(pp) {
				continue
			}
			for i := 0; i < int(s.g.nemb[pp]); i++ {
				e := &k.t.Emblems[s.embIdx(pp, i)-1]
				for j := range e.Abilities {
					a := &e.Abilities[j]
					if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || !a.Static.AddSub.Any() {
						continue
					}
					if n >= maxLfx {
						s.capFull(StallBoard) // loud: never drop the grant
						continue
					}
					copy(fx[1:n+1], fx[:n])
					fx[0] = lfx{a: a, src: NoSlot, ctrl: pp, obj: NoObj, ch: 255}
					n++
				}
			}
		}
	}
	return n
}

// isRemover reports whether static a takes abilities away itself. Such a
// static is silenced by a one-shot loss only; its timestamp against the other
// removers is the layer-6 walk's. Any other static that begins in layer 6 or
// 7 is gone with whatever removal reaches its source (CR 613.8a).
func isRemover(t *kop.Table, a *kop.Ability) bool {
	return a.Static.Kind == kop.StatOverride && a.Static.Override > 0 && t.Overrides[a.Static.Override-1].LoseAbilities
}

// keptOnLoss reports whether static a, whose source has lost all abilities in
// layer 6, still applies (CR 613.6: an effect that began in an earlier layer
// keeps applying, every part of it, though the ability generating it is
// removed). A subtype grant begins in layer 4, a type, subtype, colour or
// name setter in layers 3 to 5; a P/T, keyword or removal effect with no such
// part begins in layer 6 or 7 and ends.
func keptOnLoss(t *kop.Table, a *kop.Ability) bool {
	if subGrantStatic(a) {
		return true
	}
	if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.OutlivesLoss {
		return true
	}
	if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override > 0 {
		return ovrKeptOnLoss(&t.Overrides[a.Static.Override-1])
	}
	return false
}

// ovrKeptOnLoss reports whether override o has a part in layers 1 to 5.
func ovrKeptOnLoss(o *kop.Override) bool {
	return o.HasTypes || o.AddTypes != 0 || o.HasSub || o.AddSub.Any() || o.AllCreatureTypes || o.RemoveCreatureSub ||
		o.HasColors || o.AddColors != 0 || o.NameCard != 0
}

// subGrantStatic reports whether a is a static subtype grant: an effect that
// begins in layer 4, so it keeps applying after its source loses all
// abilities in layer 6 (CR 613.6).
func subGrantStatic(a *kop.Ability) bool {
	return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && (a.Static.AddSub.Any() || a.Static.AddChosen)
}

// l4part reports whether effect e has a layer-4 part.
func l4part(e *lfx) bool {
	if e.o == nil {
		return true
	}
	o := e.o
	return o.HasTypes || o.AddTypes != 0 || o.HasSub || o.AddSub.Any() || o.AllCreatureTypes || o.RemoveCreatureSub
}

// lfxApplies reports whether e applies to sl given the characteristics
// built so far.
func (s *State) lfxApplies(k *prep, sl Slot, e *lfx, cur *dchars) bool {
	if e.a == nil {
		return true // a record names its object (CR 611.2c)
	}
	if e.a.Static.Cond2 != 0 && !s.lfxCond(k, e) {
		return false // the re-entry guard (static_reentryguard.go)
	}
	return s.matchPermCur(k, e.a.Static.Filter, sl, e.ctrl, e.src, cur)
}

// lfxApply4 applies e's layer-4 part to c.
func lfxApply4(k *prep, e *lfx, c *dchars) {
	if e.o == nil {
		c.subs = c.subs.Or(e.a.Static.AddSub)
		if k.gen.anyAddChosen && e.a.Static.AddChosen {
			lfxChosen(k, e, c) // act_choosetype.go
		}
		return
	}
	o := e.o
	const keep = kop.TLegendary | kop.TBasic | kop.TToken
	if o.HasTypes {
		c.types = c.types&(keep|o.KeepTypes) | o.SetTypes
	}
	c.types |= o.AddTypes
	if o.HasSub {
		c.subs = c.subs.And(o.KeepSub).Or(o.SetSub)
		c.all = false
	}
	if o.RemoveCreatureSub && !DebugRmCreatSubKeep {
		c.subs, c.all = c.subs.AndNot(k.gen.creatSub), false
	}
	c.subs = c.subs.Or(o.AddSub)
	if ovrAllCreat(o) {
		c.all = true
	}
}

// chars is the walk: sl's characteristics through layer upTo.
func (s *State) chars(k *prep, sl Slot, upTo uint8) dchars {
	var fx [maxLfx]lfx
	var n int
	return s.charsFx(k, sl, upTo, &fx, &n)
}

// charsFx is chars leaving the walk's effects (with their layer-4
// verdicts) in fx[:*np] for the layer-3 and layer-6 walks (static_layer6.go).
func (s *State) charsFx(k *prep, sl Slot, upTo uint8, fx *[maxLfx]lfx, np *int) dchars {
	n := s.lfxCollect(k, sl, fx)
	*np = n
	p := &s.perms[sl]
	cid := s.cardAt(sl)
	cd := &k.t.Cards[cid]
	var c dchars
	fd := s.faceDown(k, sl)
	// layer 1: the card (or its copy-of) and a copy token's extra subtype
	if fd {
		c.types = kop.TCreature // CR 708.2
	} else {
		c.types = k.cards[cid].types
	}
	// a token copy of a face-down permanent copies the face-down values, and
	// its exceptions still apply over them (CR 707.2, 707.9b)
	exc := !fd || s.blankCopy(sl)
	if exc && k.gen.cpxTy && s.fxM.has(sl) {
		c.types |= s.copyExcTypes(k, sl) // layer 1: a copy's added types
		if k.gen.cpxNoLeg && s.copyExcNoLegendary(k, sl) {
			c.types &^= kop.TLegendary // NonLegendary$ (copy_nonlegendary.go)
		}
	}
	if p.ovr.flags&(ovTypes|ovTypesEOT) != 0 {
		c.types = p.ovr.types
	}
	if !fd {
		c.cols = cd.Colors
		c.subs = cd.Subtypes
	}
	if s.fxM.has(sl) && k.gen.cpxSub {
		c.subs = c.subs.Or(s.copyExcSubs(k, sl)) // layer 1: a copy's added subtype
	}
	if exc && s.fxM.has(sl) && k.gen.cpxCol {
		c.cols = s.copyExcCol(k, sl, c.cols) // layer 1: a copy's colour exceptions
	}
	// layer 4: the changeling CDA first (CR 613.3), then the effects in
	// dependency order
	c.all = !fd && cd.AllCreatureTypes || exc && s.fxM.has(sl) && k.gen.cpxSub && s.copyExcAll(k, sl)
	var done [maxLfx]bool
	for left := n; left > 0; left-- {
		pick := -1
		for i := 0; i < n && pick < 0; i++ {
			if !done[i] && (!l4part(&fx[i]) || s.lfxIndependent(k, sl, fx, n, &done, i, &c)) {
				pick = i
			}
		}
		if pick < 0 {
			for i := 0; i < n; i++ {
				if !done[i] {
					pick = i
					break
				}
			}
		}
		done[pick] = true
		e := &fx[pick]
		if !l4part(e) {
			continue
		}
		if s.lfxApplies(k, sl, e, &c) {
			e.ok = 1
			lfxApply4(k, e, &c)
		} else {
			e.ok = 2
		}
	}
	if upTo == upTo4 {
		return c
	}
	// layer 5: colour setters by timestamp
	for i := 0; i < n; i++ {
		if e := &fx[i]; e.o != nil && (e.o.HasColors || e.o.AddColors != 0) && s.lfxLocked(k, sl, e, &c) {
			if e.o.HasColors {
				c.cols = e.o.SetColors
			}
			if e.o.AddColors != 0 {
				c.cols = animAddColors(c.cols, e.o) // static_animategrants.go
			}
		}
	}
	if upTo == upTo5 {
		return c
	}
	// layer 7a/7b: the printed P/T or its CDA, a copy's set P/T (layer 1:
	// Offspring; copy_exceptions.go), then setters by timestamp
	c.bp, c.bt = s.basePT7a(k, sl)
	for i := 0; i < n; i++ {
		if e := &fx[i]; e.o != nil && e.o.HasPT && s.lfxLocked(k, sl, e, &c) {
			ep, et := e.p, e.t
			if e.o.LivePT {
				if e.a != nil {
					ep, et = s.livePTSrc(k, e.src, e.o) // an override static anchors on its source
				} else {
					if s.livePTLost(k, sl, e.ts) {
						continue
					}
					ep, et = s.livePT(k, sl, e.o, ep) // static_animategrants.go
				}
			}
			if !e.o.KeepP {
				c.bp = int(ep)
			}
			if !e.o.KeepT {
				c.bt = int(et)
			}
		}
	}
	return c
}

// lfxLocked: e applies in a layer after 4 (CR 613.6: an effect that
// applied in layer 4 keeps applying; one without a layer-4 part is matched
// against the finished types).
func (s *State) lfxLocked(k *prep, sl Slot, e *lfx, c *dchars) bool {
	if e.ok != 0 {
		return e.ok == 1
	}
	return s.lfxApplies(k, sl, e, c)
}

// lfxIndependent is gorge's independent(i) (types.go): no other remaining
// layer-4 effect j, applied to the current characteristics together with
// every remaining effect its application newly lets apply (never i),
// changes whether i applies.
func (s *State) lfxIndependent(k *prep, sl Slot, fx *[maxLfx]lfx, n int, done *[maxLfx]bool, i int, cur *dchars) bool {
	if fx[i].a == nil {
		return true // a record always applies
	}
	base := s.lfxApplies(k, sl, &fx[i], cur)
	for j := 0; j < n; j++ {
		if j == i || done[j] || !l4part(&fx[j]) {
			continue
		}
		sim := *cur
		if !s.lfxApplies(k, sl, &fx[j], &sim) {
			continue
		}
		var applied [maxLfx]bool
		lfxApply4(k, &fx[j], &sim)
		applied[j] = true
		for grew := true; grew; {
			grew = false
			for q := 0; q < n; q++ {
				if applied[q] || done[q] || q == i || !l4part(&fx[q]) {
					continue
				}
				if s.lfxApplies(k, sl, &fx[q], &sim) {
					lfxApply4(k, &fx[q], &sim)
					applied[q] = true
					grew = true
				}
			}
		}
		if s.lfxApplies(k, sl, &fx[i], &sim) != base {
			return false
		}
	}
	return true
}

// basePT7a is sl's layer-7a base: the printed P/T, its characteristic-
// defining ability unless sl lost it, and a copy's set P/T.
func (s *State) basePT7a(k *prep, sl Slot) (int, int) {
	p := &s.perms[sl]
	cid := s.cardAt(sl)
	pc := &k.cards[cid]
	bp, bt := int(pc.power), int(pc.tough)
	if s.faceDown(k, sl) {
		bp, bt = 2, 2 // CR 708.2
	} else if k.gen.has(cid, gfCDAPT) && !s.genLost(k, sl) {
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCDAPT {
				ctx := evalCtx{src: p.obj, srcSl: sl, ctrl: p.ctrl}
				bp, bt = s.evalV(k, a.Static.VA, ctx), s.evalV(k, a.Static.VB, ctx)
			}
		}
	}
	if s.fxM.has(sl) && k.gen.cpxPT {
		// layer 1: a copy's set P/T (CR 707.9b, 707.9d; copy_exceptions.go)
		if cp, ct, ok := s.copyExcPT(k, sl); ok {
			bp, bt = cp, ct
		}
	}
	if p.ovr.flags&(ovSetPT|ovSetPTEOT) != 0 {
		bp, bt = int(p.ovr.setP), int(p.ovr.setT)
	}
	return bp, bt
}

// hasSubCur reports whether c has matchup subtype i (subOutside: a
// creature type outside the matchup's subtypes).
func hasSubCur(k *prep, c *dchars, i uint8) bool {
	if outsideSub(k, i) {
		return c.all
	}
	if i == 255 { // none chosen; an outside code was handled above
		return false
	}
	return c.subs.Has(int(i)) || c.all && k.gen.creatSub.Has(int(i))
}

// The walk's readers (typesGen, colorsGen, hasSubGen, basePTGen call them
// behind their fxNeed branch).
func (s *State) walkTypes(k *prep, sl Slot) kop.TypeMask {
	c := s.chars(k, sl, upTo4)
	return c.types
}

func (s *State) walkColors(k *prep, sl Slot) kop.ColorMask {
	c := s.chars(k, sl, upTo5)
	return c.cols
}

func (s *State) walkHasSub(k *prep, sl Slot, i uint8) bool {
	c := s.chars(k, sl, upTo4)
	return hasSubCur(k, &c, i)
}

func (s *State) walkBasePT(k *prep, sl Slot) (int, int) {
	c := s.chars(k, sl, upTo7b)
	return c.bp, c.bt
}

// matchPermCur is matchPermGen inside the walk: sl's types, subtypes and
// colours are read from cur (the characteristics built so far), never by
// a fresh derivation of sl. Reads the walk cannot serve from cur (a
// keyword gate, a P/T predicate) use sl's printed values; kgen refuses an
// override static whose filter has them.
func (s *State) matchPermCur(k *prep, f kop.FilterID, sl Slot, ctrl uint8, src Slot, cur *dchars) bool {
	for f != 0 {
		if s.matchCurOne(k, &k.t.Filters[f], sl, ctrl, src, cur) {
			return true
		}
		f = k.t.Filters[f].Or
	}
	return false
}

func (s *State) matchCurOne(k *prep, fl *kop.Filter, sl Slot, ctrl uint8, src Slot, cur *dchars) bool {
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
		if fl.Flags&kop.FOther != 0 && sl == src || fl.Flags&kop.FSelf != 0 && sl != src {
			return false
		}
		if fl.Flags&kop.FAttachedBySrc != 0 && (src == NoSlot || s.perms[src].attached != sl) {
			return false
		}
		if fl.Flags&kop.FNotAttachedBySrc != 0 && src != NoSlot && (s.perms[src].attached == sl) != DebugNotAttachedPositive {
			return false // filter_notattached.go
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
	if fl.Types != 0 && cur.types&fl.Types == 0 || cur.types&fl.NotTypes != 0 {
		return false
	}
	if fl.Colors != 0 && cur.cols&fl.Colors == 0 || cur.cols&fl.NotColors != 0 {
		return false
	}
	if fl.Sub != 0 && !hasSubCur(k, cur, fl.Sub-1) || fl.NotSub != 0 && hasSubCur(k, cur, fl.NotSub-1) {
		return false
	}
	for _, n := range fl.NotSubs {
		if n != 0 && hasSubCur(k, cur, n-1) {
			return false
		}
	}
	if f2 := fl.Flags2; f2 != 0 {
		if f2&(kop.F2ChosenType|kop.F2ChosenColor) != 0 {
			if src == NoSlot {
				return false
			}
			ch := s.perms[src].chosen
			if f2&kop.F2ChosenType != 0 && (ch == 255 || !hasSubCur(k, cur, uint8(ch)) && !(outsideSub(k, uint8(ch)) && s.curOutsideHas(k, cur, src))) {
				return false
			}
			if f2&kop.F2ChosenColor != 0 && (ch > 4 || cur.cols&(1<<ch) == 0) {
				return false
			}
		}
		// keyword gates read printed keywords (layerKw): no re-entry
		if rest := f2 &^ (kop.F2ChosenType | kop.F2ChosenColor); rest != 0 && !s.flags2Perm(k, rest, sl, src, layerKw) {
			return false
		}
	}
	switch fl.Pred {
	case kop.PredNone:
		return true
	case kop.PredSubtype:
		return hasSubCur(k, cur, uint8(fl.PredN))
	case kop.PredPowerLE, kop.PredPowerGE, kop.PredToughnessLE, kop.PredToughnessGE, kop.PredToughnessLEV, kop.PredPowerLTToughness, kop.PredHasKeyword, kop.PredPowerGTBase:
		return false // refused by kgen on a walk filter (no re-entry)
	}
	if k.w8 && fl.Pred == kop.PredWord7 && fl.PredN&0xff >= kop.W7FaceUp {
		return s.wordPerm2(k, fl, sl, ctrl, src) // the static's controller and source
	}
	if fl.Pred == kop.PredAttached {
		return s.predAttached(sl, src)
	}
	return s.predPermGen(k, fl, sl, layerKw)
}

// walkAllOn: a global override static or a subtype lord is in force, so
// refreshTy keeps every occupied slot's walked types in perm.gty.
func (s *State) walkAllOn(k *prep) bool {
	for m := s.statM; m.any(); m = m.rest() {
		c := s.cardAt(m.first())
		if k.gen.has(c, gfGlobOvr) || k.gen.fxNeed&fxNeedSubGrant != 0 && k.gen.has(c, gfSubGrant) {
			return true
		}
	}
	return false
}

// globalGrants calls f for the abilities global override statics grant sl
// (Ygra's Food ability): each static whose filter matches sl's derived
// characteristics.
func (s *State) globalGrants(k *prep, sl Slot, lost bool, lts uint16, f func(c CardID, ab uint8, n int, ts uint16)) {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		cid := s.cardAt(src)
		if !k.gen.has(cid, gfGlobOvr) {
			continue
		}
		if lost && s.layerTs(src) < lts {
			continue
		}
		srcLost := s.genLost(k, src)
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatOverride || a.Static.Override == 0 {
				continue
			}
			if srcLost && !keptOnLoss(k.t, a) {
				continue // CR 613.6: the grant of a type-setting static outlives the loss with it
			}
			o := &k.t.Overrides[a.Static.Override-1]
			if o.Grant[0] == 0 && o.Grant[1] == 0 {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) {
				continue
			}
			c := s.chars(k, sl, upTo5)
			if !s.matchPermCur(k, a.Static.Filter, sl, st.ctrl, src, &c) {
				continue
			}
			for _, g := range o.Grant {
				if g != 0 {
					f(cid, g-1, 1, s.layerTs(src))
				}
			}
		}
	}
}

// filterAttachedOnly reports whether every alternative of filter f is
// limited to the source's host (an Aura's "enchanted creature").
func filterAttachedOnly(t *kop.Table, f kop.FilterID) bool {
	if f == 0 {
		return false
	}
	for ; f != 0; f = t.Filters[f].Or {
		if t.Filters[f].Flags&kop.FAttachedBySrc == 0 {
			return false
		}
	}
	return true
}

// filterHostOnly is filterAttachedOnly with nothing asked of the host beyond
// being a creature or a permanent: an override under it holds as long as the
// source is attached. "As long as it's a nonDetective" is read by the layer walk
// instead (Burden of Proof).
func filterHostOnly(t *kop.Table, f kop.FilterID) bool {
	if !filterAttachedOnly(t, f) {
		return false
	}
	for ; f != 0; f = t.Filters[f].Or {
		fl := &t.Filters[f]
		if fl.Sub != 0 || fl.NotSub != 0 || fl.Ctrl != 0 || fl.Colors != 0 || fl.NotColors != 0 || fl.Pred != 0 {
			return false
		}
	}
	return true
}

// filterReadsSub reports whether a filter reads subtypes.
func filterReadsSub(t *kop.Table, f kop.FilterID) bool {
	for ; f != 0; f = t.Filters[f].Or {
		fl := &t.Filters[f]
		if fl.Sub != 0 || fl.NotSub != 0 || fl.Pred == kop.PredSubtype || fl.Flags2&kop.F2ChosenType != 0 {
			return true
		}
	}
	return false
}

// prepLayerWalk sets fxNeedSubGrant: a Continuous subtype grant whose
// filter reads subtypes (a type lord over a granted type) must see the
// layer-4 result, so every subtype read walks. It sets fxNeedSubSet when a
// subtype grant and a type or subtype setter can meet on one permanent:
// a grant reaching others beside any setter, or a grant on its own
// source beside a setter that reaches others without removing abilities
// (a lose-all setter takes the source's grant with it) or one the same
// card makes on itself.
func prepLayerWalk(t *kop.Table, g *genPrep) {
	setter := func(o uint16) bool { // a 1-based Overrides index
		if o == 0 || int(o) > len(t.Overrides) {
			return false
		}
		ov := &t.Overrides[o-1]
		return ov.HasTypes || ov.HasSub || ov.RemoveCreatureSub
	}
	var grantOther, setAny, setOtherKeep bool
	var grantSelf, setSelf []bool
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && (a.Static.AddSub.Any() || a.Static.AddChosen) {
				g.setFlag(CardID(i), gfSubGrant)
				if filterReadsSub(t, a.Static.Filter) {
					g.fxNeed |= fxNeedSubGrant
				}
				if filterSelfOnly(t, a.Static.Filter) {
					if grantSelf == nil {
						grantSelf = make([]bool, len(t.Cards))
					}
					grantSelf[i] = true
				} else {
					grantOther = true
				}
			}
			mark := func(o uint16, self bool) {
				if !setter(o) {
					return
				}
				setAny = true
				if self {
					if setSelf == nil {
						setSelf = make([]bool, len(t.Cards))
					}
					setSelf[i] = true
				} else if !t.Overrides[o-1].LoseAbilities {
					setOtherKeep = true
				}
			}
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
				mark(a.Static.Override, filterSelfOnly(t, a.Static.Filter))
			}
			for pi := range a.Prog {
				in := &a.Prog[pi]
				switch in.Op {
				case kop.OpOverride:
					if in.B.N == kop.OvChosenColor {
						// C..C+4 are the five
						// colour records (one will apply at resolution)
						for n := uint16(0); n < 5; n++ {
							mark(uint16(in.C)+n+1, in.Obj == kop.RefSelf)
						}
					} else {
						mark(uint16(in.C)+1, in.Obj == kop.RefSelf)
					}
				case kop.OpOverrideAll:
					mark(uint16(in.C)+1, false)
				}
			}
		}
	}
	for _, e := range t.Emblems {
		for j := range e.Abilities {
			if a := &e.Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.AddSub.Any() {
				g.embSubGrant, grantOther = true, true
			}
		}
	}
	need := grantOther && setAny
	for i := range grantSelf {
		need = need || grantSelf[i] && (setOtherKeep || setSelf != nil && setSelf[i])
	}
	if need {
		g.fxNeed |= fxNeedSubSet
	}
}

// filterSelfOnly reports whether every alternative of filter f is the
// source itself.
func filterSelfOnly(t *kop.Table, f kop.FilterID) bool {
	if f == 0 {
		return false
	}
	for ; f != 0; f = t.Filters[f].Or {
		if t.Filters[f].Flags&kop.FSelf == 0 {
			return false
		}
	}
	return true
}

// grantSrcLost reports whether a subtype grant's source has lost its
// abilities, without walking (genLost walks the source on a walk slot, and
// the source's own walk collects this grant: Skyknight Squire under two
// Auras recursed). A one-shot loss, a lose-all record, an attached
// lose-all static, or a global creature-only lose-all static over the
// source's last settled types (perm.gty) counts.
func (s *State) grantSrcLost(k *prep, src Slot) bool {
	p := &s.perms[src]
	if p.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return true
	}
	if k.gen.anyLoseRec && s.fxM.has(src) {
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.sl == src && r.kind == fxOverride && k.t.Overrides[r.r.idx-1].LoseAbilities {
				return true
			}
		}
	}
	if !k.gen.anyLoseStat {
		return false
	}
	ty := k.cards[s.cardAt(src)].types
	if p.ovr.flags&ovGenTy != 0 {
		ty = p.gty &^ (gtyFaceDown | gtyProvMask)
	}
	for m := s.statM; m.any(); m = m.rest() {
		ls := m.first()
		c := s.cardAt(ls)
		if !k.gen.has(c, gfLoseStat) || s.perms[ls].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		if s.perms[ls].attached == src && ls != src || k.gen.has(c, gfGlobOvr) && ty&kop.TCreature != 0 {
			return true
		}
	}
	return false
}

// ovStaticOn reports an override static attached to sl, one whose timestamp
// orders against sl's override records.
func (s *State) ovStaticOn(k *prep, sl Slot) bool {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.perms[src].attached == sl && src != sl && k.gen.has(s.cardAt(src), gfOvrStat) {
			return true
		}
	}
	return false
}
