package kgen

import "strings"

// The "if it's blue" gate of a counterspell: ConditionDefined$ Targeted |
// ConditionPresent$ Spell.Blue (Pyroblast, Hydroblast, Burnout, Jace's
// Defeat). gorge's Spell base is an object on the stack (effects/filter.go
// matchesBaseSpell: o.Zone == ZStack). The defined group is the node's one
// target, a spell-only slot (TargetType$ Spell): at resolution a legal target
// of that slot is a spell on the stack, and an illegal one fizzles the whole
// single-target spell before the gate is read (CR 608.2b), so the base adds
// nothing over the card's own characteristics and the clause lowers without
// it. The gate must be the target's own node's (lower_owntarget.go): a
// later node reads the target after the root's effect has moved it (Jace's
// Defeat scries after the counter, where gorge's Spell base no longer holds).
// Any other defined group, a slot that may hold an ability or a
// battlefield object, or a chain of several slots keeps the refusal.

// targetedSpellGate reports whether a ConditionDefined$ d reads exactly one
// spell-only target slot of the current chain.
func (lc *lowerCtx) targetedSpellGate(d string, hasD bool) bool {
	if !hasD || strings.TrimSpace(d) != "Targeted" || lc.ab == nil || len(lc.chainT) != 1 {
		return false
	}
	t := lc.chainT[0]
	if t < 0 || t >= len(lc.ab.Targets) || t != lc.ownTarget() {
		return false
	}
	tg := lc.ab.Targets[t]
	return tg.Spell && !tg.StackAb && !tg.AbOnly && tg.VTgt == nil && !tg.VYou
}

// dropTargetedSpellBase clears the Spell base from a gate's clauses (see
// above); the caller has established targetedSpellGate.
func dropTargetedSpellBase(f *lFilter) {
	if f == nil {
		return
	}
	for i := range f.Clauses {
		f.Clauses[i].SpellBase = false
	}
}
