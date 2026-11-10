package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A triggered ability's own "if you do" sacrifice cost and RefSacrificed
// (gorge Ctx.Sacrificed, state.SacrificedInfoOf; CR 701.21, 608.2h: Ruthless
// Technomancer's "Treasures equal to that creature's power").
//
// RefSacrificed reads the running entry's LKI record (e.li) as the sacrificed
// permanent's last known values. A queued trigger already holds a record there,
// the triggering object's (enqueueLKI), so for an ETB-self trigger it names the
// card itself and the cost's pick never replaced it. The pick's snapshot takes
// the slot when the body reads RefSacrificed and never RefTriggerObj /
// RefTriggerSource (a body reading both keeps the triggering object's record:
// the one slot cannot hold two).

// progReads reports whether any instruction of prog names r, as its object or
// inside an amount expression.
func progReads(k *prep, prog []kop.Instr, r kop.Ref) bool {
	for i := range prog {
		in := &prog[i]
		if in.Obj == r || valueReads(k, in.A, r, 0) || valueReads(k, in.B, r, 0) {
			return true
		}
	}
	return false
}

// valueReads reports whether amount v is, or nests, an expression with Ref r.
func valueReads(k *prep, v kop.ValueRef, r kop.Ref, depth int) bool {
	if v.K != kop.VExpr || v.N <= 0 || int(v.N) >= len(k.t.Exprs) || depth > 8 {
		return false
	}
	x := &k.t.Exprs[v.N]
	return x.Ref == r || valueReads(k, x.Sub[0], r, depth+1) || valueReads(k, x.Sub[1], r, depth+1)
}

// snapTrigSacrificed records sl, the first permanent a trigger's own IfPaid
// sacrifice cost takes, as the resolving entry's last known information, before
// it leaves, when the trigger's body reads it as RefSacrificed.
//
//go:noinline
func (s *State) snapTrigSacrificed(k *prep, sl Slot) {
	e := &s.res.e
	if e.kind != seTrigger || sl == NoSlot || e.ab >= abCascade {
		return
	}
	prog := s.prog(k)
	if s.res.pc >= uint8(len(prog)) || prog[s.res.pc].Op != kop.OpIfPaid {
		return
	}
	if !progReads(k, prog, kop.RefSacrificed) || progReads(k, prog, kop.RefTriggerObj) || progReads(k, prog, kop.RefTriggerSource) {
		return
	}
	e.li = s.snapLKI(s.perms[sl].obj)
}
