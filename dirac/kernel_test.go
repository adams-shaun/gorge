package dirac

import (
	"testing"
	"unsafe"
)

// maxStateBytes is the State bar: 36 KiB since 2026-11-18, when the operator
// approved one four-seat build (MaxSeats 4, nine-bit Obj, MaxObj 488 for a
// four-seat Commander pod) in place of the two-seat build: State grew 21456 ->
// 29432 B and the largest pool's to 34616 B. 36 KiB was the four-seat class's
// (dirac_mp) bar since 2026-11-14, when the operator approved room for 16-bit
// counter kinds; it was 32 KiB since 2026-11-12, when the operator approved
// 16-bit object codes. It covers State plus a pool's cold arrays, about 34 KB
// of a 48 KiB L1d at the largest.
// Before it the two-seat build's bar was 24 KiB since 2026-11-03, when the largest pool's
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
const maxStateBytes = 36 * 1024

// stateBar is the State bar the size tests read.
const stateBar = maxStateBytes

func TestStateSize(t *testing.T) {
	sz := unsafe.Sizeof(State{})
	t.Logf("State size: %d bytes", sz)
	if sz > maxStateBytes {
		t.Fatalf("State is %d bytes > %d", sz, maxStateBytes)
	}
}
