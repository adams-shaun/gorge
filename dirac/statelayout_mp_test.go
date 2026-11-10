//go:build dirac_mp

package dirac

import (
	"testing"
	"unsafe"
)

// The dirac_mp size class has its own layout baseline: four seat slots grow
// every per-seat array, which moves perms to phase 16. The class is not
// padded back to the default class's phase.
var (
	_ = [unsafe.Offsetof(State{}.perms)%64 - 16]struct{}{}
	_ = [16 - unsafe.Offsetof(State{}.perms)%64]struct{}{}
)

// TestStateLayoutPinnedMP: the dirac_mp State's size and perms' phase are
// pinned, as TestStateLayoutPinned pins the default class's.
func TestStateLayoutPinnedMP(t *testing.T) {
	const want = 20864
	if sz := unsafe.Sizeof(State{}); sz != want {
		t.Fatalf("State is %d bytes, pinned at %d", sz, want)
	}
	if off := unsafe.Offsetof(State{}.perms); off%64 != 16 {
		t.Fatalf("perms at %d (mod 64 = %d), pinned mod 64 = 16", off, off%64)
	}
}
