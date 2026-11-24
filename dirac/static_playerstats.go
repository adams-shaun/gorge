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

// DebugHandOppAsYou is TestMaxHandOppKernel's switch: the opponents flag is
// read as "the controller". It must make the lockstep diverge. Read in
// maxHand only.
var DebugHandOppAsYou bool

// handUnlimited is maxHand's value for "no maximum".
const handUnlimited = 1 << 20

// maxHand is player p's maximum hand size at cleanup.
func (s *State) maxHand(k *prep, p uint8) int {
	if k.gen == nil || !k.gen.anyHandSize {
		return 7
	}
	max, found, emb, ts := 7, false, false, uint16(0)
	s.eachStaticG(k, func(st statSrc) bool {
		if st.a.Kind != kop.AbStatic || st.a.Static.Kind != kop.StatNoMaxHandSize {
			return true
		}
		sf := &st.a.Static
		// StHandOpp: the controller's opponents (two players: p != ctrl)
		if (sf.StFlags&kop.StHandOpp != 0 && !DebugHandOppAsYou) == (st.ctrl == p) {
			return true
		}
		ctx := s.ctxStatic(st)
		if sf.Cond2 != 0 && !s.condEval(k, sf.Cond2, ctx) {
			return true
		}
		if sf.StFlags&kop.StHandDelta != 0 {
			// CR 402.2: the additive RaiseMaxHandSize$ statics (Minamo
			// Scrollkeeper's +1, Jin-Gitaxias's -7) apply on top of the set
			// maximum, never below zero, and no maximum stays no maximum:
			// gorge maxHandSizeFor returns before the delta walk when best
			// is unlimited, then clamps at 0. The delta rides out of the
			// set-maximum walk (stHandDelta below); gorge sums every
			// matching static's amount the same way.
			return true
		}
		a := int(sf.A)
		if DebugIgnoreHandSize && a != 0 {
			return true
		}
		v := handUnlimited
		if a != 0 {
			v = a - 1
		}
		if sf.StFlags&kop.StDynamic != 0 {
			// gorge setMaxHandSizeSVar: a negative value is no maximum at all
			if v = s.evalV(k, sf.VA, ctx); v < 0 {
				return true
			}
		}
		if st.emb != 0 || emb {
			if found && v != max {
				s.capFull(StallHandSize)
				return false
			}
			max, found, emb = v, true, emb || st.emb != 0
			return true
		}
		tsl := st.sl
		if st.gr {
			tsl = st.gsrc // a granted static takes its grant's timestamp
		}
		if pt := s.layerTs(tsl); !found || pt > ts {
			max, found, ts = v, true, pt
		}
		return true
	})
	if max >= handUnlimited {
		return max // no maximum: no delta applies (gorge maxHandSizeFor)
	}
	// the delta walk rides maxHand's anyHandSize gate (no prep bit of its own)
	max += s.handDelta(k, p)
	return max0(max)
}

// handDelta is the sum of the signed RaiseMaxHandSize$ deltas (StHandDelta)
// affecting p: every static whose controller is p and whose condition holds
// adds its amount (gorge maxHandSizeFor's delta walk, CR 402.2).
//
//go:noinline
func (s *State) handDelta(k *prep, p uint8) int {
	d := 0
	s.eachStaticG(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatNoMaxHandSize ||
			a.Static.StFlags&kop.StHandDelta == 0 || st.ctrl != p {
			return true
		}
		if a.Static.Cond2 != 0 && !s.condEval(k, a.Static.Cond2, s.ctxStatic(st)) {
			return true
		}
		d += int(a.Static.A)
		return true
	})
	return d
}

// max0 is the CR 402.2 clamp of the delta read: the maximum never drops
// below zero.
func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// plProtected reports whether the damage a source controlled by srcCtrl
// would deal to player rcp is prevented by protection from each opponent.
// Not modelled: gorge never protects against a dead source object and
// honours cantPreventDamage (CR 702.16e); neither is reachable in the
// pool, and a damage call here does not say whether its source is live.
func (s *State) plProtected(k *prep, srcCtrl, rcp uint8, combat bool) bool {
	return srcCtrl != rcp && s.replN(k, rcp, rbPlProt) > 0 && !s.noPrev(k, combat) // CR 615.12
}

// playerShroud: player p has shroud (controls a StatPlayerHexproof static
// with A == 2, the shroud read of the A == 1 protection bit): no spell or
// ability may target p, p's own controller's included -- shroud is
// symmetric, unlike hexproof (CR 702.18; gorge playerShroudBlocksTarget,
// read at the offer, target_legal.go, and the resolution recheck,
// stack.go). The corpus's grants are unconditional statics, so no Cond2 is
// consulted; a conditioned grant is refused at the lowering.
//
//go:noinline
func (s *State) playerShroud(k *prep, p uint8) bool {
	if k.gen == nil || k.gen.anyRepl&rbPlHex == 0 { // every shroud static sets rbPlHex (noteStaticPlayerStats)
		return false
	}
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		if s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatPlayerHexproof && a.Static.A == 2 {
				return true
			}
		}
	}
	return false
}
