package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// maxtgt: five target slots per ability. A modal spell
// declares each chosen mode's targets in mode order (CR 700.2, 601.2c), so a
// choose-two Command with four one-target modes needs four slots. RefTarget0..2
// are contiguous in the Ref enum; slots 3 and 4 are the high ordinals
// RefTarget3/4, so every slot lookup goes through tgtSlot and tgtRef.

// DebugTgt3AsSlot2 is the mutation: RefTarget3 reads slot 2.
var DebugTgt3AsSlot2 bool

// noTargets is a stack entry's target array with every slot empty.
var noTargets = [kop.MaxTargets]uint8{uint8(NoTarget), uint8(NoTarget), uint8(NoTarget), uint8(NoTarget), uint8(NoTarget)}

// tgtSlot is the target slot a RefTarget names (ok false: r is no target ref).
func tgtSlot(r kop.Ref) (int, bool) {
	switch r {
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2:
		return int(r - kop.RefTarget0), true
	case kop.RefTarget3:
		if DebugTgt3AsSlot2 {
			return 2, true
		}
		return 3, true
	case kop.RefTarget4:
		return 4, true
	}
	return 0, false
}

// tgtRef is the RefTarget naming slot i.
func tgtRef(i int) kop.Ref {
	switch i {
	case 3:
		return kop.RefTarget3
	case 4:
		return kop.RefTarget4
	}
	return kop.RefTarget0 + kop.Ref(i)
}

// tgtSlotOf is tgtSlot's slot, or -1.
func tgtSlotOf(r kop.Ref) int {
	if i, ok := tgtSlot(r); ok {
		return i
	}
	return -1
}
