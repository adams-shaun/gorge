package dirac

import "testing"

// TestActionRoundTrip packs every kind with the edge codes and modes of this
// size class and reads each field back through the accessors. It also checks
// what canonKey relies on: the packed word orders like the (kind, source,
// target, mode) tuple, in every class.
func TestActionRoundTrip(t *testing.T) {
	if int(AkCount) > 1<<(32-actKindShift) {
		t.Fatalf("%d action kinds do not fit the %d kind bits", AkCount, 32-actKindShift)
	}
	if int(NoTarget) > actCodeMask {
		t.Fatalf("NoTarget %d does not fit the code mask %#x", NoTarget, actCodeMask)
	}
	codes := []TCode{0, 1, MaxObj - 1, StackTarget0, PlayerTarget0, PlayerTarget0 + MaxSeats - 1, NoTarget - 1, NoTarget}
	modes := []uint8{0, 1, 0x7f, 0xff}
	type tuple struct {
		k        ActionKind
		src, tgt TCode
		mode     uint8
	}
	var all []tuple
	for k := ActionKind(0); k < AkCount; k++ {
		for _, src := range codes {
			for _, tgt := range codes {
				for _, m := range modes {
					a := MakeAction(k, src, tgt, m)
					if a.Kind() != k || a.Source() != src || a.Target() != tgt || a.Mode() != m {
						t.Fatalf("MakeAction(%d, %d, %d, %d) reads back (%d, %d, %d, %d)", k, src, tgt, m, a.Kind(), a.Source(), a.Target(), a.Mode())
					}
					all = append(all, tuple{k, src, tgt, m})
				}
			}
		}
	}
	less := func(x, y tuple) bool {
		if x.k != y.k {
			return x.k < y.k
		}
		if x.src != y.src {
			return x.src < y.src
		}
		if x.tgt != y.tgt {
			return x.tgt < y.tgt
		}
		return x.mode < y.mode
	}
	for i := 1; i < len(all); i += 97 {
		for j := 0; j < len(all); j += 89 {
			x, y := all[i], all[j]
			ax, ay := MakeAction(x.k, x.src, x.tgt, x.mode), MakeAction(y.k, y.src, y.tgt, y.mode)
			if (ax < ay) != less(x, y) {
				t.Fatalf("packed order of %+v and %+v differs from the tuple order", x, y)
			}
		}
	}
}
