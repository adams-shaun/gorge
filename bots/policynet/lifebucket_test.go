package policynet

import (
	"reflect"
	"testing"
)

// sparseHasToken reports whether the encoded bag carries the row for token s.
func sparseHasToken(st State, s string) bool {
	r := hashID(s)
	for _, f := range st.Sparse {
		if f.Row == r {
			return true
		}
	}
	return false
}

// TestMZLifeBucketsDistinguishCommanderLife: life/2 clamped at 12 saturated
// every life >= 24 into one bucket, so a 40-life Commander seat encoded
// exactly like a 24-life one. Buckets 0..11 (life 0..23) must not move, and
// the clamp must now carry a 40-life seat to bucket 20 (and 80 life to 30).
func TestMZLifeBucketsDistinguishCommanderLife(t *testing.T) {
	v := goldenFixture()
	if v.Players[0].ID != 0 || v.Players[1].ID != 1 {
		t.Fatalf("fixture seats are %d/%d, want 0/1", v.Players[0].ID, v.Players[1].ID)
	}

	// Me at 40, opponent at 20.
	v.Players[0].Life = 40
	v.Players[1].Life = 20
	at40 := EncodeStateWith(FeaturesMZ, v, 0, nil)
	if !sparseHasToken(at40, "mz|me|life=20") {
		t.Fatal("seat 0 at 40 life: mz|me|life=20 (life/2 = 20) missing")
	}
	if sparseHasToken(at40, "mz|me|life=12") {
		t.Fatal("seat 0 at 40 life: mz|me|life=12 present, still saturating at 24")
	}

	// Me at 24, opponent still 20: bucket 12, the old saturation point.
	v.Players[0].Life = 24
	at24 := EncodeStateWith(FeaturesMZ, v, 0, nil)
	if !sparseHasToken(at24, "mz|me|life=12") {
		t.Fatal("seat 0 at 24 life: mz|me|life=12 (life/2 = 12) missing")
	}
	if reflect.DeepEqual(at40.Sparse, at24.Sparse) {
		t.Fatal("40-life and 24-life seats encode the same sparse bag")
	}

	// Opponent at 40 vs 25, me held at 20.
	v.Players[0].Life = 20
	v.Players[1].Life = 40
	opp40 := EncodeStateWith(FeaturesMZ, v, 0, nil)
	if !sparseHasToken(opp40, "mz|opp|life=20") {
		t.Fatal("opponent at 40 life: mz|opp|life=20 missing")
	}
	if sparseHasToken(opp40, "mz|opp|life=12") {
		t.Fatal("opponent at 40 life: mz|opp|life=12 present")
	}
	v.Players[1].Life = 25
	opp25 := EncodeStateWith(FeaturesMZ, v, 0, nil)
	if !sparseHasToken(opp25, "mz|opp|life=12") {
		t.Fatal("opponent at 25 life: mz|opp|life=12 missing")
	}
	if reflect.DeepEqual(opp40.Sparse, opp25.Sparse) {
		t.Fatal("opponent at 40 and 25 life encode the same sparse bag")
	}

	// The clamp: 80/2 = 40 lands in bucket 30, not a new bucket 40.
	v.Players[0].Life = 80
	at80 := EncodeStateWith(FeaturesMZ, v, 0, nil)
	if !sparseHasToken(at80, "mz|me|life=30") {
		t.Fatal("seat 0 at 80 life: mz|me|life=30 (the clamp) missing")
	}
	if sparseHasToken(at80, "mz|me|life=31") {
		t.Fatal("seat 0 at 80 life: mz|me|life=31 present, clamp not applied")
	}
}
