package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// remCtrlCount is ExOppExtreme OppExtRemCtrl: the living players other than
// skip that control some object of the resolution's remembered set, each
// counted once (gorge count_player.go HasPropertyIsRememberedOrController:
// controllersOf over the remembered entries, the live controller). A
// battlefield permanent's controller is its slot's; any other object's is its
// owner (a drawn card sits in its owner's hand). Cut a Deal's "you draw a
// card for each opponent who drew a card this way". DebugRemCtrlNone counts
// no member.
//
//go:noinline
func (s *State) remCtrlCount(skip uint8) int {
	var in [MaxSeats]bool
	for _, o := range s.g.rem[:s.g.nrem] {
		ob := &s.objs[o]
		p := ob.owner()
		if ob.zone == kop.ZoneBattlefield {
			p = s.perms[ob.slot].ctrl
		}
		if p < MaxSeats {
			in[p] = true
		}
	}
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if p != skip && !s.out(p) && in[p] && !DebugRemCtrlNone {
			n++
		}
	}
	return n
}

// DebugRemCtrlNone (test only, TestFourSeatCutADealLockstep): no member
// counts, so Cut a Deal's caster draws nothing.
var DebugRemCtrlNone bool
