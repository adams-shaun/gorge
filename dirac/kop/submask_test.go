package kop

import "testing"

// TestSubMaskWide: subtype indexes straddle the old 128-bit boundary and the
// third word is seen by every set operation.
func TestSubMaskWide(t *testing.T) {
	if MaxSubtypes <= 128 {
		t.Fatalf("precondition: MaxSubtypes = %d, want more than the old 128", MaxSubtypes)
	}
	m := SubMask{}.With(127).With(128).With(191)
	for _, i := range []int{127, 128, 191} {
		if !m.Has(i) {
			t.Errorf("Has(%d) = false", i)
		}
	}
	for _, i := range []int{0, 126, 129, 190, 192, -1} {
		if m.Has(i) {
			t.Errorf("Has(%d) = true", i)
		}
	}
	hi := SubMask{}.With(191)
	if !hi.Any() {
		t.Error("Any ignores the third word")
	}
	if got := hi.Or(SubMask{}.With(3)); !got.Has(191) || !got.Has(3) {
		t.Error("Or loses a word")
	}
	if got := m.And(hi); got != hi {
		t.Errorf("And = %v, want only 191", got)
	}
	if got := m.AndNot(hi); got.Has(191) || !got.Has(128) || !got.Has(127) {
		t.Errorf("AndNot = %v", got)
	}
	if (SubMask{}).With(192).Any() {
		t.Error("With(192) is out of range and must not set a bit")
	}
}
