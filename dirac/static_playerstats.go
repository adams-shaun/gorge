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
// Two statics of one player apply in timestamp order (CR 613.7), so the
// permanent with the latest layer timestamp (layerTs) sets the
// maximum, as gorge maxHandSizeFor takes the latest. An emblem (gorge's
// Effect-delivered route) carries no timestamp here: an emblem whose value
// differs from another hand-size static of the player stalls
// (StallHandSize) rather than guess the winner.
//
// Protection: gorge playerProtectedFrom holds the player against a source
// whose controller is an opponent (CR 702.16c), targeting (the kernel's
// StatPlayerHexproof, which the static keeps) and, before any replacement,
// all damage from such a source (CR 702.16e; gorge emit). replDamage reads
// rbPlProt (a StatPlayerHexproof with Static.A == 1).

// StallHandSize: an emblem's hand-size static and another of the same
// player with a different value.
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
	max, found, emb, ts := 7, false, false, uint16(0)
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
		if st.emb != 0 || emb {
			if found && v != max {
				s.capFull(StallHandSize)
				return false
			}
			max, found, emb = v, true, emb || st.emb != 0
			return true
		}
		if pt := s.layerTs(st.sl); !found || pt > ts {
			max, found, ts = v, true, pt
		}
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
	return srcCtrl != rcp && s.replN(k, rcp, rbPlProt) > 0 && !s.noPrev(k) // CR 615.12
}
