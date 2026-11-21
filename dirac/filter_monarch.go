package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The monarch's filter and trigger reads (CR 725; gorge
// engine/effects/trigger_referents.go controlReferentPlayers over the
// Player.isMonarch and EnchantedController.!isMonarch player specs,
// engine/effects/count_player.go HasPropertyisMonarch, rules/trigmatch's
// Phase ValidPlayer$ Player.isMonarch). The designation itself is
// act_monarch.go's genState.mon.

// DebugMonarchFilterInverted (TestMonarchFilterMutation) inverts every
// isMonarch read this file answers: the controller filter word and the Phase
// trigger's monarch-turn test.
var DebugMonarchFilterInverted bool

// monarchSeat is the seat holding the monarch designation, and whether
// anyone does.
func (s *State) monarchSeat() (seat uint8, ok bool) {
	m := s.g.mon & 15
	return m - 1, m != 0
}

// monarchCtrlPredicate is PredMonarchCtrl: the battlefield permanent at sl is
// controlled by the monarch (PredN 0), or by a player who is not (PredN 1).
func (s *State) monarchCtrlPredicate(fl *kop.Filter, sl Slot) bool {
	seat, ok := s.monarchSeat()
	holds := ok && seat == s.perms[sl].ctrl
	if DebugMonarchFilterInverted {
		holds = !holds
	}
	return holds == (fl.PredN == 0)
}

// monarchTurnIs reports whether seat p is the monarch: kop.WhoseMonarch, a
// printed battlefield Phase trigger's "the monarch's end step".
func (s *State) monarchTurnIs(p uint8) bool {
	seat, ok := s.monarchSeat()
	holds := ok && seat == p
	if DebugMonarchFilterInverted {
		holds = !holds
	}
	return holds
}
