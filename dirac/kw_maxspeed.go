package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kw_maxspeed.go: the max-speed reads of
// (dirac/kgen lower_maxspeedrepl.go): Count$ PlayerCountPropertyYou$HasPropertyMaxSpeed
// and the speed gate on Far Fortune's granted plus-N damage replacement.

// DebugSpeedGrantsOpen (TestMaxSpeedGrantsLockstepMutation) makes both reads
// answer "max speed" at any speed.
var DebugSpeedGrantsOpen bool

// maxSpeedProp is ExYourSpeed N 1: 1 when p's speed has reached the
// maximum (gorge playerMaxSpeedProperty: Speed >= 4), else 0.
func (s *State) maxSpeedProp(p uint8) int {
	if DebugSpeedGrantsOpen || s.speedOf(p) >= 4 {
		return 1
	}
	return 0
}

// replHostLive reports whether the damage replacement a on the permanent at
// b is live: its Repl.Cond (a granted max-speed gate) holds for the host.
//
//go:noinline
func (s *State) replHostLive(k *prep, a *kop.Ability, b Slot) bool {
	if DebugSpeedGrantsOpen {
		return true
	}
	return s.condEval(k, a.Repl.Cond, evalCtx{src: s.perms[b].obj, srcSl: b, ctrl: s.perms[b].ctrl})
}

// Max-speed grants with a mana cost (CR 702.179e,
// 113.6/602.1). gorge offers a "Max speed —" granted activated ability
// (battlefield: legal_walk_battlefield.go; a Surveyor in the graveyard:
// legal_walk_offbattlefield_grants.go) through offerCastable, which in the
// real offer walk prices its mana cost from the player's floating pool alone
// (statics_cost.go manaFeasiblePricedP, hyp nil); the oracle adapter maps
// gorge's land-funded potential plays only for cast and ability, never for a
// "granted" play. So the kernel offers a mana-costed max-speed grant only
// when the floating pool pays it.

// DebugSpeedGrantLands is the test mutation: price a max-speed grant with
// lands too (skip the pool-only test); the lockstep must then diverge.
var DebugSpeedGrantLands bool

// maxSpeedGraveOK: a graveyard ability gated on max speed is offered to p
// (speed 4, the floating pool pays its mana cost). The pool includes the
// restricted batches an activated-ability payment admits (Boommobile's four
// mana "only to activate abilities"), so the gate reads poolPaysAct rather
// than the ordinary pool alone (CR 106.6).
func (s *State) maxSpeedGraveOK(k *prep, p uint8, o Obj, a *kop.Ability) bool {
	if s.speedOf(p) < speedMaxV {
		return false
	}
	if !DebugSpeedGrantLands && !s.poolPaysAct(k, p, &a.Cost.Mana, payDesc{class: pdAct, card: s.objs[o].card}) {
		return false
	}
	if SpeedCounting {
		SpeedCounts[SpeedCntGatedOffer].Add(1)
	}
	return true
}

// maxSpeedPoolOK: a battlefield max-speed gated ability is offered only when
// the floating pool pays its mana cost (true when it has none).
func (s *State) maxSpeedPoolOK(p uint8, a *kop.Ability) bool {
	return DebugSpeedGrantLands || s.poolPays(p, &a.Cost.Mana)
}
