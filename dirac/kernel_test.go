package dirac

import (
	"testing"
	"unsafe"
)

// maxStateBytes is the State bar: 64 KiB since 2026-11-23, when the operator
// raised it from 49 KiB ("you can raise to 64k") for #2456 (a graveyard-functioning
// static's grant timestamp, gyStampSet): the largest pool's (edh) 50552 B. Before it the bar was 49 KiB
// since 2026-11-23, when the operator signed off +1.5 KiB for #3305's re-land (a per-counter CR 613.7 keyword-counter
// stamp in perm.cs): State 41720 -> 42552 B, the largest pool's (tail-18)
// 48656 -> 49488 B. Before it the bar was 48 KiB since 2026-11-23, when the
// operator signed off MaxPending and MaxStack 48 -> 80 (every pending trigger goes on the
// stack together, CR 603.3b; a recorded Scute Swarm pod puts ~60 landfall
// triggers up at once): State 38776 -> 41528 B, the largest pool's 45768 -> 48520 B.
// Before it the bar was 45 KiB since 2026-11-22, when the operator
// signed off maxXTgt 31 -> 63 and maxSacOrd 16 -> 64 for four-seat Commander
// pods: State 38072 -> 38776 B, the largest pool's 45056 -> 45768 B.
// Before it the bar was 44 KiB since 2026-11-19, when the operator
// ordered dirac's capacity stalls in four-seat Commander pods removed:
// MaxPerm 112 -> 192 (a three-word slotMask), the LKI pool 31 -> 63 records
// (its subtype sets moved to a cold region), maxXTgt 15 -> 31, maxCtrOver
// 16 -> 32 and a 16-source activation ledger grew State 29496 -> 37432 B and
// the largest pool's from 35440 to 44328 B. A board's perms past permHi are
// never touched (CopyFrom copies the slots in use), so a game's hot bytes
// grow only with its board. Before it the bar was 36 KiB since 2026-11-18,
// when the operator approved one four-seat build (MaxSeats 4, nine-bit Obj, MaxObj 488 for a
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
const maxStateBytes = 64 * 1024

// stateBar is the State bar the size tests read.
const stateBar = maxStateBytes

func TestStateSize(t *testing.T) {
	sz := unsafe.Sizeof(State{})
	t.Logf("State size: %d bytes", sz)
	if sz > maxStateBytes {
		t.Fatalf("State is %d bytes > %d", sz, maxStateBytes)
	}
}
