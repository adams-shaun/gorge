package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the numeric maximum hand size (The Ten Rings, Doctor
// Octopus, Master Planner) and protection from each opponent (Absolute
// Virtue), both `Affected$ You` statics.
//
// Maximum hand size: gorge maxHandSizeFor (engine/rules/combat.go) walks the
// printed Continuous statics, then the Effect-delivered ones, and the FIRST
// that affects the player wins; the default is seven. Static.A is the
// value: 0 unlimited (no maximum, the existing NoMaxHandSize), else A-1.
// eachStatic walks the permanents in slot order and then the emblems, which
// is gorge's order only when the statics agree: two hand-size statics of
// one player with DIFFERENT values stall (StallHandSize) rather than guess
// the winner.
//
// Protection: gorge playerProtectedFrom holds the player against a source
// whose controller is an opponent (CR 702.16c), targeting (the kernel's
// StatPlayerHexproof, which the static keeps) and, before any replacement,
// all damage from such a source (CR 702.16e; gorge emit). replDamage reads
// rbPlProt (a StatPlayerHexproof with Static.A == 1).

// StallHandSize: two hand-size statics of one player with different values.
const StallHandSize = 118

func init() { stallNames[StallHandSize] = "handsize" }

// DebugIgnoreHandSize is TestPlstatMutation's switch: a numeric maximum
// hand size is treated as absent (seven). It must make the lockstep
// diverge. Read in maxHand only.
var DebugIgnoreHandSize bool

// handUnlimited is maxHand's value for "no maximum".
const handUnlimited = 1 << 20

// maxHand is player p's maximum hand size at cleanup.
func (s *State) maxHand(k *prep, p uint8) int {
	if k.gen == nil || !k.gen.anyHandSize {
		return 7
	}
	max, found := 7, false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind != kop.AbStatic || st.a.Static.Kind != kop.StatNoMaxHandSize || st.ctrl != p {
			return true
		}
		a := int(st.a.Static.A)
		if DebugIgnoreHandSize && a != 0 {
			return true
		}
		v := handUnlimited
		if a != 0 {
			v = a - 1
		}
		if found && v != max {
			s.capFull(StallHandSize)
			return false
		}
		max, found = v, true
		return true
	})
	return max
}

// plProtected reports whether the damage a source controlled by srcCtrl
// would deal to player rcp is prevented by protection from each opponent.
// Not modelled: gorge never protects against a dead source object and
// honours cantPreventDamage (CR 702.16e); neither is reachable in the
// pool, and a damage call here does not say whether its source is live.
func (s *State) plProtected(k *prep, srcCtrl, rcp uint8) bool {
	return srcCtrl != rcp && s.replN(k, rcp, rbPlProt) > 0
}
