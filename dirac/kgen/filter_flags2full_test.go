package kgen

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterFlags2Full: the kernel's FilterFlags2 has no free bit — all 32
// constants are named (dirac/kop/filters.go, kernel filter_negations.go) — so
// the next boolean property a clause needs is a FilterWord literal in the
// spill program (filter_spill.go), never a new Flags2 bit. Adding a 33rd F2
// constant fails to compile; this pins the current fill and that the compiler
// maps every clause flag onto a named bit.
func TestFilterFlags2Full(t *testing.T) {
	var all kop.FilterFlags2
	for _, f2 := range []kop.FilterFlags2{
		kop.F2Kicked, kop.F2CastByYou, kop.F2EnteredThisTurn, kop.F2ChosenType, kop.F2ChosenColor,
		kop.F2Remembered, kop.F2NotRemembered, kop.F2ChosenCard, kop.F2NotChosenCard, kop.F2AttachedToT0,
		kop.F2TargetedBy, kop.F2SameNameAsSource, kop.F2WithFlying, kop.F2WithoutFlying, kop.F2Emblem,
		kop.F2Opponent, kop.F2You, kop.F2Basic, kop.F2SharesColorRem, kop.F2NotTriggerObj, kop.F2WasDamaged,
		kop.F2Legendary, kop.F2Saddled, kop.F2BasePermanent, kop.F2Sneaked, kop.F2Unblocked, kop.F2BlockedBySrc,
		kop.F2DamagedBy, kop.F2CrewedSrc, kop.F2Warped, kop.F2TgtPlayerCtrl, kop.F2DefendingCtrl,
	} {
		if bits.OnesCount32(uint32(f2)) != 1 {
			t.Errorf("F2 %d is not a single bit", f2)
		}
		all |= f2
	}
	if n := bits.OnesCount32(uint32(all)); n != 32 {
		t.Errorf("%d of 32 FilterFlags2 bits named; the free ones are boolean properties the spill words carry", n)
	}
}
