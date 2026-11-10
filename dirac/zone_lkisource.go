package dirac

// The departure snapshot of an ability's source (CR 608.2h; gorge
// bindSourceLKI). A permanent that leaves the battlefield while one of its
// abilities is on the stack, pending, resolving or being built is snapshot
// into the shared LKI pool (zone_lki.go) as it last existed there, the
// record marked lkiDepart. selfPTLKI reads that record's P/T: re-deriving
// the departed slot instead loses what stopped applying after it left (an
// Equipment's bonus once the state-based action unattached it: Firebending
// Student's firebending trigger after Razor Rings killed it equipped with
// Glider Staff added 2 instead of 3).
//
// The record has no index holder of its own: lkiSweep keeps it while an
// ability entry of that source (obj, srcVer == the record's ver) is live, so
// its reference count stays a function of the holders.

// lkiDepart marks a departure snapshot of an ability's source (lkiRec.bits).
const lkiDepart uint8 = 1 << 4

// srcEntryLive is whether an ability sourced by o at version ver is on the
// stack, pending, resolving or being built (settleSrcKw's holders).
func (s *State) srcEntryLive(o Obj, ver uint8) bool {
	for i := 0; i < min(int(s.sp), MaxStack); i++ {
		if e := &s.stack[i]; e.kind != seSpell && e.obj == o && e.srcVer == ver {
			return true
		}
	}
	for i := uint8(0); i < s.npend; i++ {
		if p := &s.pend[i]; p.obj == o && p.srcVer == ver {
			return true
		}
	}
	if e := &s.res.e; s.res.active && e.kind != seSpell && e.obj == o && e.srcVer == ver {
		return true
	}
	if s.buildLive() {
		if e := &s.bld().e; e.kind != seSpell && e.obj == o && e.srcVer == ver {
			return true
		}
	}
	return false
}

// snapSrcDepart takes the departure snapshot of o, still on the battlefield,
// when one of its abilities is live. Cold: only a departure with something on
// the stack or pending reaches it.
//
//go:noinline
func (s *State) snapSrcDepart(o Obj) {
	if !s.srcEntryLive(o, s.objs[o].ver) {
		return
	}
	if li := s.snapLKI(o); li != 0 {
		s.lk(li).bits |= lkiDepart
	}
}

// srcDepartHold is lkiSweep's hold of the departure snapshots whose source
// still has a live ability entry.
func (s *State) srcDepartHold() {
	for i := range s.g.lki {
		if r := &s.g.lki[i]; r.ref == 0 && r.bits&lkiDepart != 0 && s.srcEntryLive(r.obj, r.ver) {
			r.ref = 1
		}
	}
}

// srcDepartIdx is the pool index (lk's, 1-based) of the departure snapshot of
// ability e's source, 0 when there is none.
func (s *State) srcDepartIdx(e *stackEntry) uint8 {
	for i := range s.g.lki {
		if r := &s.g.lki[i]; r.ref != 0 && r.bits&lkiDepart != 0 && r.obj == e.obj && r.ver == e.srcVer {
			return uint8(i + 1)
		}
	}
	return 0
}

// srcDepartRec is the departure snapshot of ability e's source, or nil.
func (s *State) srcDepartRec(e *stackEntry) *lkiRec {
	if i := s.srcDepartIdx(e); i != 0 {
		return s.lk(i)
	}
	return nil
}
