package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// turnUpTapXPays: gorge's turn-up witness is the payment planner's over the
// whole census, which counts a sacrifice-another mana source (Diamond
// Kaleidoscope's "Sacrifice a Prism token: Add one mana of any color") and a
// tap-another one as a unit, so a keyword turn-up the printed sources do not
// pay is a potential play when the tap-another search funds it (CR 605.3a,
// 116.2b). It is the hand special actions' price (handActTapXPays, plot's and
// suspend's) asked for the turn-up action.
//
//go:noinline
func (s *State) turnUpTapXPays(k *prep, p uint8, sl Slot, mode uint8) bool {
	if k.gen == nil {
		return false
	}
	return s.handActTapXPays(k, p, MakeAction(AkTurnFaceUp, TCode(s.perms[sl].obj), 0, mode))
}

// turnUpTapXOnly: no other route pays the turn-up's cost, only the
// tap-another search does, so the planner has no plan to tap (tapForTurnUp
// would find none).
//
//go:noinline
func (s *State) turnUpTapXOnly(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, mode uint8) bool {
	return !s.sourcesPay(k, p, cost) && !s.turnUpRestrPays(k, p, cost, sp, sl) && !s.turnUpRiderPays(k, p, cost) &&
		!s.turnUpPlannerPays(k, p, cost) && !s.turnUpFilterPays(k, p, sl, mode) && s.turnUpTapXPays(k, p, sl, mode)
}

// playTurnUpTapX plays the tap-another search's prefix for the turn-up of o:
// the sacrifices and taps that leave the mana in the pool.
//
//go:noinline
func (s *State) playTurnUpTapX(k *prep, p uint8, o Obj, mode uint8) {
	s.tapXPrefix(k, MakeAction(AkTurnFaceUp, TCode(o), 0, mode), p)
}
