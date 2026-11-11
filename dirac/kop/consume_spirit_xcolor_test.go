package kop

import (
	"testing"
	"unsafe"
)

// TestConsumeSpiritXColorAtX pins the XColor$ fold: {X}{1}{B} with XPip black
// is {1}{B}{B} at X = 1 and {1}{B}{B}{B} at X = 2, never generic X. The
// payment reads Pips after AtX, so this is the whole CR 107.3 restriction.
func TestConsumeSpiritXColorAtX(t *testing.T) {
	m := ManaCost{Generic: 1, Pips: [NumManaKinds]uint8{B: 1}, X: 1, XPip: B + 1}
	if got := m.XGeneric(1); got != 0 {
		t.Fatalf("XGeneric(1) with a coloured X = %d, want 0", got)
	}
	if got := m.XAmount(2); got != 2 {
		t.Fatalf("XAmount(2) = %d, want 2", got)
	}
	for _, tc := range []struct {
		x       int
		generic uint8
		black   uint8
	}{
		{x: 0, generic: 1, black: 1},
		{x: 1, generic: 1, black: 2},
		{x: 2, generic: 1, black: 3},
	} {
		got := m.AtX(tc.x)
		if got.X != 0 || got.XPip != 0 {
			t.Errorf("X = %d: announcement not consumed: %+v", tc.x, got)
		}
		if got.Generic != tc.generic || got.Pips[B] != tc.black {
			t.Errorf("X = %d: {generic %d, black %d}, want {generic %d, black %d}",
				tc.x, got.Generic, got.Pips[B], tc.generic, tc.black)
		}
	}
	// The fold must be idempotent: the payment path folds before every read.
	if again := m.AtX(1).AtX(1); again.Pips[B] != 2 {
		t.Errorf("second AtX changed the cost: %+v", again)
	}
	if n := unsafe.Sizeof(ManaCost{}); n != 18 { // XPip widened the table struct from 16
		t.Errorf("sizeof(ManaCost) = %d, want 18", n)
	}
}
