package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A multi-pick charm with a 1..N target slot. gorge's
// askCharmModeTargets (target_modes.go) poses the ONE combined KTarget only
// when every chosen mode's slot is 1..1 (allSimple); otherwise it asks the
// chosen modes' root slots SEQUENTIALLY (askCharmSeqSlot), one KTarget per
// target-bearing mode in chosen-mode order, each with the slot's own Min..Max
// clamped to its candidate count -- the 1..1 slots included. The answers bind
// positionally, as the combined answer does.
//
// The kernel derives the shape from the slot bounds (no table flag): the
// sequential asks are the ordinary per-slot asks of the build's bsTargets
// stage (a chain link is never present with a 1..N slot: kgen refuses it),
// the build's ti the cursor and genState.mt the resume point of a plural slot.
// The only difference from an ordinary spell's multi-target slot is the
// clamp: gorge shrinks Max to the candidate count, so a one-candidate 1..2
// slot closes after its pick rather than offering a done-only step.

// DebugCharmSeqCombined poses the combined ask even when a slot is 1..N (each
// slot then takes one pick); the mutation test asserts the lockstep catches it.
var DebugCharmSeqCombined bool

// charmSeqSlots reports whether the chosen modes' root slots are asked
// sequentially: two or more modes bear a target (a lone one takes the
// ordinary ask, gorge's len(slots) < 2) and some root slot takes more than
// one target or is "up to" (Min 0: Kozilek's Command's X cards from graveyards).
func charmSeqSlots(a *kop.Ability, mode uint8) bool {
	if DebugCharmSeqCombined || a.Repeats() {
		return false
	}
	mask, n := charmRootSlots(a, mode)
	if n < 2 {
		return false
	}
	for j := 0; j < int(a.NTargets); j++ {
		if mask&(1<<j) != 0 && (a.Targets[j].Max > 1 || a.Targets[j].UpTo && !DebugCharmUpToX) {
			return true // an "up to X" slot (0..X, TgfBoundsV) is outside 1..1 too
		}
	}
	return false
}

// charmSeqSlot reports whether root slot ti of the build is asked
// sequentially.
func (s *State) charmSeqSlot(k *prep, ti int) bool {
	if k.gen == nil || !k.gen.anyCharmNum {
		return false
	}
	a := s.buildAbility(k)
	if a == nil || !charmSeqSlots(a, s.bld().e.mode) {
		return false
	}
	roots, _ := charmRootSlots(a, s.bld().e.mode)
	return roots&(1<<ti) != 0
}

// charmSeqPrime readies root slot ti of a sequential charm ask (called by
// bsTargets under anyCharmNum before the slot is posed): it runs the
// pre-target gate ahead of the first ask and starts a plural slot with its Max
// clamped to the candidate count (askCharmSeqSlot). It returns false when the
// cast was reversed.
//
//go:noinline
func (s *State) charmSeqPrime(k *prep, a *kop.Ability, ti int) bool {
	e := &s.bld().e
	if !charmSeqSlots(a, e.mode) {
		return true
	}
	roots, _ := charmRootSlots(a, e.mode)
	if roots&(1<<ti) == 0 {
		return true
	}
	mt := &s.g.mt
	if mt.active && int(mt.slot) == ti {
		return true
	}
	if roots&(1<<ti-1) == 0 && s.charmTargetAskDead(k) {
		s.reverseCast(k) // gorge's pre-target gate runs ahead of the first ask
		return false
	}
	t := &a.Targets[ti]
	if t.UpTo && t.Divide == 0 && t.TFlags&^(kop.TgfDecl|kop.TgfBoundsV) == 0 && t.TFlags&kop.TgfBoundsV != 0 {
		s.charmUpToPrime(k, t, ti) // an "up to X" slot
		return true
	}
	if t.Max < 2 || t.UpTo || t.Divide != 0 || t.TFlags&^kop.TgfDecl != 0 {
		return true
	}
	var buf [MaxOptions]TCode
	n := s.targetCands(k, t, e.ctrl, s.srcSlot(), e.obj, &buf)
	if k.gen != nil {
		n = s.affordableTargets(k, &buf, n)
	}
	if n < int(t.Min) {
		panic("kernel: multi-pick charm mode without a legal target")
	}
	*mt = multiTgt{active: true, slot: uint8(ti), min: t.Min, max: min8(t.Max, uint8(n))}
	return true
}

func min8(a, b uint8) uint8 {
	if b < a {
		return b
	}
	return a
}

// charmUpToPrime readies a sequential charm ask's "up to X" root slot (a
// TgfBoundsV slot of Min 0, Kozilek's Command's X cards from graveyards):
// gorge's askCharmSeqSlot resolves the slot's bounds as it is announced,
// asks nothing for a resolved maximum of 0 or a slot with no candidate (an
// empty group), and clamps the maximum to the candidate count, so a slot
// taking every candidate closes with no done-only ask (an ordinary spell's
// "up to X" slot keeps that ask: tgt_variablemax.go). The bound is
// evaluated here, as tgtBoundsV would, and clamped.
//
//go:noinline
func (s *State) charmUpToPrime(k *prep, t *kop.TargetSpec, ti int) {
	e := &s.bld().e
	lo, hi := s.tgtBoundsEval(k, t, e)
	var buf [MaxOptions]TCode
	n := s.targetCands(k, t, e.ctrl, s.srcSlot(), e.obj, &buf)
	if k.gen != nil {
		n = s.affordableTargets(k, &buf, n)
	}
	if hi == 0 || n == 0 {
		return // no ask: the ordinary slot step records the empty group
	}
	if DebugCharmShapeStats != nil {
		DebugCharmShapeStats.UpToXSlots++
	}
	if hi > n {
		hi = n
	}
	s.g.mt = multiTgt{active: true, slot: uint8(ti), min: uint8(min(lo, hi)), max: uint8(min(hi, 254))}
}
