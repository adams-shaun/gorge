package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// an effect's OptionalDecider$ on Draw and DealDamage, an
// OpMayAsk with a Draw / Damage guard and, when its Obj is set, a named
// decider (gorge effDraw draw.go:80-144, effDealDamage damage_deal.go:91-118).

// DebugMayDeciderCtrl is a mutation switch for the lockstep tests: the
// resolving controller answers every OptionalDecider$ ask.
var DebugMayDeciderCtrl bool

// drawAsks is MayGuardDraw: the next Draw's amount is above 0 and the drawing
// player's library holds a card (gorge canDraw). Else nothing is asked and
// the Draw is skipped (gorge answers "no" itself, draw.go:137-144): run, it
// would draw from the empty library and lose the game.
//
//go:noinline
func (s *State) drawAsks(k *prep, in, next *kop.Instr) bool {
	if s.value(k, next.A) > 0 {
		if p := s.playerRef(k, next.Obj); p < MaxSeats && s.pl[p].libLen() > 0 {
			return true
		}
	}
	s.res.pc += uint8(s.value(k, in.A))
	return false
}

// damageAsks is MayGuardDamage: the next DealDamage's amount is above 0 and
// its recipient is a player or a permanent still on the battlefield (gorge
// damageHasRecipient); an illegal target is no recipient.
//
//go:noinline
func (s *State) damageAsks(k *prep, next *kop.Instr) bool {
	if s.value(k, next.A) <= 0 {
		return false
	}
	if sl, ok := tgtSlot(next.Obj); ok {
		if t := s.tgtCode(sl); isPlayerOrNone(TCode(t)) && TCode(t) != NoTarget {
			return true
		}
	} else if next.Obj == kop.RefController {
		return true
	}
	return s.objRef(k, next.Obj) != NoSlot
}

// mayAsker is the player an OpMayAsk asks: the resolving controller, or the
// OptionalDecider$ player its Obj names (255: none, which a legal target
// cannot produce; loud).
//
//go:noinline
func (s *State) mayAsker(k *prep, in *kop.Instr) uint8 {
	if DebugMayDeciderCtrl {
		return s.res.e.ctrl
	}
	p := s.playerRef(k, in.Obj)
	if p >= MaxSeats {
		s.capFull(StallLowering)
		return 255
	}
	return p
}
