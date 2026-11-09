package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestCounterFeatureLayoutFrozen (kcounters): the unfeaturized counter
// kinds (kop.CtrCharge..) leave the encoder's widths, the counter table it
// hashes and its order exactly as the trained models saw them. The option
// row is 28 wide since the relative-seat columns (24..27; a 24-wide net
// reads only the first 24); the dense vector is 92 wide since the Commander
// block (68..91; a 68-wide net reads only the first 68).
func TestCounterFeatureLayoutFrozen(t *testing.T) {
	if DenseWidth != 92 || EntityRawWidth != 100 || OptionDenseWidth != 28 || OptionSlotWidth != 128 {
		t.Fatalf("widths %d %d %d %d changed", DenseWidth, EntityRawWidth, OptionDenseWidth, OptionSlotWidth)
	}
	want := [...]string{"P1P1", "M1M1", "LOYALTY", "STUN", "SHIELD", "FLYING", "OTHER",
		"INCUBATION", "BAIT", "SOUL", "FELLOWSHIP", "REVIVAL", "STASH", "Deathtouched"}
	if len(counterNames) != len(want) || dirac.FeatCtrKinds != len(want) {
		t.Fatalf("%d counter names, FeatCtrKinds %d; pinned %d", len(counterNames), dirac.FeatCtrKinds, len(want))
	}
	for i, n := range want {
		if counterNames[i] != n {
			t.Fatalf("counterNames[%d] = %q, pinned %q", i, counterNames[i], n)
		}
	}
	for i := 1; i < len(counterOrder); i++ {
		if counterNames[counterOrder[i-1]] > counterNames[counterOrder[i]] {
			t.Fatalf("counterOrder not in name order at %d", i)
		}
	}
}
