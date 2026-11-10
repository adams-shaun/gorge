package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Damage can't be prevented (CR 615.12): kop.StatCantPreventDamage, printed
// or on an emblem (an Effect's "damage can't be prevented this turn").
// gorge's cantPreventDamage (engine/rules/protection.go) turns off every
// prevention its replacement pipeline classifies as one: protection's
// prevention before the replacements (emit), the Prevent$ True and DB$
// ReplaceDamage bodies, granted prevention keywords and the shield counter
// (replacement_damage.go). Each kernel prevention predicate asks noPrev only
// after it found a prevention that would apply, so a pool without the static
// pays nothing (genPrep.anyNoPrev) and the static costs one scan per hit.
//
// Two preventions gorge applies regardless are not copied: its Fog
// (combat.go fogActive) and The Mindskinner's prevent-and-mill body (no
// Prevent$ True, so damageReplacementPrevents misses it) still prevent the
// damage there, which CR 615.12 forbids. Meeting either while the static
// functions stalls (StallNoPrevent) rather than play either reading.

// StallNoPrevent: damage can't be prevented while a Fog or a prevent-and-mill
// replacement would apply (gorge prevents it anyway; CR 615.12 does not).
const StallNoPrevent = 151

func init() { stallNames[StallNoPrevent] = "noprevent" }

// DebugNoPreventOff is TestCantPreventDamage's mutation switch: the static
// is ignored. It must make the shield prevent again.
var DebugNoPreventOff bool

// noPrev reports whether damage can't be prevented now.
func (s *State) noPrev(k *prep) bool {
	return k.gen != nil && k.gen.anyNoPrev && s.noPrevLive(k)
}

// noPrevLive: a functioning StatCantPreventDamage static (a battlefield
// permanent that has not lost its abilities, its Cond2 holding, or an
// emblem) is in force.
//
//go:noinline
func (s *State) noPrevLive(k *prep) bool {
	if DebugNoPreventOff {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatCantPreventDamage && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// fogNoPrev stalls a Fog's combat damage step while damage can't be
// prevented (gorge's fogActive ignores the static).
func (s *State) fogNoPrev(k *prep) {
	if s.noPrev(k) {
		s.capFull(StallNoPrevent)
	}
}
