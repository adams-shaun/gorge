package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// union: an op whose subject is several target slots' union (RefTargets01,
// RefTargets012, RefTargets12, RefTargets0123): gorge's Defined$ Targeted over a SubAbility$
// chain (rules trigger_subtargets.go chainTargetUnion, effects count_ref.go
// refTargetUnion) -- the root's targets, then each link's, in chain order,
// a target illegal on resolution left out (CR 608.2b). Cold code, reached
// through one prep bit (gen.anyUnion) in execGen.

// unionMask is the slot mask a union ref names (0: not a union ref).
func unionMask(r kop.Ref) uint8 {
	switch r {
	case kop.RefTargets0:
		return 1
	case kop.RefTargets01:
		return 3
	case kop.RefTargets012:
		return 7
	case kop.RefTargets12:
		return 6
	case kop.RefTargets0123:
		return 15
	case kop.RefSet:
		return setUnionMark // not a slot union: execGen hands it to execSet (set_eval.go)
	}
	return 0
}

// setUnionMark is unionMask's answer for a RefSet subject: no target slot's
// bit, so tableUnion's one prep bit gates the set arm too.
const setUnionMark = 0x80

// tableUnion reports whether some program's subject is a union ref.
func tableUnion(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if unionMask(in.Obj) != 0 {
					return true
				}
			}
		}
	}
	return false
}

// execUnion runs in once per legal object of its union's slots: each
// slot's first object, then its other objects (the multi-object slot's
// order, execEachTarget), slot by slot -- the order of gorge's union. The
// objects are listed before the first run, as gorge resolves Defined once:
// an extra target is judged as the resolution began (CR 608.2b, xtLegalRes), so
// an earlier op of the resolution that changed what a target is (an Animate
// making a noncreature artifact a creature, Depthshaker Titan) does not drop
// it from the next op's set.
// A destroy drops the indestructible objects first: gorge effDestroy builds
// its victim list before the batch's first move, so one destroyed this way
// (an Equipment granting indestructible) does not expose another.
//
//go:noinline
func (s *State) execUnion(k *prep, in *kop.Instr) {
	e := &s.res.e
	m := unionMask(in.Obj)
	var slot, ver [kop.MaxTargets + maxXTgt]uint8
	var code [kop.MaxTargets + maxXTgt]TCode
	n := 0
	for i := uint8(0); i < kop.MaxTargets; i++ {
		if m&(1<<i) == 0 {
			continue
		}
		if c := s.tgtCode(int(i)); c != NoTarget {
			slot[n], code[n], ver[n] = i, c, e.tver[i]
			n++
		}
		xt := s.xtOf(e)
		for j := 0; j < int(xt.n); j++ {
			if xt.slot[j] == i && s.xtLegalRes(k, e, j, i) {
				slot[n], code[n], ver[n] = i, xt.code[j], xt.ver[j]
				n++
			}
		}
	}
	if n == 0 {
		return
	}
	if in.Op == kop.OpDestroy {
		for j := 0; j < n; j++ {
			if c := code[j]; isObjTarget(c) {
				if ob := &s.objs[c]; ob.zone == kop.ZoneBattlefield && s.kw(k, ob.slot)&kop.KwIndestructible != 0 {
					code[j] = TCode(NoTarget)
				}
			}
		}
	}
	saveT, saveV, saveBad, saveLoop := e.tgt, e.tver, s.res.bad, s.g.inLoop
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	hr := s.holdOpen(k)
	defer s.holdClose(k, hr)
	s.g.inLoop = true
	one := *in
	ndly0 := s.ndly
	for j := 0; j < n && !s.over; j++ {
		if code[j] == NoTarget {
			continue
		}
		i := slot[j]
		e.tgt[i], e.tver[i] = code[j], ver[j]
		s.res.bad &^= 1 << i
		one.Obj = tgtRef(int(i))
		if in.Op == kop.OpAtEOT && s.ndly > ndly0 && s.atEOTUnionJoin(k, &one) {
			continue // a later object of the set joins the first one's grouped promise
		}
		s.exec(k, &one)
	}
	e.tgt, e.tver, s.res.bad, s.g.inLoop = saveT, saveV, saveBad, saveLoop
}
