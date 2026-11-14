package dirac

import (
	"testing"
	"unsafe"
)

// layoutClass names the kernel's rows in testdata/layout.tsv.
const layoutClass = "default"

// perms starts at the same offset within a cache line as before the reserved
// block (gen.go): a size change that shifts it is a layout change, not a free
// one.
var (
	_ = [unsafe.Offsetof(State{}.perms)%64 - 24]struct{}{}
	_ = [24 - unsafe.Offsetof(State{}.perms)%64]struct{}{}
)

// TestStateLayoutPinned: State's size and perms' phase are pinned in
// testdata/layout.tsv (layout_pins_test.go). New State bytes come out of an
// arena key, a cold region or a scr overlay; a change to either number is a
// deliberate layout change, made by regenerating the file.
func TestStateLayoutPinned(t *testing.T) { checkStateLayout(t) }
