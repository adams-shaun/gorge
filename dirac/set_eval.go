package dirac

import (
	"slices"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// SetExpr evaluation (kop.SetExpr, dirac/kop/sets.go): the one way the kernel
// names any set of objects or players an effect acts on. evalSet runs a set's
// postfix program on the Go stack and iteration (set_iter.go) visits the
// members; nothing here is State and nothing allocates.
//
// A members list is exact: an object a base names is a member whether or not
// it is still where the base read it, and the member body (set_ops.go) applies
// its own liveness check, as the single-object paths do. SoLKI (or SrLKI on
// the instruction) only changes how a member that left the battlefield is read
// by the ops that follow it: its controller is the departed slot's record, not
// the owner.

// setMax is the largest set evalSet holds. A bigger one is StallSetOverflow.
const setMax = MaxPerm

// StallSetOverflow is a set of more than setMax members (evalSet).
const StallSetOverflow = 74

func init() { stallNames[StallSetOverflow] = "setoverflow" }

// setVal is one evaluated set: the members in order, objects or (isPl) seats.
// It is 390 B and lives on the Go stack, never in State.
type setVal struct {
	n    uint16
	pl   uint8 // isPl: the member seats as a bit mask
	isPl bool
	lki  bool // read through last known information
	objs [setMax]Obj
}

func (v *setVal) reset(players, lki bool) {
	v.n, v.pl, v.isPl, v.lki = 0, 0, players, lki
}

// has reports whether o (an object, or a seat of a players set) is a member.
func (v *setVal) has(o Obj) bool {
	if v.isPl {
		return v.pl&(1<<uint(o)) != 0
	}
	for i := uint16(0); i < v.n; i++ {
		if v.objs[i] == o {
			return true
		}
	}
	return false
}

// push appends o; a seat already a member of a players set adds nothing.
func (s *State) setPush(v *setVal, o Obj) {
	if v.isPl {
		if v.pl&(1<<uint(o)) != 0 {
			return
		}
		v.pl |= 1 << uint(o)
	}
	if int(v.n) == setMax {
		s.stall(StallSetOverflow)
		return
	}
	v.objs[v.n] = o
	v.n++
}

// setPushNew appends o unless it is a member.
func (s *State) setPushNew(v *setVal, o Obj) {
	if !v.has(o) {
		s.setPush(v, o)
	}
}

// setPushSeats appends the seats of q.
func (s *State) setPushSeats(v *setVal, q seatSeq) {
	for i := uint8(0); i < q.n; i++ {
		s.setPush(v, Obj(q.s[i]))
	}
}

// evalSet runs set id in ctx into out. The program keeps at most
// kop.SetMaxDepth sets: one reusable array of them, local to this call.
func (s *State) evalSet(k *prep, id kop.SetID, ctx evalCtx, out *setVal) {
	s.evalSetAs(k, id, ctx, 0, out)
}

// evalSetAs is evalSet under an instruction's SetRef.Flags: SrLKI reads every
// base through last known information, SrOncePerController keeps each
// controller's first member only.
func (s *State) evalSetAs(k *prep, id kop.SetID, ctx evalCtx, fl uint8, out *setVal) {
	ex := &k.t.Sets[id]
	var st [kop.SetMaxDepth]setVal
	lki := fl&kop.SrLKI != 0
	sp := 0
	for i := range ex.Ops {
		op := &ex.Ops[i]
		switch op.Kind {
		case kop.SoBaseRef, kop.SoBaseValid:
			if sp == len(st) {
				panic("kernel: SetExpr deeper than kop.SetMaxDepth")
			}
			v := &st[sp]
			sp++
			if op.Kind == kop.SoBaseRef {
				v.reset(op.Arg != 0, lki)
				s.setBaseRef(k, op.R, ctx, v)
			} else {
				v.reset(false, lki)
				s.setBaseValid(k, op.F, kop.Zone(op.Arg), ctx, v)
			}
		case kop.SoInter:
			s.setInter(k, op.F, ctx, &st[sp-1])
		case kop.SoUnion:
			sp--
			s.setUnion(&st[sp-1], &st[sp])
		case kop.SoController, kop.SoOwner:
			s.setToPlayers(&st[sp-1], op.Kind == kop.SoOwner)
		case kop.SoComplementPlayers:
			s.setComplement(&st[sp-1], ctx)
		case kop.SoLKI:
			if st[sp-1].isPl {
				panic("kernel: SoLKI over a players set")
			}
			st[sp-1].lki = true
		default:
			panic("kernel: SetExpr op not implemented")
		}
	}
	if sp != 1 {
		panic("kernel: SetExpr leaves no single set")
	}
	v := &st[0]
	s.setOrder(v, ex.Order, ctx)
	if fl&kop.SrOncePerController != 0 {
		s.setOncePerController(v)
	}
	out.n, out.pl, out.isPl, out.lki = v.n, v.pl, v.isPl, v.lki
	copy(out.objs[:v.n], v.objs[:v.n])
}

// setBaseRef is SoBaseRef: the objects (or, with v.isPl, the players) the Ref
// names, through the readers the single-object ops use.
func (s *State) setBaseRef(k *prep, r kop.Ref, ctx evalCtx, v *setVal) {
	if v.isPl {
		s.setBasePlayers(k, r, ctx, v)
		return
	}
	switch r {
	case kop.RefRemembered:
		for _, o := range s.g.rem[:s.g.nrem] {
			if o != NoObj {
				s.setPush(v, o)
			}
		}
	case kop.RefChosen:
		for _, o := range s.g.chosen[:s.g.nchosen] {
			if o != NoObj {
				s.setPush(v, o)
			}
		}
	case kop.RefImprinted:
		if _, ok := s.impRefSrc(ctx); !ok {
			for _, o := range s.g.imp[:s.g.nimp] {
				if o != NoObj {
					s.setPush(v, o)
				}
			}
			return
		}
		if o := s.refObjCtx(r, ctx); o != NoObj {
			s.setPush(v, o) // another source's store: its first record (refObjCtx)
		}
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		s.setSlotMembers(k, ctx, uint8(sl), v)
	case kop.RefTargets01:
		s.setSlotMembers(k, ctx, 0, v)
		s.setSlotMembers(k, ctx, 1, v)
	case kop.RefTargets012:
		s.setSlotMembers(k, ctx, 0, v)
		s.setSlotMembers(k, ctx, 1, v)
		s.setSlotMembers(k, ctx, 2, v)
	case kop.RefTargets12:
		s.setSlotMembers(k, ctx, 1, v)
		s.setSlotMembers(k, ctx, 2, v)
	case kop.RefTargets0123:
		for i := uint8(0); i < 4; i++ {
			s.setSlotMembers(k, ctx, i, v)
		}
	case kop.RefLastCreated:
		if ctx.e == &s.res.e && s.res.lastTok != NoObj {
			s.setPush(v, s.res.lastTok)
		}
	default:
		if o := s.refObjCtx(r, ctx); o != NoObj {
			s.setPush(v, o)
		}
	}
}

// setSlotMembers appends target slot i's members of v's kind: the slot's
// object targets (its extra objects included), or its player targets, in pick
// order.
func (s *State) setSlotMembers(k *prep, ctx evalCtx, i uint8, v *setVal) {
	e := ctx.e
	if e == nil {
		return
	}
	if e == &s.res.e {
		var codes [maxXTgt + 1]TCode
		m := s.divTargets(k, e, i, &codes)
		for _, c := range codes[:m] {
			s.setPushTarget(v, c)
		}
		return
	}
	s.setPushTarget(v, e.tgt[i])
}

func (s *State) setPushTarget(v *setVal, c TCode) {
	switch {
	case c == NoTarget:
	case isPlayerTarget(c):
		if v.isPl {
			s.setPush(v, Obj(CodeSeat(c)))
		}
	case IsObjTarget(c):
		if !v.isPl {
			s.setPush(v, Obj(c))
		}
	}
}

// setBasePlayers is setBaseRef for a players set.
func (s *State) setBasePlayers(k *prep, r kop.Ref, ctx evalCtx, v *setVal) {
	switch r {
	case kop.RefEachPlayer:
		s.setPushSeats(v, s.aliveFrom(ctx.ctrl))
		return
	case kop.RefEachOpponent:
		s.setPushSeats(v, s.oppsFrom(ctx.ctrl))
		return
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		before := v.n
		s.setSlotMembers(k, ctx, uint8(sl), v)
		if v.n != before {
			return
		}
	case kop.RefTargets01, kop.RefTargets012, kop.RefTargets12, kop.RefTargets0123:
		s.setBaseTargetsPlayers(k, r, ctx, v)
		return
	}
	if ctx.e == &s.res.e {
		s.setPushSeats(v, s.eachPlayers(k, r)) // the resolving entry's readers, one home (interp_gen.go)
		return
	}
	if p := s.refPlayer(k, r, ctx); p != 255 {
		s.setPush(v, Obj(p))
	}
}

// setBaseTargetsPlayers is the player half of the multi-slot target refs.
func (s *State) setBaseTargetsPlayers(k *prep, r kop.Ref, ctx evalCtx, v *setVal) {
	lo, hi := uint8(0), uint8(1)
	switch r {
	case kop.RefTargets012:
		hi = 2
	case kop.RefTargets12:
		lo, hi = 1, 2
	case kop.RefTargets0123:
		hi = 3
	}
	for i := lo; i <= hi; i++ {
		s.setSlotMembers(k, ctx, i, v)
	}
}

// setBaseValid is SoBaseValid: every object matching f in zone z.
func (s *State) setBaseValid(k *prep, f kop.FilterID, z kop.Zone, ctx evalCtx, v *setVal) {
	if z == kop.ZoneBattlefield {
		var bf [MaxPerm]Slot
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			n := s.battlefieldOrder(p, s.ctrlM[p], bf[:], 0)
			for _, sl := range bf[:n] {
				if s.matchPerm(k, f, sl, ctx.ctrl, ctx.srcSl) {
					s.setPush(v, s.perms[sl].obj)
				}
			}
		}
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		pl := &s.pl[p]
		switch z {
		case kop.ZoneGraveyard:
			for i := 0; i < int(pl.graveN); i++ {
				s.setPushCard(k, f, pl.grave[i], p, ctx, v)
			}
		case kop.ZoneExile:
			for i := 0; i < int(pl.exileN); i++ {
				s.setPushCard(k, f, pl.exile[i], p, ctx, v)
			}
		case kop.ZoneHand:
			for i := 0; i < int(pl.handN); i++ {
				s.setPushCard(k, f, pl.hand[i], p, ctx, v)
			}
		case kop.ZoneLibrary:
			for i := int(pl.libLo); i < int(pl.libHi); i++ {
				s.setPushCard(k, f, pl.lib[i], p, ctx, v)
			}
		default:
			panic("kernel: SoBaseValid zone not implemented")
		}
	}
}

func (s *State) setPushCard(k *prep, f kop.FilterID, o Obj, owner uint8, ctx evalCtx, v *setVal) {
	if s.setMatchCard(k, f, o, owner, ctx.ctrl) {
		s.setPush(v, o)
	}
}

// setMatchCard matches the card o, not on the battlefield, against f.
func (s *State) setMatchCard(k *prep, f kop.FilterID, o Obj, owner, ctrl uint8) bool {
	if k.gen != nil {
		return s.matchCardGen(k, f, s.objs[o].card, owner, ctrl, o)
	}
	return s.matchCard(k, f, s.objs[o].card, owner, ctrl)
}

// setInter is SoInter: keep the members matching f. A member off the
// battlefield is matched as the card in its zone (the kernel's convention for
// a moved object, countRefMatches).
func (s *State) setInter(k *prep, f kop.FilterID, ctx evalCtx, v *setVal) {
	if v.isPl {
		panic("kernel: SoInter over a players set")
	}
	n := uint16(0)
	for i := uint16(0); i < v.n; i++ {
		o := v.objs[i]
		ob := &s.objs[o]
		var m bool
		if ob.zone == kop.ZoneBattlefield {
			m = s.matchPerm(k, f, ob.slot, ctx.ctrl, ctx.srcSl)
		} else {
			m = s.setMatchCard(k, f, o, ob.owner(), ctx.ctrl)
		}
		if m {
			v.objs[n] = o
			n++
		}
	}
	v.n = n
}

// setUnion is SoUnion: a keeps its members and takes b's new ones, in order.
func (s *State) setUnion(a, b *setVal) {
	if a.isPl != b.isPl {
		panic("kernel: SoUnion of an object set and a players set")
	}
	for i := uint16(0); i < b.n; i++ {
		s.setPushNew(a, b.objs[i])
	}
	a.lki = a.lki || b.lki
}

// setToPlayers is SoController / SoOwner: the members' controllers (owners),
// deduplicated, in first-seen order.
func (s *State) setToPlayers(v *setVal, owner bool) {
	if v.isPl {
		panic("kernel: SoController of a players set")
	}
	var seats [MaxSeats]uint8
	var m uint8
	n := 0
	for i := uint16(0); i < v.n; i++ {
		o := v.objs[i]
		var p uint8
		switch {
		case owner:
			p = s.objs[o].owner()
		case v.lki:
			p = s.lkiCtrlOf(o)
		default:
			p = s.ctrlOfObj(o)
		}
		if p < MaxSeats && m&(1<<p) == 0 {
			m |= 1 << p
			seats[n] = p
			n++
		}
	}
	v.reset(true, false)
	for _, p := range seats[:n] {
		s.setPush(v, Obj(p))
	}
}

// lkiCtrlOf is the controller of o as it last was: the live controller on the
// battlefield; off it, the newest departed slot record that still names o (the
// record keeps the controller it left under), else the owner.
func (s *State) lkiCtrlOf(o Obj) uint8 {
	if s.objs[o].zone == kop.ZoneBattlefield {
		return s.perms[s.objs[o].slot].ctrl
	}
	best, ts := uint8(255), uint32(0)
	for sl := Slot(0); sl < MaxPerm; sl++ {
		if p := &s.perms[sl]; p.obj == o && p.ts != 0 && !s.occ.has(sl) && uint32(p.ts) > ts {
			best, ts = p.ctrl, uint32(p.ts)
		}
	}
	if best != 255 {
		return best
	}
	return s.objs[o].owner()
}

// setComplement is SoComplementPlayers: the living players not in v, in seat
// order from the controller.
func (s *State) setComplement(v *setVal, ctx evalCtx) {
	if !v.isPl {
		panic("kernel: SoComplementPlayers of an object set")
	}
	mask := v.pl
	v.reset(true, false)
	q := s.aliveFrom(ctx.ctrl)
	for i := uint8(0); i < q.n; i++ {
		if mask&(1<<q.s[i]) == 0 {
			s.setPush(v, Obj(q.s[i]))
		}
	}
}

// setOrder puts v's members in the set's Order (kop.SetOrder).
func (s *State) setOrder(v *setVal, order kop.SetOrder, ctx evalCtx) {
	if order == kop.OrderList {
		return
	}
	if v.isPl {
		mask := v.pl
		v.reset(true, false)
		if order == kop.OrderSeat {
			q := s.aliveFrom(ctx.ctrl)
			for i := uint8(0); i < q.n; i++ {
				if mask&(1<<q.s[i]) != 0 {
					s.setPush(v, Obj(q.s[i]))
				}
			}
			return
		}
		for p := uint8(0); p < MaxSeats; p++ {
			if mask&(1<<p) != 0 {
				s.setPush(v, Obj(p))
			}
		}
		return
	}
	slices.Sort(v.objs[:v.n])
	if order != kop.OrderSeat {
		return
	}
	// grouped by controller in seat order from the controller, each seat's
	// objects in object order; a member whose controller is out of the game
	// follows, in object order
	var tmp [setMax]Obj
	var placed [setMax]bool
	n := 0
	q := s.aliveFrom(ctx.ctrl)
	for i := uint8(0); i < q.n; i++ {
		for j := uint16(0); j < v.n; j++ {
			if !placed[j] && s.setCtrlOf(v, v.objs[j]) == q.s[i] {
				placed[j] = true
				tmp[n] = v.objs[j]
				n++
			}
		}
	}
	for j := uint16(0); j < v.n; j++ {
		if !placed[j] {
			tmp[n] = v.objs[j]
			n++
		}
	}
	copy(v.objs[:n], tmp[:n])
}

func (s *State) setCtrlOf(v *setVal, o Obj) uint8 {
	if v.lki {
		return s.lkiCtrlOf(o)
	}
	return s.ctrlOfObj(o)
}

// setOncePerController keeps each controller's first member (SrOncePerController).
func (s *State) setOncePerController(v *setVal) {
	if v.isPl {
		return
	}
	var seen uint8
	n := uint16(0)
	for i := uint16(0); i < v.n; i++ {
		p := s.setCtrlOf(v, v.objs[i])
		if p < MaxSeats {
			if seen&(1<<p) != 0 {
				continue
			}
			seen |= 1 << p
		}
		v.objs[n] = v.objs[i]
		n++
	}
	v.n = n
}

// setAbility is the ability entry e runs, nil when it names none (a stack
// helper program: storm, cascade).
func (s *State) setAbility(k *prep, e *stackEntry) *kop.Ability {
	if int(e.card) >= len(k.t.Cards) {
		return nil
	}
	abs := k.t.Cards[e.card].Abilities
	if int(e.ab) >= len(abs) {
		return nil
	}
	return &abs[e.ab]
}

// setRefOf is the SetRef of instruction pc of ab: a linear scan of at most
// kop.MaxSetRefs entries.
func setRefOf(ab *kop.Ability, pc int) (kop.SetRef, bool) {
	for i := range ab.SetRefs {
		if int(ab.SetRefs[i].PC) == pc {
			return ab.SetRefs[i], true
		}
	}
	return kop.SetRef{}, false
}

// setOf is the set instruction pc of ab acts on, 0 when it has none.
func (s *State) setOf(k *prep, ab *kop.Ability, pc int) kop.SetID {
	r, _ := setRefOf(ab, pc)
	return r.Set
}
