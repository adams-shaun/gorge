package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExileCtrPackRoundTrips: the two-byte record returns every kind and
// count it was written with, an empty record reads as no counter, and the
// whole region stays two bytes per object (the State bar, poolbytes_test.go).
func TestExileCtrPackRoundTrips(t *testing.T) {
	if n := len(exileCtrKinds); n > 255 {
		t.Fatalf("%d kinds do not fit the record's kind byte", n)
	}
	for _, kind := range exileCtrKinds {
		code := exileCtrCodeOf(kind)
		if code == 0 {
			t.Fatalf("kind %v has no code", kind)
		}
		for n := uint8(1); n <= maxExileCtrCount; n++ {
			for ver := 0; ver < 256; ver += 37 {
				e := packExileCtr(uint8(ver), code, n)
				if e.kind() != kind || e.n() != n || e.ver() != uint8(ver)&15 {
					t.Fatalf("pack(%d,%v,%d) reads back (%v,%d,%d)", ver, kind, n, e.kind(), e.n(), e.ver())
				}
			}
		}
	}
	if e := exileCtr(0); e.kind() != kop.CounterKind(0) || e.n() != 0 {
		t.Fatalf("the zero record reads (%v,%d)", e.kind(), e.n())
	}
	if got := len(exileCtrKinds); got != 5 {
		t.Fatalf("the code table has %d kinds, want 5 (Void, Ice, Blood, Hit, Croak)", got)
	}
}
