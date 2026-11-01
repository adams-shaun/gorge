package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cardFlyingOK is matchCardOne's withFlying / withoutFlying read on a card
// off the battlefield (Momo, Friendly Flier's
// Count$ThisTurnCast_Creature.nonLemur+withFlying+YouCtrl reading a cast
// that is still on the stack or has left it). gorge's matchesSpec binds the
// object's Derived keywords (rules/statics.go matchesSpec, sc.ExtraKeywords)
// before objectHasKeyword reads the face: the printed keywords (none face
// down, CR 708.2) plus every battlefield Continuous static whose Affected$
// spec matches the object in whatever zone it is, which FeatStaticPT
// already mirrors for the feature view. o is NoObj when the object is
// unknown: the printed keywords alone.
func (s *State) cardFlyingOK(k *prep, f2 kop.FilterFlags2, pc *pcard, o Obj, fd bool) bool {
	var kw kop.KwMask
	if !fd {
		kw = pc.kw
	}
	if o != NoObj && s.objs[o].zone != kop.ZoneBattlefield {
		_, _, g, _ := s.FeatStaticPT(o)
		kw |= g
	}
	fly := kw&kop.KwFlying != 0
	return !(f2&kop.F2WithFlying != 0 && !fly || f2&kop.F2WithoutFlying != 0 && fly)
}
