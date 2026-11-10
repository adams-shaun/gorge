package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Count$Void.<yes>.<no> (ExVoid), gorge cov3BranchHoldsVoid.
// Two halves, either holds:
//
//   - some entry of this turn's zone-add ring left the battlefield and its
//     object is not a land where it now lives (gorge g.Obj(en.Obj) != nil &&
//     !hasType(o, "Land")). gorge never deletes an object, so a token or copy
//     that ceased to exist still names its face and counts, and the ring
//     (enteredReferenced) keeps the kernel from recycling it;
//   - some spell was cast for its warp cost this turn, by either player
//     (SpellsCastThisTurnMatching with no You in "Card.CastSa Spell.Warp").
//     gorge strips the CastSa token per push from THAT cast's pay-time flags
//     and matches the bare "Card" against the object wherever it is now, so
//     a warp spell that was countered, or whose permanent was exiled, still
//     counts: the bit is per push (castPush.prov bit castWarpPush), not a read of
//     the object's present state.
//
// Both rings record only for a table with an ExVoid (prepareGen).

// castWarpPush is castPush.prov's warp bit: the push was a warp cast.
const castWarpPush uint8 = 8

// voidHolds reports whether Void holds. It is cold: two short bounded
// walks, no allocation.
//
//go:noinline
func (s *State) voidHolds(k *prep) bool {
	for i := uint8(0); i < s.g.enteredN; i++ {
		e := &s.cold.entered[i]
		if e.from != uint8(kop.ZoneBattlefield) {
			continue
		}
		if DebugVoidLandToo || !s.voidLand(k, &s.objs[e.o]) {
			return true
		}
	}
	hide := NoObj
	hideP := uint8(2)
	if s.bld().bfl&(bflCostComp|bflPushed) == bflCostComp|bflPushed && s.bld().e.kind == seSpell {
		hide, hideP = s.bld().e.obj, s.bld().e.ctrl // as thisTurnCastSpec: the in-flight cast is not yet in the ledger
	}
	for p := uint8(0); p < MaxSeats; p++ {
		ring := s.castPushes(p)
		if p == hideP && len(ring) > 0 && Obj(ring[len(ring)-1].obj-1) == hide {
			ring = ring[:len(ring)-1]
		}
		for _, r := range ring {
			if r.prov&castWarpPush != 0 && r.prov&castReversedPush == 0 {
				return true
			}
		}
	}
	return false
}

// voidLand reports whether ob, where it now lives, is a land: a live
// permanent reads its copy-aware face (face down: no land type, CR 708.2),
// anything else its card (gorge hasType over o.Face()).
func (s *State) voidLand(k *prep, ob *object) bool {
	c := ob.card
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		if s.faceDown(k, ob.slot) {
			return false
		}
		c = s.cardAt(ob.slot)
	}
	return k.t.Cards[c].Types&kop.TLand != 0
}

// DebugVoidLandToo is the mutation test's switch (TestVoidLockstep): a land
// departure counts too.
var DebugVoidLandToo bool
