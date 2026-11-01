package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cardFlyingOK is matchCardOne's withFlying / withoutFlying read on a card
// off the battlefield (Momo, Friendly Flier's
// Count$ThisTurnCast_Creature.nonLemur+withFlying+YouCtrl reading a cast
// that is still on the stack or has left it): the printed keywords, none face
// down (CR 708.2). A battlefield static's grant reaches permanents (CR 109.2).
func (s *State) cardFlyingOK(f2 kop.FilterFlags2, pc *pcard, fd bool) bool {
	var kw kop.KwMask
	if !fd {
		kw = pc.kw
	}
	fly := kw&kop.KwFlying != 0
	return !(f2&kop.F2WithFlying != 0 && !fly || f2&kop.F2WithoutFlying != 0 && fly)
}
