package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugPropNegPositive (a mutation hook): PredNotF2
// returns the POSITIVE flag, so the lockstep must diverge on !targetedBy /
// !ThisTurnEntered / !wasDealtDamageThisTurn.
var DebugPropNegPositive bool

// notF2Perm is PredNotF2 on a battlefield permanent: the F2 bit PredN is
// absent (gorge matchPredicate '!'). None of the three negatable flags reads
// the source or the layer.
//
//go:noinline
func (s *State) notF2Perm(k *prep, fl *kop.Filter, sl Slot) bool {
	return s.flags2Perm(k, kop.FilterFlags2(1)<<fl.PredN, sl, NoSlot, 0) == DebugPropNegPositive
}

// notF2Card is PredNotF2 on a card off the battlefield (mirrors the F2 reads
// of matchCardOneAt): EnteredThisTurn is the zone-change stamp, TargetedBy
// reads the resolving entry; no object (o == NoObj) holds none of them. kgen
// lowers no other bit: the negated !wasDealtDamageThisTurn stays refused
// (kgen lower_selfcondition.go); the positive form reads s.g.dmgTurn, which
// survives a zone change like gorge's WasDealtDamageThisTurn.
//
//go:noinline
func (s *State) notF2Card(fl *kop.Filter, o Obj) bool {
	f := kop.FilterFlags2(1) << fl.PredN
	var has bool
	if o != NoObj {
		switch f {
		case kop.F2EnteredThisTurn:
			has = s.g.zoneTurn.has(o)
		case kop.F2TargetedBy:
			has = s.flags2Obj(f, o)
		}
	}
	return has == DebugPropNegPositive
}
