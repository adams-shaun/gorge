//go:build !dirac_mp

package dirac

import (
	"testing"
	"unsafe"
)

// maxStateBytes is the State bar: 24 KiB since 2026-11-03, when the largest pool's
// State reached 16352 B. Measured that day with stateCore (the part every fork copies
// whole) padded by 8192 B, eight interleaved pinned reps, user-mode cycles:
// TransitionFDN (300k steps) median 7153M base, 7110M padded; hbot Pick (20k)
// 125.5M base, 128M padded, inside the rep spread; instructions within 0.5%.
// Before it the bar was 16 KiB since 2026-10-21, when MaxPerm
// doubled 64 -> 128 and State grew to about 15 KB. The size
// and bench cost of the wider board were accepted on that date. Before it the
// bar was 12 KiB since 2026-10-18, when State
// was 9168 B: measured that day with State padded to 12288 and 16384 B (a
// trailing field only), six interleaved pinned reps: TransitionFDN, Game,
// Search (one State copy per simulation) and ViewHash all within ~2% of
// 9168 B, no trend with size. It was 9 KB since CardID became 16 bits
// (8.6 KB then), and 8 KB before that. Moving this bar is a deliberate decision, never
// the side effect of a change.
const maxStateBytes = 24 * 1024

func TestStateSize(t *testing.T) {
	sz := unsafe.Sizeof(State{})
	t.Logf("State size: %d bytes", sz)
	if sz > maxStateBytes {
		t.Fatalf("State is %d bytes > %d", sz, maxStateBytes)
	}
}
