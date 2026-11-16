package cardset

import (
	"slices"
	"testing"
)

// bucketIndex maps each name to the index of the bucket holding it.
func bucketIndex(t *testing.T, buckets [][]string) map[string]int {
	t.Helper()
	out := map[string]int{}
	for i, b := range buckets {
		if !slices.IsSorted(b) {
			t.Errorf("bucket %d is not sorted by name", i)
		}
		for _, n := range b {
			if _, dup := out[n]; dup {
				t.Errorf("%q is in two buckets", n)
			}
			out[n] = i
		}
	}
	return out
}

func TestTailBucketsPartitionIntoTailBucketCount(t *testing.T) {
	names := synthNames(3*TailBucketCount*TailBucket/2 + 7)
	input := slices.Clone(names)
	got := TailBuckets(names)
	if !slices.Equal(names, input) {
		t.Error("TailBuckets reordered its input")
	}
	if len(got) != TailBucketCount {
		t.Fatalf("%d buckets, want %d", len(got), TailBucketCount)
	}
	idx := bucketIndex(t, got)
	if len(idx) != len(input) {
		t.Fatalf("the buckets hold %d names of %d", len(idx), len(input))
	}
	for n, i := range idx {
		if i != TailBucketOf(n) {
			t.Errorf("%q is in bucket %d, TailBucketOf says %d", n, i, TailBucketOf(n))
		}
	}
	// The order of the input does not matter: only the names do.
	slices.Reverse(input)
	if again := TailBuckets(input); !slices.EqualFunc(got, again, slices.Equal[[]string]) {
		t.Error("TailBuckets depends on input order")
	}
}

// TestTailBucketsMembershipIgnoresOtherCards: lowering or unlowering one card moves
// no other card's bucket, which is what lets a repro recorded on tail-NN keep
// naming it.
func TestTailBucketsMembershipIgnoresOtherCards(t *testing.T) {
	names := synthNames(TailBucketCount * TailBucket)
	base := bucketIndex(t, TailBuckets(names))
	if len(base) != len(names) {
		t.Fatalf("the base buckets hold %d names of %d", len(base), len(names))
	}
	moved := 0
	for _, drop := range []int{0, 1, len(names) / 2, len(names) - 1} {
		less := slices.Concat(names[:drop], names[drop+1:])
		for n, i := range bucketIndex(t, TailBuckets(less)) {
			if base[n] != i {
				moved++
				t.Errorf("dropping %q moved %q from bucket %d to %d", names[drop], n, base[n], i)
			}
		}
		more := append(slices.Clone(names), "Extra Card "+names[drop])
		for n, i := range bucketIndex(t, TailBuckets(more)) {
			if j, ok := base[n]; ok && j != i {
				moved++
				t.Errorf("adding a card moved %q from bucket %d to %d", n, j, i)
			}
		}
	}
	if moved > 0 {
		t.Logf("%d cards moved", moved)
	}
	// Precondition: the cards do spread over more than one bucket.
	seen := map[int]bool{}
	for _, i := range base {
		seen[i] = true
	}
	if len(seen) < TailBucketCount/2 {
		t.Errorf("only %d of %d buckets are used", len(seen), TailBucketCount)
	}
}
