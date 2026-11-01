package dirac

import (
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// maxStateBytes is the State bar: 16 KiB since 2026-10-21, when MaxPerm
// doubled 64 -> 128 and State grew to about 15 KB. The size
// and bench cost of the wider board were accepted on that date. Before it the
// bar was 12 KiB since 2026-10-18, when State
// was 9168 B: measured that day with State padded to 12288 and 16384 B (a
// trailing field only), six interleaved pinned reps: TransitionFDN, Game,
// Search (one State copy per simulation) and ViewHash all within ~2% of
// 9168 B, no trend with size. It was 9 KB since CardID became 16 bits
// (8.6 KB then), and 8 KB before that. Moving this bar is a deliberate decision, never
// the side effect of a change.
const maxStateBytes = 16 * 1024

func TestStateSize(t *testing.T) {
	sz := unsafe.Sizeof(State{})
	t.Logf("State size: %d bytes", sz)
	if sz > maxStateBytes {
		t.Fatalf("State is %d bytes > %d", sz, maxStateBytes)
	}
}

func TestRandomGames(t *testing.T) {
	for g := 0; g < 200; g++ {
		a, b := vanilla.Decks[g%4], vanilla.Decks[(g/4)%4]
		s := NewConfig(vanilla.Table, a, b, uint64(g), Config{Mulligans: uint8(g % 3)})
		pol := NewRandomPolicy(uint64(g) * 7919)
		n, done := PlayRandom(s, &pol, 20000)
		if !done {
			t.Fatalf("game %d not over after %d decisions (turn %d)", g, n, s.turn)
		}
	}
}

func testTable() *Table { return vanilla.Table }

// TestStackHoldsGorgeDepth: the stack holds the 23 entries gorge reached in
// the 10k slice (pairs 6096 and 8504, a trigger chain under several
// spells), so such a game plays on instead of stalling at the capacity.
func TestStackHoldsGorgeDepth(t *testing.T) {
	var s State
	s.ctl = ctlPriority
	for i := 0; i < 23; i++ {
		s.push(stackEntry{kind: seTrigger})
	}
	if int(s.sp) != 23 || MaxStack < 23 {
		t.Fatalf("stack holds %d of 23 (MaxStack %d)", s.sp, MaxStack)
	}
}
