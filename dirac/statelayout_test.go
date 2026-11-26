package dirac

import (
	"unsafe"
)

// layoutClass names the kernel's rows in testdata/layout.tsv.
const layoutClass = "default"

// perms starts on a cache line: four seat slots grow every per-seat array and
// the reserved block and linkPadN (gen.go, seats.go) round the State before
// perms to a multiple of 64. A size change that shifts it is a layout change,
// not a free one. State's total byte budget is held by TestStateSize
// (maxStateBytes); the fixed field inventory is held by
// TestNoNewFixedStateFields (layout_pins_test.go).
var _ = [0 - unsafe.Offsetof(State{}.perms)%64]struct{}{}
