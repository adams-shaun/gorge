package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugStrayHandOwnerSlot (TestStrayFromHistoryMutation): a hand move
// whose owner is a target slot reads target slot 0 instead.
var DebugStrayHandOwnerSlot bool

// handMoveOwnerRef is the reference a hand move's owner is read from.
func handMoveOwnerRef(r kop.Ref) kop.Ref {
	if DebugStrayHandOwnerSlot && r == kop.RefTarget1 {
		return kop.RefTarget0
	}
	return r
}
