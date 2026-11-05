package search

import (
	"math"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// probeEval values like Uniform but records the batch it is handed and
// checks the searcher's tree for virtual-loss marks while the values are
// pending.
type probeEval struct {
	Uniform
	sr          *Searcher
	calls       int
	maxN        int
	sawInFlight bool
}

func (p *probeEval) ValueBatch(states []*dirac.State, decs []*dirac.Decision, seat uint8, out []float32) {
	p.calls++
	if len(out) > p.maxN {
		p.maxN = len(out)
	}
	if p.sr != nil {
		for i := 0; i < int(p.sr.t.ne); i++ {
			if p.sr.t.edges[i].inFlight > 0 {
				p.sawInFlight = true
				break
			}
		}
	}
	for i := range out {
		out[i] = p.Value(states[i], decs[i], seat)
	}
}

func TestBatchTerminatesAndStats(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 160
	o.Batch = 4
	o.VirtualLoss = 1
	pe := &probeEval{}
	sr := NewSearcher(o, pe, Heuristic{})
	pe.sr = sr
	rs := roots(8)
	searched := 0
	for i := range rs {
		r := sr.Search(rs[i], 7)
		if r.Stats.Searched == 0 {
			continue
		}
		searched++
		if r.Stats.Simulations != o.Sims {
			t.Fatalf("root %d: simulations %d, want %d", i, r.Stats.Simulations, o.Sims)
		}
		if r.Stats.Completed <= 0 || r.Stats.Completed > r.Stats.Simulations {
			t.Fatalf("root %d: completed %d of %d", i, r.Stats.Completed, r.Stats.Simulations)
		}
	}
	if searched == 0 {
		t.Fatal("no searched root")
	}
	if pe.calls == 0 {
		t.Fatal("batch evaluator was never called")
	}
	if pe.maxN < 2 {
		t.Fatalf("largest batch %d, want > 1 (batching did not happen)", pe.maxN)
	}
	if !pe.sawInFlight {
		t.Fatal("no edge was marked in flight during a batch")
	}
}

func TestBatchZeroVirtualLoss(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 120
	o.Batch = 4
	o.VirtualLoss = 0
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	rs := roots(8)
	searched := 0
	for i := range rs {
		r := sr.Search(rs[i], 7)
		if r.Stats.Searched == 0 {
			continue
		}
		searched++
		if r.Stats.Simulations != o.Sims || r.Stats.Completed <= 0 || r.Stats.Completed > r.Stats.Simulations {
			t.Fatalf("root %d: simulations %d, completed %d", i, r.Stats.Simulations, r.Stats.Completed)
		}
	}
	if searched == 0 {
		t.Fatal("no searched root")
	}
}

func TestBatchOneMatchesSerial(t *testing.T) {
	rs := roots(20)
	base := DefaultOptions()
	base.Sims = 120
	// LazyPriors off: the batched path stays eager on purpose (walkBatch
	// never sets the hint), so this pins the trees' equality, not a counter.
	base.LazyPriors = false
	serial := NewSearcher(base, HeuristicLeaf{}, Heuristic{})
	bo := base
	bo.Batch = 1
	batched := NewSearcher(bo, HeuristicLeaf{}, Heuristic{})
	checked := 0
	for i := range rs {
		a := serial.Search(rs[i], uint64(i)+1)
		b := batched.Search(rs[i], uint64(i)+1)
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

func TestVirtualLossSelection(t *testing.T) {
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	tr := &sr.t
	tr.reset()
	var pt point
	pt.n = 2
	pt.keys[0], pt.keys[1] = 1, 2
	pt.prior[0], pt.prior[1] = 0.5, 0.5
	tr.newNode(&pt)
	o := DefaultOptions()
	if got := tr.selectEdgeVL(0, &pt, &o, 1); got != 0 {
		t.Fatalf("no in-flight edge: selected %d, want 0 (first on a tie)", got)
	}
	tr.edges[0].inFlight = 1
	if got := tr.selectEdgeVL(0, &pt, &o, 1); got != 1 {
		t.Fatalf("in-flight first edge: selected %d, want 1 (virtual loss)", got)
	}
	if got := tr.selectEdgeVL(0, &pt, &o, 0); got != 0 {
		t.Fatalf("virtual loss 0: selected %d, want 0 (no penalty)", got)
	}
}

func TestBatchZeroAllocs(t *testing.T) {
	o := DefaultOptions()
	o.Batch = 4
	o.VirtualLoss = 1
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 11, dirac.Config{})
	pol := dirac.NewRandomPolicy(5)
	var d dirac.Decision
	for i := 0; i < 60; i++ {
		s.StepInto(&d)
		s.Apply(pol.Pick(&d))
	}
	for {
		s.StepInto(&d)
		if d.Player == 0 && d.Kind == dirac.DkPriority {
			break
		}
		s.Apply(pol.Pick(&d))
	}
	allocs := testing.AllocsPerRun(5, func() { sr.Search(&s, 1) })
	if allocs != 0 {
		t.Fatalf("allocs per batched search = %v", allocs)
	}
}

func TestBatchValidate(t *testing.T) {
	valid := func(f func(*Options)) error {
		o := DefaultOptions()
		f(&o)
		return o.Validate()
	}
	if err := valid(func(o *Options) { o.Batch = 0 }); err != nil {
		t.Fatalf("batch 0: %v", err)
	}
	if err := valid(func(o *Options) { o.Batch = MaxBatch; o.VirtualLoss = 2 }); err != nil {
		t.Fatalf("batch %d: %v", MaxBatch, err)
	}
	for name, f := range map[string]func(*Options){
		"batch<0":     func(o *Options) { o.Batch = -1 },
		"batch>max":   func(o *Options) { o.Batch = MaxBatch + 1 },
		"vl<0":        func(o *Options) { o.Batch = 2; o.VirtualLoss = -1 },
		"vl NaN":      func(o *Options) { o.Batch = 2; o.VirtualLoss = math.NaN() },
		"vl inf":      func(o *Options) { o.Batch = 2; o.VirtualLoss = math.Inf(1) },
		"vl no batch": func(o *Options) { o.VirtualLoss = 1 },
		"node cache":  func(o *Options) { o.Batch = 2; o.NodeCache = 8 },
		"early gap":   func(o *Options) { o.Batch = 2; o.EarlyCutGap = true },
		"early share": func(o *Options) { o.Batch = 2; o.EarlyCutShare = 0.5 },
	} {
		if err := valid(f); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
}
