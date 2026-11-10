package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// castTargetsAll is Tf2TargetsFilter2 (gorge TargetShapeMatches'
// TargetsValid$): at least one target of spell e's whole chain (gorge
// ChainTargets, root Targets plus SubTargets) is a battlefield
// object matching f, read with the trigger's source and controller
// (Card.Self is the heroic permanent). kgen lowers only battlefield object
// alternatives, so a player or off-battlefield target never matches; a spell
// with no targets never matches.
//
//go:noinline
func (s *State) castTargetsAll(k *prep, f kop.FilterID, e *stackEntry, l trigListener) bool {
	var codes [kop.MaxTargets + maxXTgt]uint8
	n := s.chainTargets(e, &codes)
	for i := 0; i < n; i++ {
		t := codes[i]
		if isPlayerTarget(t) {
			if s.castTargetsPlayer(k, f, CodeSeat(t), l.ctrl) {
				return true
			}
			continue
		}
		if isObjTarget(t) && s.objs[t].zone == kop.ZoneBattlefield && s.matchPermGen(k, f, s.objs[t].slot, l.ctrl, l.sl, layerFull) {
			return true
		}
	}
	return false
}

// gorge (a4d11dd45) made the spell-cast target-shape
// qualifiers read the WHOLE target chain (trigmatch.ChainTargets: the root
// declaration's Targets then every SubAbility$ link's SubTargets).
//
//   - TargetsValid$ is any-match: at least one target of the chain matches.
//   - IsSingleTarget$ True counts DISTINCT targets over the chain (objects by
//     id, players by seat): two slots naming one permanent are one target.
//
// Cold and out of line; the hot sites test the trigger flags once.

// chainTargets fills codes with the spell's whole cast-time target chain: every
// set declaration slot (root and SubAbility$ link alike), then every extra
// pick of a multi-object slot. A mid-resolution declaration has not been
// chosen when a SpellCast trigger is evaluated, so only cast-time slots are set.
//
//go:noinline
func (s *State) chainTargets(e *stackEntry, codes *[kop.MaxTargets + maxXTgt]uint8) int {
	n := 0
	for i := 0; i < int(e.nt); i++ {
		if c := e.tgt[i]; c != NoTarget {
			codes[n] = c
			n++
		}
	}
	x := s.xtOf(e)
	for j := 0; j < int(x.n); j++ {
		if c := x.code[j]; c != NoTarget {
			codes[n] = c
			n++
		}
	}
	return n
}

// distinctTargets is the number of different codes among codes[:n] (an object
// target's code is its object index, a player target's its seat code).
//
//go:noinline
func distinctTargets(codes *[kop.MaxTargets + maxXTgt]uint8, n int) int {
	d := 0
	for i := 0; i < n; i++ {
		dup := false
		for j := 0; j < i; j++ {
			if codes[j] == codes[i] {
				dup = true
				break
			}
		}
		if !dup {
			d++
		}
	}
	return d
}
