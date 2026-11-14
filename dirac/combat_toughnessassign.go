package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugToughAssignOff is TestToughMutation's switch: the toughness statics
// are ignored, combat damage and Crew/Saddle read power.
var DebugToughAssignOff bool

// combatAmtTough is the combat damage creature sl assigns (gorge
// combatDamageAmount): its toughness under a StatCombatDmgToughness
// static, else its power.
//
//go:noinline
func (s *State) combatAmtTough(k *prep, sl Slot) int {
	if !DebugToughAssignOff && (s.staticFlagGen(k, kop.StatCombatDmgToughness, sl) || s.asgnTough(sl)) {
		_, t := s.pt(k, sl)
		return t
	}
	return s.power(k, sl)
}
