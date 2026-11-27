package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The single object target of the spell that raised a SpellCast trigger
// (kop.RefTriggerSpellTarget; gorge Defined$ TriggeredSpellAbilityTargets on a
// line with IsSingleTarget$ True, effects definedSpecTriggeredSpellAbilityTargets).
//
// gorge reads the targets off the cast spell's object at resolution: the
// spell's root and chain targets while it is on the stack, nothing once it
// left (a counterspell cast in response to the trigger clears them, CR 400.7),
// and never the trigger's source. The kernel reads the spell's stack entry the
// same way: the trigger object is the spell, and no entry means no referent.

// triggerSpellTarget is the one distinct object target of the stack entry of
// spell o, NoObj when o is not on the stack, names no target, a player, or more
// than one distinct target (the kgen lowering admits only a line whose
// IsSingleTarget$ True qualifier held as the spell was cast).
//
//go:noinline
func (s *State) triggerSpellTarget(o Obj) Obj {
	if o == NoObj {
		return NoObj
	}
	i := s.stackIndexOf(o)
	if i < 0 {
		return NoObj
	}
	var codes [kop.MaxTargets + maxXTgt]TCode
	n := s.chainTargets(&s.stack[i], &codes)
	if distinctTargets(&codes, n) != 1 || !isObjTarget(codes[0]) {
		return NoObj
	}
	return Obj(codes[0])
}

// triggerSpellTargetList fills out with every distinct object target of the
// stack entry of spell o, in target order. It skips player targets, returns 0
// when the spell is gone or names no object target. The op runs once per
// object (kop.RefTriggerSpellTargets; gorge's set read).
//
//go:noinline
func (s *State) triggerSpellTargetList(o Obj, out *[kop.MaxTargets + maxXTgt]Obj) int {
	if o == NoObj {
		return 0
	}
	i := s.stackIndexOf(o)
	if i < 0 {
		return 0
	}
	var codes [kop.MaxTargets + maxXTgt]TCode
	n := s.chainTargets(&s.stack[i], &codes)
	k := 0
	for j := 0; j < n; j++ {
		c := codes[j]
		if !isObjTarget(c) {
			continue
		}
		dup := false
		for x := 0; x < k; x++ {
			if out[x] == Obj(c) {
				dup = true
				break
			}
		}
		if !dup {
			out[k] = Obj(c)
			k++
		}
	}
	return k
}
