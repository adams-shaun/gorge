package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugMustTargetOff is a mutation switch: a StatMustTarget static obliges
// nothing when set. TestMustTargetMutation sets it and requires a divergence.
var DebugMustTargetOff bool

// MustTargetStats counts the target asks a live MustTarget static narrowed:
// Single for a one-object ask, Multi for a multi-object slot. DebugMustTargetStats
// arms it (the tests).
type MustTargetStats struct {
	Single int
	Multi  int
	// Partnered counts the nonmatching candidates a multi-object slot's set
	// constraint dropped because no matching pick could still join them.
	Partnered int
}

// DebugMustTargetStats, when non-nil, counts the narrowing (a test's proof the
// mechanic fired; nil in a normal game).
var DebugMustTargetStats *MustTargetStats

// eachMustTarget calls f for every live printed S:Mode$ MustTarget static that
// binds p for the target ask e poses. p must be the spell's or activated
// ability's controller (a TargetingPlayer$ ask is exempt), p is an opponent of
// the static's controller (no teams), the static's condition holds, and the
// action kind is the spell or activated ability kgen lowers (the two
// ValidSA$ alternatives are checked there). eachStatic skips a source that
// lost its abilities, as gorge's activeStatics does.
func (s *State) eachMustTarget(k *prep, e *stackEntry, p uint8, f func(st statSrc) bool) {
	if DebugMustTargetOff || k.gen == nil || p != e.ctrl {
		return
	}
	if e.kind != seSpell && e.kind != seAbility {
		return
	}
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatMustTarget {
			return true
		}
		if st.ctrl == p || !s.statCond(k, st) {
			return true
		}
		return f(st)
	})
}

// mustTargetMask marks the candidates of cands a live MustTarget static obliges
// p to choose among and reports whether any candidate matches. A false report
// leaves the ask unrestricted: "if able" is the offer itself.
func (s *State) mustTargetMask(k *prep, e *stackEntry, p uint8, cands []TCode, mask *[MaxOptions]bool) bool {
	any := false
	s.eachMustTarget(k, e, p, func(st statSrc) bool {
		f := st.a.Static.Filter
		for i, c := range cands {
			if mask[i] || !isObjTarget(c) {
				continue
			}
			o := Obj(c)
			if s.objs[o].zone != kop.ZoneBattlefield {
				continue
			}
			if s.matchPermGen(k, f, s.objs[o].slot, st.ctrl, st.sl, layerFull) {
				mask[i] = true
				any = true
			}
		}
		return true
	})
	return any
}

// mustTargetCode reports whether one chosen object matches a live MustTarget
// static binding p.
func (s *State) mustTargetCode(k *prep, e *stackEntry, p uint8, c TCode) bool {
	if !isObjTarget(c) {
		return false
	}
	o := Obj(c)
	if s.objs[o].zone != kop.ZoneBattlefield {
		return false
	}
	hit := false
	s.eachMustTarget(k, e, p, func(st statSrc) bool {
		if s.matchPermGen(k, st.a.Static.Filter, s.objs[o].slot, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// mustTargetPicked reports whether a pick already made in slot ti matches a
// live MustTarget static binding p.
func (s *State) mustTargetPicked(k *prep, e *stackEntry, p uint8, ti uint8) bool {
	if c := e.tgt[ti]; c != NoTarget && s.mustTargetCode(k, e, p, c) {
		return true
	}
	xt := s.xtOf(e)
	for i := 0; i < int(xt.n); i++ {
		if xt.slot[i] == ti && s.mustTargetCode(k, e, p, xt.code[i]) {
			return true
		}
	}
	return false
}

// mustTargetSingle narrows a one-object target ask's candidates to the matches
// a live MustTarget static requires, as gorge's applyMustTarget does for a
// single-target ask. It returns the new candidate count and whether the ask
// was narrowed (so an "up to one" ask no longer offers its empty answer).
func (s *State) mustTargetSingle(k *prep, e *stackEntry, cands *[MaxOptions]TCode, n int) (int, bool) {
	if n == 0 {
		return n, false
	}
	var mask [MaxOptions]bool
	if !s.mustTargetMask(k, e, e.ctrl, cands[:n], &mask) {
		return n, false
	}
	if DebugMustTargetStats != nil {
		DebugMustTargetStats.Single++
	}
	m := 0
	for i := 0; i < n; i++ {
		if mask[i] {
			cands[m] = cands[i]
			m++
		}
	}
	return m, true
}

// mustTargetMulti narrows a multi-object target slot's next pick. It removes a
// nonmatching candidate only when this pick is the slot's last (a valid
// selection must include a match), and reports whether Done is still a legal
// answer. A mana-value-capped slot is left unrestricted, as gorge leaves a
// multi-target ask whose Value already carries a budget. A divided slot is not
// budgeted: its total only caps the target count, and the division is a later
// ask, so gorge's floor binds it (Storm the Seedcore beside a Standard Bearer).
func (s *State) mustTargetMulti(k *prep, e *stackEntry, t *kop.TargetSpec, ti uint8, mt *multiTgt, cands *[MaxOptions]TCode, n int) (int, bool) {
	if n == 0 || k.gen == nil || e.kind == seTrigger || t.MaxMV != 0 {
		return n, true
	}
	if s.mustTargetPicked(k, e, e.ctrl, ti) {
		return n, true
	}
	var mask [MaxOptions]bool
	if !s.mustTargetMask(k, e, e.ctrl, cands[:n], &mask) {
		return n, true
	}
	if DebugMustTargetStats != nil {
		DebugMustTargetStats.Multi++
	}
	if mt.picked+1 >= mt.max {
		// the last pick must itself supply the match
		m := 0
		for i := 0; i < n; i++ {
			if mask[i] {
				cands[m] = cands[i]
				m++
			}
		}
		n = m
	} else {
		// a nonmatching pick is admissible only while a matching pick can
		// still join it under the slot's set constraints (same/different
		// controller, shared creature type, distinct names), as gorge's
		// completable filter admits only picks that can still complete.
		m := 0
		for i := 0; i < n; i++ {
			c := cands[i]
			if mask[i] || s.mustTargetPartner(k, t, c, cands[:n], &mask) {
				cands[m] = c
				m++
			} else if DebugMustTargetStats != nil {
				DebugMustTargetStats.Partnered++
			}
		}
		n = m
	}
	return n, false
}

// mustTargetPartner reports whether a matching candidate compatible with c is
// still among cands: after choosing c the slot can still include a match.
func (s *State) mustTargetPartner(k *prep, t *kop.TargetSpec, c TCode, cands []TCode, mask *[MaxOptions]bool) bool {
	for j, m := range cands {
		if mask[j] && s.mustTargetCompatible(k, t, c, m) {
			return true
		}
	}
	return false
}

// mustTargetCompatible reports whether candidates c and m may be picked into
// the same slot under the set constraints tgtSetAdmit enforces per pick: the
// same or different controller, no shared name, and a shared creature type.
func (s *State) mustTargetCompatible(k *prep, t *kop.TargetSpec, c, m TCode) bool {
	if t.TFlags&kop.TgfSameController != 0 && s.ctrlOfObj(Obj(c)) != s.ctrlOfObj(Obj(m)) {
		return false
	}
	if t.TFlags&kop.TgfDiffController != 0 && s.ctrlOfObj(Obj(c)) == s.ctrlOfObj(Obj(m)) {
		return false
	}
	if t.TFlags&kop.TgfDistinctNames != 0 && s.objs[c].card == s.objs[m].card {
		return false
	}
	if t.TFlags&kop.TgfSharedCrType != 0 {
		cm, ca := s.crTypesOf(k, Obj(c))
		mm, ma := s.crTypesOf(k, Obj(m))
		if !s.sharesCr(cm, ca, mm, ma) {
			return false
		}
	}
	return true
}

// mustTargetDone reports whether a multi-target slot may end: its minimum is
// met and no live MustTarget static still requires a match.
func mustTargetDone(picked, min uint8, mustDone bool) bool { return picked >= min && mustDone }
