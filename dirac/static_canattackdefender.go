package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugCanAtkDefNoCond (test only, TestCanatkdefMutation): canAttackDefender
// ignores the static's condition and always lifts the defender wall.
var DebugCanAtkDefNoCond bool

// canAttackDefender reports whether a StatCanAttackDefender static (its Cond
// holding, its Filter matching the creature at sl) lets the defender at sl
// attack (CR 702.3b; gorge combat.AttackAllowedThroughDefender). Cold: called
// only for a Defender creature while a card in the table carries the static.
//
//go:noinline
func (s *State) canAttackDefender(k *prep, sl Slot) bool {
	if DebugCanAtkDefNoCond {
		hit := false
		s.eachStatic(k, func(st statSrc) bool {
			a := st.a
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCanAttackDefender && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
				hit = true
				return false
			}
			return true
		})
		return hit
	}
	return s.canAtkDefGen(k, sl) // static_emblemcapture.go
}
