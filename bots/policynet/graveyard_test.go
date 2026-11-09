package policynet

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
)

func graveView() view.View {
	v := ownLibView()
	v.Players[0].Graveyard = []view.CardView{
		{ID: 3, Name: "Goblin", Types: "Creature Goblin", ManaCost: "1 R", Power: 2, Toughness: 1, Keywords: []string{"Haste"}, Owner: 0},
		{ID: 4, Name: "Bolt", Types: "Instant", ManaCost: "R", SpellAPI: "DealDamage", Owner: 0},
	}
	v.Players[1].Graveyard = []view.CardView{
		{ID: 5, Name: "Wall", Types: "Artifact Creature", ManaCost: "3", Power: 0, Toughness: 4, Keywords: []string{"Defender"}, Owner: 1},
	}
	return v
}

// mz-grave is mz plus per-card graveyard properties for both seats, mz is
// untouched, and the opponent's rows are tagged apart from the viewer's.
func TestGraveyardFeatures(t *testing.T) {
	v := graveView()
	if len(v.Players[0].Graveyard) != 2 || len(v.Players[1].Graveyard) != 1 {
		t.Fatal("precondition: graveyards not populated")
	}
	mz := EncodeStateWith(FeaturesMZ, v, 0, nil)
	st := EncodeStateWith(FeaturesMZGrave, v, 0, nil)
	if !reflect.DeepEqual(st.Dense, mz.Dense) || len(st.Sparse) <= len(mz.Sparse) {
		t.Fatalf("mz-grave is not mz plus rows: %d vs %d", len(st.Sparse), len(mz.Sparse))
	}
	for name, want := range map[string]float32{
		"mz|gy-me|type|Creature": 1, "mz|gy-me|type|Instant": 1, "mz|gy-me|mv2": 1, "mz|gy-me|mv1": 1,
		"mz|gy-me|api|DealDamage": 1, "mz|gy-me|kw|haste": 1, "mz|gy-me|cr|p2|t1": 1, "mz|gy-me|n": 2,
		"mz|gy-me|n|type|Creature": 1,
		"mz|gy-opp|type|Artifact":  1, "mz|gy-opp|mv3": 1, "mz|gy-opp|kw|defender": 1, "mz|gy-opp|cr|p0|t4": 1, "mz|gy-opp|n": 1,
	} {
		if got, ok := sparseValue(st, name); !ok || got != want {
			t.Fatalf("%s = %v (present %v), want %v", name, got, ok, want)
		}
		if _, ok := sparseValue(mz, name); ok {
			t.Fatalf("%s present under plain mz", name)
		}
	}
	// Seat 1's view swaps which graveyard is "me".
	st1 := EncodeStateWith(FeaturesMZGrave, v, 1, nil)
	if got, ok := sparseValue(st1, "mz|gy-opp|n"); !ok || got != 2 {
		t.Fatalf("seat 1 gy-opp n = %v %v", got, ok)
	}
	for i := 1; i < len(st.Sparse); i++ {
		if st.Sparse[i].Row < st.Sparse[i-1].Row {
			t.Fatal("sparse bag not sorted")
		}
	}
	if !reflect.DeepEqual(EncodeStateWith(FeaturesMZGrave, v, 0, nil), st) {
		t.Fatal("encoding is not deterministic")
	}
	// An empty graveyard adds nothing.
	empty := ownLibView()
	empty.Players[0].Graveyard = nil
	if a, b := EncodeStateWith(FeaturesMZGrave, empty, 0, nil), EncodeStateWith(FeaturesMZ, empty, 0, nil); !reflect.DeepEqual(a, b) {
		t.Fatal("empty graveyards changed the encoding")
	}
}

// The mz-grave hash is distinct, existing hashes did not move, and the set
// maps back from a hash and round-trips a checkpoint.
func TestGraveyardCheckpoint(t *testing.T) {
	h := EncoderHashFor(FeaturesMZGrave)
	for _, fs := range []FeatureSet{FeaturesV1, FeaturesMZ, FeaturesMZOppHand, FeaturesMZOracle, FeaturesEntity, FeaturesMZOwnLib} {
		if EncoderHashFor(fs) == h {
			t.Fatalf("mz-grave hash collides with %s", fs)
		}
	}
	if EncoderHashFor(FeaturesMZOwnLib) != 0x74f6f973c0a86be7 {
		t.Fatal("mz-ownlib hash moved")
	}
	if fs, ok := FeaturesForHash(h); !ok || fs != FeaturesMZGrave || FeaturesMZGrave.Diagnostic() {
		t.Fatalf("FeaturesForHash = %v %v", fs, ok)
	}
	if fs, ok := onPolicyFeatures(fmt.Sprintf("%016x", h)); !ok || fs != FeaturesMZGrave {
		t.Fatalf("onPolicyFeatures = %v %v", fs, ok)
	}
	if fs, err := ParseFeatureSet("mz-grave"); err != nil || fs != FeaturesMZGrave {
		t.Fatalf("ParseFeatureSet = %v %v", fs, err)
	}
	m := NewModel(TableRows, 4, 3, rand.New(rand.NewPCG(1, 2)))
	m.Features = FeaturesMZGrave
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil || got.Features != FeaturesMZGrave {
		t.Fatalf("round trip: %v %v", got, err)
	}
}
