package dirac

// The layer walk's re-entry guard (selfcond): a static whose condition
// counts permanents (Grond, the Gatebreaker's "you control an Army",
// Rusted Relic's metalcraft) is evaluated inside the walk of the permanent
// it affects, and the count derives every permanent it tests through the
// walk again, the static's own source among them: unguarded, the walk
// recursed until the stack overflowed. gorge evaluates such a gate during
// its static scan with every nested characteristic read served from the
// PREVIOUS scan's static list (engine/rules/layercache.go
// refreshStaticContinuous stamps the epoch before the scan, so a gate's
// nested Derived reuses the old list), so it never recurses either.
//
// The guard marks the source (perm.ovr.walk's walkCond bit) for the
// extent of its condition's evaluation; a nested walk that meets an effect
// of a marked source treats it as not applying instead of evaluating the
// condition again. That answer is gorge's whenever the static's writes
// cannot change what its condition reads, which kgen proves for every
// conditioned static it lowers (dirac/kgen/lower_selfcondition.go selfCondCheck:
// Grond adds types and counts a subtype; Relic adds Creature to an
// artifact and counts artifacts). Cold: only lfxApplies (the walk) calls
// it, and only for an effect with a condition.

// perm.ovr.walk bits.
const (
	walkOn   uint8 = 1 << iota // the slot's characteristics come from the walk (refreshTy)
	walkCond                   // transient: the slot's static condition is being evaluated (lfxCond)
)

// lfxCond is statCond for the walk's effect e (a static of e.src).
func (s *State) lfxCond(k *prep, e *lfx) bool {
	st := statSrc{a: e.a, sl: e.src, ctrl: e.ctrl, obj: e.obj}
	if e.src == NoSlot {
		return s.statCond(k, st) // an emblem's grant: no slot to re-enter
	}
	p := &s.perms[e.src]
	if p.ovr.walk&walkCond != 0 {
		return false // re-entered from its own condition: not applying
	}
	p.ovr.walk |= walkCond
	ok := s.statCond(k, st)
	s.perms[e.src].ovr.walk &^= walkCond
	return ok
}
