package searchprobe

import (
	"math"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/rules"
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

func TestSummarizeWeightDiagnosticsSeparatesIsolationMass(t *testing.T) {
	logs := []float64{0, 0, math.Log(2), math.Log(6)}
	weights, _, err := normalizeWeights(logs)
	if err != nil {
		t.Fatal(err)
	}
	d := summarizeWeightDiagnostics(logs, weights, []proposalDiagnostics{
		{isolationEligible: 1, isolationSelected: 1, isolationInside: 1},
		{isolationEligible: 1, isolationInside: 1},
		{isolationEligible: 1, isolationOutside: 1},
		{isolationEligible: 1, isolationOutside: 1},
	})
	if d.Accepted != 4 || d.IsolationEligibleAttempts != 4 || d.IsolationSelectedAttempts != 1 || d.IsolationInsideAttempts != 2 || d.IsolationOutsideAttempts != 2 {
		t.Fatalf("component counts = %+v", d)
	}
	if math.Abs(d.LogWeightMin) > 1e-12 || math.Abs(d.LogWeightMedian-math.Log(2)/2) > 1e-12 || math.Abs(d.LogWeightMax-math.Log(6)) > 1e-12 {
		t.Fatalf("log-weight distribution = %+v", d)
	}
	if math.Abs(d.MaxNormalizedMass-0.6) > 1e-12 || math.Abs(d.Top4NormalizedMass-1) > 1e-12 || d.Mass50Count != 1 || d.Mass90Count != 3 {
		t.Fatalf("mass concentration = %+v", d)
	}
	if math.Abs(d.IsolationSelectedMass-0.1) > 1e-12 || math.Abs(d.IsolationOutsideMass-0.8) > 1e-12 {
		t.Fatalf("component mass = %+v", d)
	}
	if math.Abs(d.IsolationSelectedSquaredShare-(0.01/0.42)) > 1e-12 || math.Abs(d.IsolationOutsideSquaredShare-(0.40/0.42)) > 1e-12 {
		t.Fatalf("effective-sample contribution = %+v", d)
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

func TestProposalAccumulatesExactEpochWeights(t *testing.T) {
	p := &proposalState{epochs: map[epochKey]epochConstraints{
		{Player: 0}:             {Positions: []epochPosition{{Index: 0, Name: "A"}}},
		{Player: 1, Ordinal: 1}: {Deadlines: []deadlineConstraint{{Through: 1, Name: "A", Count: 2}}},
	}, observer: NewCollector(0), result: &SampleResult{}, logWeight: -math.Log(2)}
	if _, err := p.plan(rules.ShuffleContext{Player: 0, Library: []rules.ShuffleCard{{ID: 1, Name: "A"}, {ID: 2, Name: "A"}, {ID: 3, Name: "B"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.plan(rules.ShuffleContext{Player: 1, Ordinal: 1, Hand: []rules.ShuffleCard{{ID: 4, Name: "A"}}, Library: []rules.ShuffleCard{{ID: 5, Name: "A"}, {ID: 6, Name: "B"}, {ID: 7, Name: "B"}}}); err != nil {
		t.Fatal(err)
	}
	// Exhaustive physical orders: first epoch accepts 4/6, second 2/6;
	// multiplying the public toss 1/2 gives exactly 1/9.
	if math.Abs(math.Exp(p.logWeight)-1.0/9) > 1e-12 {
		t.Fatalf("combined weight = %g", math.Exp(p.logWeight))
	}
	weights, ess, err := normalizeWeights([]float64{p.logWeight, p.logWeight, p.logWeight, p.logWeight})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(weights, []float64{.25, .25, .25, .25}) || ess != 4 {
		t.Fatalf("weights=%v ESS=%g", weights, ess)
	}
}
