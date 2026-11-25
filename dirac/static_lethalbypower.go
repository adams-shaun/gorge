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

// useLethalPow: a card of the table has a StatLethalByPower static (a genPrep
// use bit, not a field: TestNoNewFixedStateFields).
var useLethalPow = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatLethalByPower) })
