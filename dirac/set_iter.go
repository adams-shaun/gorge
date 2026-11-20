package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Iteration over an evaluated SetExpr (set_eval.go) and its suspension.
//
// forSet evaluates the instruction's set on the Go stack and runs a member
// body once per member. The body is passed as a method expression, a static
// function value, so there is no closure and nothing allocates.
//
// A member body may suspend for a decision: it returns false (the CR 616.1
// counter-order ask: the instruction runs again, runProg) or leaves a parked
// move waiting (the answer ends in continueRes). Either way the iteration is
// frozen in coldSetIter: the members as bits, the member that suspended, and
// which way it resumes. A table has the region only when a RefSet op the
// kernel marks suspension-capable (setSuspOp) is in it. The members are bits,
// so only OrderObj and OrderSeat sets can suspend; an OrderList set that does
// is StallSetIter, as is a second live iterator (the depth is one).

// StallSetIter is a RefSet iteration that cannot be held: a second live
// iterator, a member that suspended in a table with no setIter region, or an
// OrderList set that suspended.
const StallSetIter = 75

// StallSetShape is a RefSet op shape the kernel does not serve (set_ops.go): an
// optional, defined or filtered discard, a bounce or library move with a
// rider.
const StallSetShape = 76

func init() {
	stallNames[StallSetIter] = "setiter"
	stallNames[StallSetShape] = "setshape"
}

// setIter is the one live iteration, about 72 B.
type setIter struct {
	members objBits // the frozen members: object indexes, or (pl) seats
	cur     uint16  // the member that suspended, an index into the ordered members
	pc      uint8   // the instruction iterating
	pl      uint8   // 1: members is a players set
	live    bool
	// skip: the suspended member is complete, resume after it (a parked move
	// answered through continueRes). Clear: it runs again (the order ask).
	skip bool
}

// coldSetIter is the one iterator, for a table with a suspension-capable
// RefSet op.
var coldSetIter = newCold[setIter]("setIter", func(g *genPrep) int {
	if g != nil && g.uses.has(useSetSusp) {
		return 1
	}
	return 0
})

// useSetRef is genPrep.uses' bit for a table where some instruction reads a
// RefSet: its generated prep is needed, so execGen's RefSet arm is reached.
var useSetRef = newGenUse(func(t *kop.Table) bool { return tableHasSet(t, false) })

// useSetSusp is the bit for a table with a RefSet op that can suspend.
var useSetSusp = newGenUse(func(t *kop.Table) bool { return tableHasSet(t, true) })

// setSuspOp reports whether op, run per member of a RefSet, can suspend for a
// decision: its single-object path reaches the CR 616.1 counter-order ask
// (OpPutCounter) or parks a move on a replacement-order ask (the leave of a
// destroy, an exile, a bounce, a library move; repl_exilemark.go).
func setSuspOp(op kop.Op) bool {
	switch op {
	case kop.OpPutCounter, kop.OpDestroy, kop.OpExile, kop.OpBounce, kop.OpToLibrary:
		return true
	}
	return false
}

func tableHasSet(t *kop.Table, susp bool) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			ab := &t.Cards[i].Abilities[j]
			for _, sr := range ab.SetRefs {
				if !susp || int(sr.PC) < len(ab.Prog) && setSuspOp(ab.Prog[sr.PC].Op) {
					return true
				}
			}
		}
	}
	return false
}

// setIterOf is the table's iterator, nil when it has no suspension-capable op.
func (s *State) setIterOf() *setIter {
	if r := coldSetIter.of(s); r != nil {
		return &r[0]
	}
	return nil
}

// setParked reports whether the member body that just ran left a move parked
// on a decision: runProg's checks after an op, the ones that end in continueRes.
func (s *State) setParked() bool {
	return len(s.cold.parked) != 0 && s.parkedWaiting() || s.res.ask == raGenCmdZone || s.res.ask == raGenEnterDefender
}

// setIterResume reports whether the answer continueRes is finishing belongs to
// a suspended RefSet member that is complete: the iteration then goes on from
// the member after it, the instruction run again without advancing pc.
func (s *State) setIterResume() bool {
	it := s.setIterOf()
	return it != nil && it.live && it.skip && it.pc == s.res.pc
}

// forSet runs f once per member of the set instruction pc names, in the set's
// Order. It returns false when a member suspended on a decision (res.ask is
// set), true otherwise: a parked move leaves the iteration frozen for
// continueRes and returns true, as the single-object op does.
func (s *State) forSet(k *prep, e *stackEntry, in *kop.Instr, pc int, f func(*State, *prep, *kop.Instr, Obj) bool) bool {
	ab := s.setAbility(k, e)
	if ab == nil {
		panic("kernel: RefSet in a program with no ability")
	}
	ref, ok := setRefOf(ab, pc)
	if !ok {
		panic("kernel: RefSet instruction has no SetRef")
	}
	ctx := s.ctxEntry(e)
	var v setVal
	it := s.setIterOf()
	i := 0
	if it != nil && it.live {
		if int(it.pc) != pc {
			s.stall(StallSetIter) // a second live iterator
			return true
		}
		s.setThaw(k, ref, ctx, it, &v)
		i = int(it.cur)
		if it.skip {
			i++
		}
		it.live, it.skip = false, false
	} else {
		s.evalSetAs(k, ref.Set, ctx, ref.Flags, &v)
	}
	for ; i < int(v.n) && !s.over; i++ {
		if !f(s, k, in, v.objs[i]) {
			s.setSuspend(k, ref, it, &v, pc, i, false)
			return false
		}
		if s.setParked() {
			s.setSuspend(k, ref, it, &v, pc, i, true)
			return true
		}
	}
	return true
}

// setSuspend freezes the iteration of v at member i.
func (s *State) setSuspend(k *prep, ref kop.SetRef, it *setIter, v *setVal, pc, i int, skip bool) {
	if it == nil || k.t.Sets[ref.Set].Order == kop.OrderList {
		s.stall(StallSetIter)
		return
	}
	it.members = objBits{}
	for j := uint16(0); j < v.n; j++ {
		it.members.set(v.objs[j])
	}
	it.pl = 0
	if v.isPl {
		it.pl = 1
	}
	it.cur, it.pc, it.live, it.skip = uint16(i), uint8(pc), true, skip
}

// setThaw rebuilds the ordered members of a suspended iteration from its bits.
func (s *State) setThaw(k *prep, ref kop.SetRef, ctx evalCtx, it *setIter, v *setVal) {
	v.reset(it.pl != 0, ref.Flags&kop.SrLKI != 0)
	for o := Obj(0); o < MaxObj; o++ {
		if it.members.has(o) {
			s.setPush(v, o)
		}
	}
	s.setOrder(v, k.t.Sets[ref.Set].Order, ctx)
}

// forSetPlayers is forSet over a players set. A member body that returns
// false stops the loop; the ops that use it (life, set_ops.go) never suspend.
func (s *State) forSetPlayers(k *prep, e *stackEntry, in *kop.Instr, pc int, f func(*State, *prep, *kop.Instr, uint8) bool) bool {
	var v setVal
	if !s.setLoad(k, e, pc, &v, true) {
		return true
	}
	for i := uint16(0); i < v.n && !s.over; i++ {
		if !f(s, k, in, uint8(v.objs[i])) {
			return false
		}
	}
	return true
}

// setLoad evaluates the set instruction pc names into v; wantPl is the kind the
// op reads. false: the set is of the other kind, which kgen never emits (a
// stall, not a panic: a hand table can).
func (s *State) setLoad(k *prep, e *stackEntry, pc int, v *setVal, wantPl bool) bool {
	ab := s.setAbility(k, e)
	if ab == nil {
		panic("kernel: RefSet in a program with no ability")
	}
	ref, ok := setRefOf(ab, pc)
	if !ok {
		panic("kernel: RefSet instruction has no SetRef")
	}
	s.evalSetAs(k, ref.Set, s.ctxEntry(e), ref.Flags, v)
	if v.isPl != wantPl {
		s.stall(StallSetShape)
		return false
	}
	return true
}
