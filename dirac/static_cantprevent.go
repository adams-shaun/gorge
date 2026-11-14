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
// Both preventions that carry a rider are copied as gorge plays them
// (CR 615.12): a Fog (combat.go fogActive) stops nothing the static covers,
// so damageStep runs the ordinary pass (gorge's fogUnpreventable keeps every
// assignment the blanket static covers), and The Mindskinner's
// prevent-and-mill body (repl_preventmill.go) still mills while the damage
// stands. No kernel path stalls on the static any more; StallNoPrevent stays
// in the append-only stall table, unused.

// StallNoPrevent: formerly a Fog or a prevent-and-mill replacement under
// "damage can't be prevented"; no longer raised.
const StallNoPrevent = 151

func init() { stallNames[StallNoPrevent] = "noprevent" }

// DebugNoPreventOff is TestCantPreventDamage's mutation switch: the static
// is ignored. It must make the shield prevent again.
var DebugNoPreventOff bool

// noPrev reports whether damage can't be prevented now; combat says the
// damage being prevented is combat damage (a StNoPrevCombat static, IsCombat$
// True, reaches only that: gorge cantPreventDamage's combatDamaging).
func (s *State) noPrev(k *prep, combat bool) bool {
	return k.gen != nil && k.gen.anyNoPrev && s.noPrevLive(k, combat)
}

// noPrevLive: a functioning StatCantPreventDamage static (a battlefield
// permanent that has not lost its abilities, its Cond2 holding, or an
// emblem) is in force, a combat-only one (StNoPrevCombat) only for combat
// damage.
//
//go:noinline
func (s *State) noPrevLive(k *prep, combat bool) bool {
	if DebugNoPreventOff {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatCantPreventDamage &&
			(combat || st.a.Static.StFlags&kop.StNoPrevCombat == 0) && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	if !hit && s.res.active && s.res.e.kind == seSpell {
		hit = s.noPrevSpellSelf(k)
	}
	return hit
}

// noPrevSpellSelf: the spell resolving now carries a StatCantPreventDamage
// static that functions on the stack (StNoPrevSpellSelf: EffectZone$ Stack |
// ValidSource$ Spell.Self, Banefire's "if X is 5 or more, the damage can't be
// prevented") whose Cond2 holds against the entry. gorge's source match is the
// spell itself; the kernel reads every damage dealt while the spell resolves
// as its own (a resolution deals no other source's damage).
func (s *State) noPrevSpellSelf(k *prep) bool {
	e := &s.res.e
	cd := &k.t.Cards[e.card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantPreventDamage || a.Static.StFlags&kop.StNoPrevSpellSelf == 0 {
			continue
		}
		if c := a.Static.Cond2; c == 0 || s.condEval(k, c, evalCtx{e: e, src: e.obj, ctrl: e.ctrl}) {
			return true
		}
	}
	return false
}
