package oracle

import "testing"

// A creature printing split second (Celestial Crusader, Sulfur Elemental)
// carries the keyword in gorge's derived list on the battlefield; the kernel
// models it as CardDef.SplitSecond, read off a spell on the stack, so the
// projection maps it to no bit instead of failing.
func TestSplitSecondProjectsToNoBit(t *testing.T) {
	if b, ok := gorgeKw("Split second"); !ok || b != 0 {
		t.Fatalf("gorgeKw(%q) = %v, %v; want 0, true", "Split second", b, ok)
	}
}
