package neteval

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/search"
)

func TestZeroAllocsPerEvalBatch(t *testing.T) {
	n := testNet(t)
	ps := positions(8)
	states := make([]*dirac.State, len(ps))
	decs := make([]dirac.Decision, len(ps))
	dptrs := make([]*dirac.Decision, len(ps))
	for i := range ps {
		ps[i].StepInto(&decs[i])
		states[i] = ps[i]
		dptrs[i] = &decs[i]
	}
	out := make([]float32, len(ps))
	run := func() {
		n.NewWorld()
		n.ValueBatch(states, dptrs, 0, out)
	}
	run() // grow the scratch
	if a := testing.AllocsPerRun(5, run); a != 0 {
		t.Fatalf("allocs per batch of %d positions = %v", len(ps), a)
	}
}

// TestBatchOneMatchesSerial is the exactness check with the real
// evaluator: one in-flight descent reproduces the serial tree exactly.
func TestBatchOneMatchesSerial(t *testing.T) {
	n := testNet(t)
	ps := positions(8)
	base := search.DefaultOptions()
	base.Sims = 100
	// LazyPriors off: the batched path stays eager on purpose (walkBatch
	// never sets the hint), so this pins the trees' equality, not a counter.
	base.LazyPriors = false
	serial := search.NewSearcher(base, n, search.Heuristic{})
	o := base
	o.Batch = 1
	batched := search.NewSearcher(o, n, search.Heuristic{})
	checked := 0
	for i := range ps {
		a := serial.Search(ps[i], uint64(i)+1)
		b := batched.Search(ps[i], uint64(i)+1)
		if a.Stats.Searched == 0 {
			continue
		}
		checked++
		if !reflect.DeepEqual(*a, *b) {
			t.Fatalf("root %d: batch=1 differs from serial\nserial: %+v\nbatch:  %+v", i, *a, *b)
		}
	}
	if checked == 0 {
		t.Fatal("no searched root")
	}
}

// TestBatchAgreement compares the batched tree's move with the serial
// tree's at the same roots and seat seeds (the report's move-agreement
// measurement). Batch 1 is the exactness check; 4 and 8 use virtual loss.
func TestBatchAgreement(t *testing.T) {
	n := testNet(t)
	ps := positions(64)
	base := search.DefaultOptions()
	base.Sims = 100
	serial := search.NewSearcher(base, n, search.Heuristic{})
	for _, arm := range []struct {
		batch int
		vl    float64
	}{{1, 0}, {4, 1}, {8, 1}, {8, 0}} {
		o := base
		o.Batch, o.VirtualLoss = arm.batch, arm.vl
		batched := search.NewSearcher(o, n, search.Heuristic{})
		agree, total := 0, 0
		for i := range ps {
			a := serial.Search(ps[i], uint64(i)+1)
			b := batched.Search(ps[i], uint64(i)+1)
			if a.Stats.Searched == 0 {
				continue
			}
			total++
			if a.Choice == b.Choice {
				agree++
			}
		}
		t.Logf("batch=%d vl=%g: choice agreement %d/%d (%.1f%%)", arm.batch, arm.vl, agree, total, 100*float64(agree)/float64(max(1, total)))
	}
}

func BenchmarkEvalBatch(b *testing.B) {
	n := testNet(b)
	ps := positions(8)
	states := make([]*dirac.State, len(ps))
	decs := make([]dirac.Decision, len(ps))
	dptrs := make([]*dirac.Decision, len(ps))
	for i := range ps {
		ps[i].StepInto(&decs[i])
		states[i] = ps[i]
		dptrs[i] = &decs[i]
	}
	out := make([]float32, len(ps))
	// The evaluation cache would answer every position after the first
	// iteration; turn it off so this measures the forward pass.
	n.cache = nil
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n.NewWorld()
		n.ValueBatch(states, dptrs, 0, out)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(ps)), "ns/position")
}

func BenchmarkValueSerial(b *testing.B) {
	n := testNet(b)
	ps := positions(8)
	states := make([]*dirac.State, len(ps))
	decs := make([]dirac.Decision, len(ps))
	for i := range ps {
		ps[i].StepInto(&decs[i])
		states[i] = ps[i]
	}
	out := make([]float32, len(ps))
	n.cache = nil
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n.NewWorld()
		for j := range states {
			out[j] = n.Value(states[j], &decs[j], 0)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(ps)), "ns/position")
}
