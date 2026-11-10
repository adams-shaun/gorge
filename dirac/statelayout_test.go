//go:build !dirac_mp

package dirac

import (
	"testing"
	"unsafe"
)

// perms starts at the same offset within a cache line as before the reserved
// block (gen.go): a size change that shifts it is a layout change, not a free
// one.
var (
	_ = [unsafe.Offsetof(State{}.perms)%64 - 24]struct{}{}
	_ = [24 - unsafe.Offsetof(State{}.perms)%64]struct{}{}
)

// TestStateLayoutPinned: State's size and perms' phase are pinned. New State
// bytes come out of genState.rsv, the mech arena, a cold array or a scr
// overlay; a change to this number is a deliberate layout change.
func TestStateLayoutPinned(t *testing.T) {
	const want = 19720
	if sz := unsafe.Sizeof(State{}); sz != want {
		t.Fatalf("State is %d bytes, pinned at %d", sz, want)
	}
	if off := unsafe.Offsetof(State{}.perms); off%64 != 24 {
		t.Fatalf("perms at %d (mod 64 = %d), pinned mod 64 = 24", off, off%64)
	}
}
