package dirac

import (
	"math/bits"
	"strconv"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Set-valued operands (design 2026-11-19-dirac-operand-algebras K3; kop
// setexpr.go): an instruction whose Obj is kop.RefSet names a kop.SetExpr
// through its ability's SetRefs. evalSet evaluates it on the Go stack (0 State
// bytes, no allocation); execSet runs the op once per member. The arm is cold:
// tableUnion counts a RefSet subject as a union one (unionMask), so a table
// that names no set never reaches it, and the operand read of every other op
// gains no branch.

// setMaxOrd is the longest ordered member list a set keeps: a target union's
// slots and their extra objects.
const setMaxOrd = kop.MaxTargets + maxXTgt

// setOrd is a target base's member order (gorge's chainTargetUnion slot
// order), with each member's version as the target recorded it.
type setOrd struct {
	c   [setMaxOrd]TCode
	ver [setMaxOrd]uint8
	n   uint8
	on  bool
}

// setVal is one evaluated set: objects as a bit set over MaxObj, players as a
// seat mask, and the order a target base gives them (ord.on), which only
// SetAndFilter keeps.
type setVal struct {
	objs objBits
	pl   uint8
	ord  setOrd
}

// setCtx is the context a set is evaluated in: the reading source, its
// controller and the stack entry whose targets and trigger referents the
// bases read (nil: none).
type setCtx struct {
	ev evalCtx
}

func (v *setVal) addObj(o Obj, ver uint8, ordered bool) {
	if o >= MaxObj || v.objs.has(o) {
		return
	}
	v.objs.set(o)
	if ordered {
		v.ord.c[v.ord.n], v.ord.ver[v.ord.n] = TCode(o), ver
		v.ord.n++
	}
}

func (v *setVal) addPl(p uint8, ordered bool) {
	if p >= MaxSeats || v.pl&(1<<p) != 0 {
		return
	}
	v.pl |= 1 << p
	if ordered {
		v.ord.c[v.ord.n], v.ord.ver[v.ord.n] = PlayerCode(p), 0
		v.ord.n++
	}
}

// setRefOf is the Table.Sets index ab's instruction at pc names.
func setRefOf(ab *kop.Ability, pc int) uint16 {
	for _, r := range ab.SetRefs {
		if int(r.PC) == pc {
			return r.Set
		}
	}
	panic("kernel: a RefSet instruction at pc " + strconv.Itoa(pc) + " with no SetRefs entry")
}

// evalSet evaluates the set ab's instruction at pc names, in ctx. It is read
// afresh every time the instruction runs: a resolution that suspended and
// resumes re-evaluates it, as gorge re-reads Defined$ when the op runs.
func (s *State) evalSet(k *prep, ab *kop.Ability, pc int, ctx setCtx) setVal {
	return s.evalSetExpr(k, k.t.Sets[setRefOf(ab, pc)], ctx)
}

// evalSetExpr runs postfix program e on a fixed operand stack.
func (s *State) evalSetExpr(k *prep, e kop.SetExpr, ctx setCtx) setVal {
	var st [kop.MaxSetDepth]setVal
	n := 0
	for _, op := range e {
		switch {
		case op.Kind.SetPushes():
			if n == len(st) {
				panic("kernel: a SetExpr deeper than kop.MaxSetDepth")
			}
			st[n] = setVal{}
			s.setBase(k, op, ctx, &st[n])
			n++
		case op.Kind == kop.SetUnion:
			if n < 2 {
				panic("kernel: SetUnion over fewer than two sets")
			}
			setUnion(&st[n-2], &st[n-1])
			n--
		default:
			if n < 1 {
				panic("kernel: " + op.Kind.String() + " over an empty set stack")
			}
			s.setApply(k, op, ctx, &st[n-1])
		}
	}
	if n != 1 {
		panic("kernel: a SetExpr that leaves " + strconv.Itoa(n) + " sets")
	}
	return st[0]
}

// setBase pushes base op's set into v.
func (s *State) setBase(k *prep, op kop.SetOp, ctx setCtx, v *setVal) {
	e := ctx.ev.e
	switch op.Kind {
	case kop.SetTarget:
		s.setTargets(k, e, 1<<op.Arg, v)
	case kop.SetTargets:
		s.setTargets(k, e, uint8(op.Arg), v)
	case kop.SetSelf:
		if o := ctx.ev.src; o != NoObj {
			v.addObj(o, 0, false)
		}
	case kop.SetController:
		v.addPl(ctx.ev.ctrl, false)
	case kop.SetTriggerObj:
		if setTrigEntry(e) {
			v.addObj(s.refObjCtx(kop.RefTriggerObj, ctx.ev), 0, false)
		}
	case kop.SetTriggerSource:
		if setTrigEntry(e) {
			v.addObj(s.refObjCtx(kop.RefTriggerSource, ctx.ev), 0, false)
		}
	case kop.SetTriggerPlayer:
		if setTrigEntry(e) {
			v.addPl(e.trigPl, false)
		}
	case kop.SetEachOpponent:
		q := s.oppsFrom(ctx.ev.ctrl)
		for i := uint8(0); i < q.n; i++ {
			v.addPl(q.s[i], false)
		}
	case kop.SetEachPlayer:
		q := s.aliveFrom(ctx.ev.ctrl)
		for i := uint8(0); i < q.n; i++ {
			v.addPl(q.s[i], false)
		}
	case kop.SetRefBase:
		s.setRefBase(k, op.Arg, v)
	case kop.SetBattlefield:
		// phased-out permanents are off occ, as gorge's sweep skips them
		for m := s.occ; m.any(); m = m.rest() {
			v.addObj(s.perms[m.first()].obj, 0, false)
		}
	default:
		panic("kernel: evalSet: base " + op.Kind.String() + " is declared but not modelled (its own ticket)")
	}
}

// setRefBase pushes into v the one object or player the Ref arg names, read as
// an op's own subject read of it (refCode, so from the resolving entry: the
// context every set is evaluated in); a Ref that names nobody pushes nothing.
func (s *State) setRefBase(k *prep, arg uint16, v *setVal) {
	r := kop.Ref(arg)
	if arg > 0xff || !kop.SetRefBaseOK(r) {
		panic("kernel: evalSet: SetRefBase over Ref " + strconv.Itoa(int(arg)) + ", which is not one admitted referent")
	}
	c := s.refCode(k, r)
	var ver uint8
	if isObjTarget(c) {
		ver = s.objs[c].ver
	}
	v.addCode(c, ver) // NoTarget names neither an object nor a player
}

// setTrigEntry reports whether e is a triggered ability's entry, the only
// kind whose trigObj and trigPl are the firing event's referents: a spell's
// carry its cast bookkeeping (the cast mode, a Discard cost's card), and
// gorge's Triggered referents name nothing outside a trigger.
func setTrigEntry(e *stackEntry) bool { return e != nil && e.kind&seFamMask == seTrigger }

// setTargets adds the legal targets of e's slots in mask m, slot by slot,
// each slot's first object then its extra objects (execUnion's order): a
// target illegal on resolution is left out (CR 608.2b). A stack ability
// target names no object or player and is skipped.
func (s *State) setTargets(k *prep, e *stackEntry, m uint8, v *setVal) {
	if e == nil {
		return
	}
	v.ord.on = true
	res := e == &s.res.e
	xt := s.xtOf(e)
	for i := uint8(0); i < kop.MaxTargets; i++ {
		if m&(1<<i) == 0 {
			continue
		}
		c, ver := e.tgt[i], e.tver[i]
		if res {
			c = s.tgtCode(int(i))
		}
		v.addCode(c, ver)
		for j := 0; j < int(xt.n); j++ {
			if xt.slot[j] != i {
				continue
			}
			if res && s.xtLegalRes(k, e, j, i) || !res && s.xtLegal(k, e, j) {
				v.addCode(xt.code[j], xt.ver[j])
			}
		}
	}
}

func (v *setVal) addCode(c TCode, ver uint8) {
	switch {
	case isObjTarget(c):
		v.addObj(Obj(c), ver, true)
	case isPlayerTarget(c):
		v.addPl(CodeSeat(c), true)
	}
}

// setUnion folds b into a. A union keeps no slot order: its members walk in
// object index order, then seat order.
func setUnion(a, b *setVal) {
	for i := range a.objs {
		a.objs[i] |= b.objs[i]
	}
	a.pl |= b.pl
	a.ord.on = false
}

// setApply applies the one-operand combinator op to v in place.
func (s *State) setApply(k *prep, op kop.SetOp, ctx setCtx, v *setVal) {
	switch op.Kind {
	case kop.SetAndFilter:
		f := kop.FilterID(op.Arg)
		for w := range v.objs {
			for b := v.objs[w]; b != 0; b &= b - 1 {
				o := Obj(w*64 + bits.TrailingZeros64(b))
				if !s.setMatch(k, f, o, ctx) {
					v.objs.clr(o)
				}
			}
		}
		v.pl = 0
		if v.ord.on {
			n := uint8(0)
			for i := uint8(0); i < v.ord.n; i++ {
				if c := v.ord.c[i]; isObjTarget(c) && v.objs.has(Obj(c)) {
					v.ord.c[n], v.ord.ver[n] = c, v.ord.ver[i]
					n++
				}
			}
			v.ord.n = n
		}
	case kop.SetPlayersComplement:
		var all uint8
		for p := uint8(0); p < s.nSeats(); p++ {
			if !s.out(p) {
				all |= 1 << p
			}
		}
		pl := all &^ v.pl
		*v = setVal{pl: pl}
	case kop.SetControllerOf, kop.SetOwnerOf:
		pl := v.pl
		for w := range v.objs {
			for b := v.objs[w]; b != 0; b &= b - 1 {
				o := Obj(w*64 + bits.TrailingZeros64(b))
				p := s.objs[o].owner()
				if op.Kind == kop.SetControllerOf {
					p = s.ctrlOfObj(o)
				}
				if p < MaxSeats {
					pl |= 1 << p
				}
			}
		}
		*v = setVal{pl: pl}
	default:
		panic("kernel: evalSet: combinator " + op.Kind.String() + " is declared but not modelled (its own ticket)")
	}
}

// setMatch reports whether object o matches filter f read from ctx's source:
// a permanent through the permanent matcher, any other object through the
// card matcher over its card and owner.
func (s *State) setMatch(k *prep, f kop.FilterID, o Obj, ctx setCtx) bool {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield {
		return s.matchPerm(k, f, ob.slot, ctx.ev.ctrl, ctx.ev.srcSl)
	}
	if k.gen == nil {
		return s.matchCard(k, f, ob.card, ob.owner(), ctx.ev.ctrl)
	}
	return s.matchCardGenAt(k, f, ob.card, ob.owner(), ctx.ev.ctrl, o, &ctx.ev)
}

// setMembers lists v's members in the order an op walks them: a target base's
// slot order, else object index order, then seats in turn order from ctrl.
// out's versions are the target's recorded one for an ordered member and the
// object's current one otherwise.
func (s *State) setMembers(v *setVal, ctrl uint8, out *[MaxObj + MaxSeats]TCode, ver *[MaxObj + MaxSeats]uint8) int {
	n := 0
	if v.ord.on {
		for i := uint8(0); i < v.ord.n; i++ {
			out[n], ver[n] = v.ord.c[i], v.ord.ver[i]
			n++
		}
		return n
	}
	for w := range v.objs {
		for b := v.objs[w]; b != 0; b &= b - 1 {
			o := Obj(w*64 + bits.TrailingZeros64(b))
			out[n], ver[n] = TCode(o), s.objs[o].ver
			n++
		}
	}
	ns := s.nSeats()
	for i := uint8(0); i < ns; i++ {
		p := ctrl + i
		if p >= ns {
			p -= ns
		}
		if v.pl&(1<<p) != 0 {
			out[n], ver[n] = PlayerCode(p), 0
			n++
		}
	}
	return n
}

// setOps are the ops that take a RefSet subject (the design's 12, the ones
// with the most Defined$ refusal carriers): ChangeZone's moves, Pump,
// PutCounter, DealDamage, Destroy, Sacrifice, Tap/Untap, GainLife/LoseLife,
// Draw, Discard, GainControl and Animate.
func setOp(op kop.Op) bool {
	switch op {
	case kop.OpExile, kop.OpBounce, kop.OpReturn, kop.OpToLibrary,
		kop.OpPumpUntilEOT, kop.OpPutCounter, kop.OpDealDamage, kop.OpDestroy, kop.OpSacrifice,
		kop.OpTap, kop.OpUntap, kop.OpGainLife, kop.OpLoseLife, kop.OpDraw, kop.OpDiscard,
		kop.OpGainControl, kop.OpGainControlEOT, kop.OpAnimate:
		return true
	}
	return false
}

// setBindSlot is the target slot execSet binds each member to: the last one,
// which an ability that leaves it free never reads.
const setBindSlot = kop.MaxTargets - 1

// setSeatPlayers is a walk's member list as a seat sequence (the order the
// walk takes them), and whether every member is a player: the shape gorge's
// effSacrifice and effDiscard resolve over Defined$ Player.
func setSeatPlayers(list []TCode) (q seatSeq, players bool) {
	for _, c := range list {
		if !isPlayerTarget(c) {
			return seatSeq{}, false
		}
		q.add(CodeSeat(c))
	}
	return q, len(list) > 0
}

// execSet runs in, whose subject is kop.RefSet, once per member of its set,
// each member bound in turn to the free target slot setBindSlot (the way
// execUnion binds a union's objects): an object or a player, so each op's
// own subject read (objRef, playerRef, refCode) answers it. The set is
// evaluated once before the first run, as gorge resolves Defined$ once per op.
// A member whose run suspends (an ask) ends the walk with the binding in
// place for the answer; the members after it are not run. The all-player
// Sacrifice and Discard walk is the exception gorge itself draws: over
// Defined$ Player those two effects ask every player before anything moves
// (CR 101.4), so the members still to come ride the each-player repNext
// chain and the ask's answer runs the walk on (setSeatPlayers).
//
//go:noinline
func (s *State) execSet(k *prep, in *kop.Instr) bool {
	if !setOp(in.Op) {
		panic("kernel: a RefSet subject under " + in.Op.String() + ", an op that does not take a set")
	}
	e := &s.res.e
	ab := &k.t.Cards[e.card].Abilities[e.ab]
	pc := int(s.res.pc)
	if pc >= len(ab.Prog) || ab.Prog[pc].Obj != kop.RefSet || ab.Prog[pc].Op != in.Op {
		panic("kernel: a RefSet instruction outside its ability's program")
	}
	if ab.NTargets > setBindSlot {
		panic("kernel: a RefSet instruction in an ability that targets every slot")
	}
	v := s.evalSet(k, ab, pc, setCtx{ev: s.ctxEntry(e)})
	var list [MaxObj + MaxSeats]TCode
	var vers [MaxObj + MaxSeats]uint8
	n := s.setMembers(&v, e.ctrl, &list, &vers)
	if st := DebugSetStats; st != nil {
		st.Runs++
		st.Members += uint64(n)
	}
	if n == 0 {
		return true
	}
	// A Sacrifice or Discard whose members are all players is gorge's
	// effSacrifice and effDiscard over Defined$ Player: each player answers
	// in turn order from the controller and everything they chose moves at
	// once (CR 101.4, flushSac, flushDiscards). Running it one member at a
	// time would sacrifice the earlier picks before the later players chose
	// and drop every member after a suspension, so the seats still to come
	// ride the each-player repNext chain: the ask's answer pops the next
	// seat and asks it (interp_ask.go's raGenSac, act_eachdiscard.go's
	// raDiscard), and the members already run are skipped with the chain.
	if q, players := setSeatPlayers(list[:n]); players {
		switch {
		case in.Op == kop.OpSacrifice && in.C&kop.OptAsk == 0:
			s.repRest(q, 1)
			return s.startSac(k, in, q.at(0))
		case in.Op == kop.OpDiscard && in.F == 0 && in.C == kop.ChooseSelf:
			return s.discardSeq(k, in, q)
		}
	}
	saveT, saveV, saveBad, saveLoop := e.tgt[setBindSlot], e.tver[setBindSlot], s.res.bad, s.g.inLoop
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	hr := s.holdOpen(k)
	defer s.holdClose(k, hr)
	s.g.inLoop = true
	one := *in
	one.Obj = tgtRef(setBindSlot)
	for j := 0; j < n && !s.over; j++ {
		e.tgt[setBindSlot], e.tver[setBindSlot] = list[j], vers[j]
		s.res.bad &^= 1 << setBindSlot
		if !s.exec(k, &one) {
			s.g.inLoop = saveLoop
			return false
		}
	}
	e.tgt[setBindSlot], e.tver[setBindSlot], s.res.bad, s.g.inLoop = saveT, saveV, saveBad, saveLoop
	return true
}
