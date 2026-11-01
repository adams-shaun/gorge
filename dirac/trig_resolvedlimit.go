package dirac

// --- ResolvedLimit$ 1 (kop.Tf2ResolvedLimit) ---
//
// gorge triggerTurnResolved: a per-turn resolution count keyed by the
// trigger's SOURCE object, shared by its ResolvedLimit$ lines; a mandatory
// limited line counts as it resolves (past the intervening-if recheck), an
// optional one when its controller accepts it; a limited line whose source
// reached the limit does not trigger. The kernel keeps the count as
// once-per-turn records (s.g.used, cleared at each turn's start; the pool's
// cap stalls loud, StallUsed) with the ability byte rlMark, keyed by the
// source object and its ver (a source that changed zones is a new object).

// rlMark is the used-record ability byte of a ResolvedLimit$ resolution
// (once-per-turn records keep ability indices below it; resolveMark|ab
// lies above).
const rlMark = 0x7F

// countRL records one resolution of a limited line of e's source.
//
//go:noinline
func (s *State) countRL(e *stackEntry) {
	if int(s.g.nused) < maxUsed {
		s.g.used[s.g.nused] = usedRec{obj: e.obj, ab: rlMark, ver: e.srcVer}
		s.g.nused++
	} else {
		s.capFull(StallUsed)
	}
}

// rlSpent: source object o (as it is now) resolved a limited line this
// turn (the limit is 1).
//
//go:noinline
func (s *State) rlSpent(o Obj) bool {
	v := s.objs[o].ver
	for i := 0; i < int(s.g.nused); i++ {
		if r := s.g.used[i]; r.obj == o && r.ab == rlMark && r.ver == v {
			return true
		}
	}
	return false
}
