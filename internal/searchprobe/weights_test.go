package searchprobe

import (
	"math"
	"reflect"
	"testing"
)

type fixedFloat struct {
	values []float64
	at     int
}

func (r *fixedFloat) Float64() float64 { v := r.values[r.at]; r.at++; return v }

func TestWeightsNormalizeWithoutUnderflow(t *testing.T) {
	for _, tc := range []struct {
		logs, want []float64
		ess        float64
	}{
		{[]float64{-10000, -10000, -10000, -10000}, []float64{.25, .25, .25, .25}, 4},
		{[]float64{math.Inf(-1), -9000, math.Inf(-1)}, []float64{0, 1, 0}, 1},
		{[]float64{math.Log(1), math.Log(3)}, []float64{.25, .75}, 1.6},
	} {
		got, ess, err := normalizeWeights(tc.logs)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(tc.want) {
			t.Fatal("wrong weight count")
		}
		for i := range got {
			if math.Abs(got[i]-tc.want[i]) > 1e-14 {
				t.Fatalf("weights %v want %v", got, tc.want)
			}
		}
		if math.Abs(ess-tc.ess) > 1e-14 {
			t.Fatalf("ESS=%g want %g", ess, tc.ess)
		}
	}
}

func TestWeightsRejectNonDistributions(t *testing.T) {
	for _, logs := range [][]float64{nil, {math.Inf(-1), math.Inf(-1)}, {math.NaN()}, {math.Inf(1)}} {
		if _, _, err := normalizeWeights(logs); err == nil {
			t.Fatalf("accepted log weights %v", logs)
		}
	}
}

func TestWeightedResamplingSkipsZerosAndPreservesDuplicates(t *testing.T) {
	r := &fixedFloat{values: []float64{0, .249, .25, .999}}
	got, err := weightedIndices([]float64{0, .25, 0, .75, 0}, 4, r)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{1, 1, 3, 3}) {
		t.Fatalf("got %v", got)
	}
	for _, w := range [][]float64{nil, {0, 0}, {-1, 2}, {math.NaN(), 1}, {math.Inf(1)}} {
		if _, err := weightedIndices(w, 1, &fixedFloat{values: []float64{0}}); err == nil {
			t.Fatalf("accepted weights %v", w)
		}
	}
	if _, err := weightedIndices([]float64{1}, -1, r); err == nil {
		t.Fatal("accepted negative output count")
	}
}
