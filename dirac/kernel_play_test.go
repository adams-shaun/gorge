package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

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
