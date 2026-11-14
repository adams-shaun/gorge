package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cmdGreatestMV is kop.ExCmdGreatestMV: the greatest mana value among seat
// p's commanders on the battlefield (the permanent's, 0 face down, CR 708.2;
// the ExValidAgg read) or in the command zone (the card's printed mana
// value). gorge folds Count$ValidBattlefield,Command
// Card.IsCommander+YouOwn$GreatestCardManaCost over both zones; a seat's
// commander is always its own card (CR 903.3), so YouOwn is the seat.
//
//go:noinline
func (s *State) cmdGreatestMV(k *prep, p uint8) int {
	if s.cfg&cfgCommander == 0 {
		return 0
	}
	p = SeatIx(p)
	best := 0
	for j := 0; j < 2; j++ {
		o := s.cmdObj(p, j)
		if o == NoObj {
			continue
		}
		v := 0
		switch ob := &s.objs[o]; ob.zone {
		case kop.ZoneBattlefield:
			if !s.faceDown(k, ob.slot) {
				v = s.mvPerm(k, ob.slot)
			}
		case kop.ZoneCommand:
			v = s.objMV(k, ob.card, o)
		default:
			continue
		}
		if v > best {
			best = v
		}
	}
	return best
}
