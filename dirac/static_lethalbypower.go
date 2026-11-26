package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// LethalDamageByPower$ (kop.StatLethalByPower, Zilortha, Strength
// Incarnate): lethal damage to a creature the static's Filter matches is
// judged by its power, not its toughness (gorge
// Engine.LethalDamageThreshold, Forge Card.getLethal). Every kernel read of
// "damage that is lethal" goes through lethalThreshold: the SBA scan's
// CR 704.5g check (static_sbascan.go), the attacker's and the dividing
// blocker's lethal assignment (combat.go, combat_extrablock.go; gorge
// lethalDamageNeed) and the CR 120.10 excess (act_empower.go
// excessLethalGen; gorge effects.LethalThreshold).

// DebugLethalByPowerOff is the lockstep mutation switch (test only): the
// static is ignored and toughness is read, as before the lowering.
var DebugLethalByPowerOff bool

// lethalThreshold is the damage lethal to creature sl before marked damage:
// its power pw under a StatLethalByPower static, else its toughness t (the
// caller's P/T read).
func (s *State) lethalThreshold(k *prep, sl Slot, pw, t int) int {
	if k.gen == nil || !k.gen.uses.has(useLethalPow) || DebugLethalByPowerOff {
		return t
	}
	if s.staticFlagGen(k, kop.StatLethalByPower, sl) {
		return pw
	}
	return t
}

// lethalThresholdFor is sbaScan's threshold read: the pass's lethal damage for
// creature sl, and whether reading it dirties the pass (the second result).
// LethalDamageByPower$ reads power, which the clean-scan key was not built
// around, so a table with the static is never a cached clean scan; a creature
// with no marked damage can never cross whatever the threshold is, so it keeps
// the plain toughness and leaves the pass clean — which is what lets the
// caller's lethal test read the threshold without a damage>0 guard of its own
// (no marked damage leaves the threshold at the positive toughness).
func (s *State) lethalThresholdFor(k *prep, sl Slot, pw, tough int, damage uint8) (int, bool) {
	if k.gen == nil || !k.gen.uses.has(useLethalPow) || damage == 0 {
		return tough, false
	}
	return s.lethalThreshold(k, sl, pw, tough), true
}

// useLethalPow: a card of the table has a StatLethalByPower static (a genPrep
// use bit, not a field: TestNoNewFixedStateFields).
var useLethalPow = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatLethalByPower) })
