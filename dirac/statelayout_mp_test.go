//go:build dirac_mp

package dirac

import (
	"testing"
	"unsafe"
)

// The dirac_mp size class has its own layout baseline: four seat slots grow
// every per-seat array and the code space holds two fewer objects, which
// puts perms at phase 0. The class is not padded back to the default class's
// phase.
var _ = [0 - unsafe.Offsetof(State{}.perms)%64]struct{}{}

// TestStateLayoutPinnedMP: the dirac_mp State's size and perms' phase are
// pinned, as TestStateLayoutPinned pins the default class's.
func TestStateLayoutPinnedMP(t *testing.T) {
	// +24: coldArrays.hexUB's slice header, as in the default class.
	const want = 21704
	if sz := unsafe.Sizeof(State{}); sz != want {
		t.Fatalf("State is %d bytes, pinned at %d", sz, want)
	}
	if off := unsafe.Offsetof(State{}.perms); off%64 != 0 {
		t.Fatalf("perms at %d (mod 64 = %d), pinned mod 64 = 0", off, off%64)
	}
}
