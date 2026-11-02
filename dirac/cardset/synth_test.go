package cardset

import (
	"fmt"
	"slices"
	"testing"
)

func synthNames(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Card %d", i)
	}
	return out
}

func TestTailBucketsCutEveryBucketInHashOrder(t *testing.T) {
	names := synthNames(2*TailBucket + 7)
	input := slices.Clone(names)
	got := TailBuckets(names)
	if !slices.Equal(names, input) {
		t.Error("TailBuckets reordered its input")
	}
	if len(got) != 3 || len(got[0]) != TailBucket || len(got[1]) != TailBucket || len(got[2]) != 7 {
		t.Fatalf("bucket sizes: %d buckets", len(got))
	}
	var all []string
	for _, b := range got {
		if !slices.IsSorted(b) {
			t.Error("a bucket is not sorted by name")
		}
		all = append(all, b...)
	}
	slices.Sort(all)
	slices.Sort(input)
	if !slices.Equal(all, input) {
		t.Error("the buckets are not a partition of the input")
	}
	// The order of the input does not matter: only the names do.
	slices.Reverse(input)
	if again := TailBuckets(input); !slices.EqualFunc(got, again, slices.Equal[[]string]) {
		t.Error("TailBuckets depends on input order")
	}
	// Every card of an earlier bucket hashes no later than every card of a later one.
	maxHash := func(b []string) (m uint32) {
		for _, n := range b {
			m = max(m, SynthNameHash(n))
		}
		return m
	}
	minHash := func(b []string) uint32 {
		m := ^uint32(0)
		for _, n := range b {
			m = min(m, SynthNameHash(n))
		}
		return m
	}
	if maxHash(got[0]) > minHash(got[1]) {
		t.Error("buckets are not cut in hash order")
	}
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
