package cardset

import (
	"fmt"
	"testing"
)

func synthNames(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Card %d", i)
	}
	return out
}

func TestSplitByHashIsAPartitionOnOneBit(t *testing.T) {
	names := synthNames(500)
	seen := map[string]bool{}
	for bit := 0; bit < 32; bit++ {
		a, b := SplitByHash(names, bit)
		if len(a)+len(b) != len(names) {
			t.Fatalf("bit %d: %d + %d of %d", bit, len(a), len(b), len(names))
		}
		for _, n := range a {
			if SynthNameHash(n)>>bit&1 != 0 {
				t.Fatalf("bit %d: %q in the wrong half", bit, n)
			}
		}
		seen[fmt.Sprint(a)] = true
	}
	if len(seen) < 2 {
		t.Error("every bit split the same way: the bit is ignored")
	}
}
