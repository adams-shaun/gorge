//go:build dirac_mp

package dirac

import (
	"testing"
	"unsafe"
)

// layoutClass names this size class's rows in testdata/layout.tsv.
const layoutClass = "mp"

// The dirac_mp size class has its own layout baseline: four seat slots grow
// every per-seat array and the code space holds two fewer objects, which
// puts perms at phase 0. The class is not padded back to the default class's
// phase.
var _ = [0 - unsafe.Offsetof(State{}.perms)%64]struct{}{}

// TestStateLayoutPinnedMP: the dirac_mp State's size and perms' phase are
// pinned in testdata/layout.tsv, as TestStateLayoutPinned pins the default
// class's.
func TestStateLayoutPinnedMP(t *testing.T) { checkStateLayout(t) }
