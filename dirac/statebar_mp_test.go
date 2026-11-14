//go:build dirac_mp

package dirac

import (
	"testing"
	"unsafe"
)

// maxStateBytesMP is the dirac_mp class's State bar: 36 KiB since 2026-11-14,
// when the operator approved room for 16-bit counter kinds (perm.ck,
// lkiRec.ck, the overflow table's kinds): State grew 28024 -> 28728 B and the
// largest pool 32680 -> 33384 B, 88 B of headroom under 32 KiB before it.
// Before it the bar was 32 KiB since 2026-11-12, when the operator approved an
// mp-only bar for 16-bit object codes (MaxObj 488 for a four-seat Commander
// pod, OBJPEAK in the capacity notes). It covers the mp State plus a pool's
// cold arrays, about 33 KB of a 48 KiB L1d at the largest. The default class
// keeps maxStateBytes (kernel_test.go), and its State is untouched by the mp
// knobs. Moving this bar is a deliberate decision, never the side effect of a
// change.
const maxStateBytesMP = 36 * 1024

// stateBar is the State bar the size tests read in this size class.
const stateBar = maxStateBytesMP

func TestStateSize(t *testing.T) {
	sz := unsafe.Sizeof(State{})
	t.Logf("State size: %d bytes", sz)
	if sz > maxStateBytesMP {
		t.Fatalf("State is %d bytes > %d", sz, maxStateBytesMP)
	}
}
