package search

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestPCRFullIsAPureCoin mirrors azmcts's: PCRFull is a pure function of
// its arguments, p >= 1 always full, p <= 0 (and NaN) never, and about p of
// the seeds full.
func TestPCRFullIsAPureCoin(t *testing.T) {
	full := 0
	for seed := uint64(0); seed < 20000; seed++ {
		if PCRFull(seed, 0.25) != PCRFull(seed, 0.25) {
			t.Fatalf("seed %d: two draws differ", seed)
		}
		if !PCRFull(seed, 1) || PCRFull(seed, 0) || PCRFull(seed, -1) || PCRFull(seed, math.NaN()) {
			t.Fatalf("seed %d: an extreme probability drew wrong", seed)
		}
		if PCRFull(seed, 0.25) {
			full++
		}
	}
	if full < 4600 || full > 5400 {
		t.Fatalf("p=0.25: %d of 20000 full", full)
	}
}

// TestPCROffLeavesTheGameUnchanged mirrors azmcts's: FullProb 0 (whatever
// CheapSims says) and FullProb 1 play the plain seat's game, search for
// search, every decision recorded.
func TestPCROffLeavesTheGameUnchanged(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 12
	plain, _ := seatGame(NewSearcher(o, Uniform{}, Heuristic{}), 3)
	for _, mut := range []func(*Options){
		func(o *Options) { o.CheapSims = 3 },
		func(o *Options) { o.FullProb, o.CheapSims = 1, 3 },
	} {
		oo := o
		mut(&oo)
		got, _ := seatGame(NewSearcher(oo, Uniform{}, Heuristic{}), 3)
		if len(got) != len(plain) {
			t.Fatalf("%+v: %d decisions, plain %d", oo, len(got), len(plain))
		}
		for i := range got {
			a, b := &got[i].res, &plain[i].res
			if !sameSearch(a, b) || a.Cheap || a.Record != b.Record || a.Weight != 1 {
				t.Fatalf("decision %d: the seat with PCR off differs", i)
			}
		}
	}
}

// TestPCRCheapSearches: at FullProb 0.5 a searched decision is cheap
// exactly where PCRFull says so -- CheapSims simulations, no root noise or
// sampling, no forced playouts, not recorded -- and full otherwise;
// FullProbByKind overrides FullProb per searched kind; the counts add up;
// SeatSearch allocates nothing.
func TestPCRCheapSearches(t *testing.T) {
	o := DefaultOptions()
	o.Sims, o.FullProb, o.CheapSims, o.ForcedK = 12, 0.5, 3, 2
	o.Noise, o.Sample = true, true
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 9, dirac.Config{})
	pol := dirac.NewRandomPolicy(4)
	var d dirac.Decision
	var st SeatState
	full, cheap := 0, 0
	for i := 0; i < 3000; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if d.Player != 0 {
			s.Apply(pol.Pick(&d))
			continue
		}
		want := !PCRFull(DecisionSeed(77, uint64(s.SeqNo())), 0.5)
		r := sr.SeatSearch(&s, &d, 77, &st)
		if r.Stats.Searched > 0 {
			if r.Cheap != want {
				t.Fatalf("decision %d: cheap %v, PCRFull says %v", i, r.Cheap, want)
			}
			if r.Cheap {
				cheap++
				if r.Stats.Simulations != 3 || r.HasTarget || r.Record || r.Stats.ForcedSearched != 0 {
					t.Fatalf("decision %d: a cheap search ran %d sims, target %v, record %v", i, r.Stats.Simulations, r.HasTarget, r.Record)
				}
			} else {
				full++
				if r.Stats.Simulations != 12 || !r.HasTarget || !r.Record {
					t.Fatalf("decision %d: a full search ran %d sims, target %v, record %v", i, r.Stats.Simulations, r.HasTarget, r.Record)
				}
			}
		}
		if sr.Opts.Sims != 12 || !sr.Opts.Noise || sr.Opts.ForcedK != 2 {
			t.Fatalf("SeatSearch left the options changed: %+v", sr.Opts)
		}
		r.Play(&s)
	}
	if full == 0 || cheap == 0 || st.Full != full || st.Cheap != cheap {
		t.Fatalf("full %d cheap %d, seat counts %d %d", full, cheap, st.Full, st.Cheap)
	}

	// FullProbByKind: priority always cheap, every other kind full
	o.FullProb, o.FullProbKinds = 1, 1<<kindPriority
	o.FullProbByKind[kindPriority] = 0
	sr = NewSearcher(o, Uniform{}, Heuristic{})
	steps, _ := seatGame(sr, 4)
	seen := [2]int{}
	for _, x := range steps {
		r := &x.res
		if r.Stats.Searched == 0 {
			continue
		}
		prio := r.Kind == kindPriority
		if r.Cheap != prio {
			t.Fatalf("kind %s: cheap %v", KindNames[r.Kind], r.Cheap)
		}
		if prio {
			seen[0]++
		} else {
			seen[1]++
		}
	}
	if seen[0] == 0 || seen[1] == 0 {
		t.Fatalf("priority %d, other %d searched", seen[0], seen[1])
	}

	// SeatSearch allocates nothing (every knob on, at a fresh root)
	rs := roots(1)
	rs[0].StepInto(&d)
	var st2 SeatState
	o.DecidedHigh, o.DecidedN, o.DecidedSims, o.DecidedWeight = 0.9, 1, 3, 0.5
	o.SimsByKind[kindAttackers] = 20
	sr = NewSearcher(o, Uniform{}, Heuristic{})
	if a := testing.AllocsPerRun(5, func() { sr.SeatSearch(rs[0], &d, 1, &st2) }); a != 0 {
		t.Fatalf("allocs per seat search = %v", a)
	}
}

// TestValidatePCR mirrors azmcts TestNewSeatValidatesPCR.
func TestValidatePCR(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*Options)
	}{
		{"negative", func(o *Options) { o.FullProb, o.CheapSims = -0.1, 10 }},
		{"above one", func(o *Options) { o.FullProb, o.CheapSims = 1.5, 10 }},
		{"NaN", func(o *Options) { o.FullProb, o.CheapSims = math.NaN(), 10 }},
		{"no cheap sims", func(o *Options) { o.FullProb = 0.5 }},
		{"kind without prob", func(o *Options) { o.CheapSims, o.FullProbKinds = 10, 1<<kindPriority }},
		{"kind above one", func(o *Options) {
			o.FullProb, o.CheapSims, o.FullProbKinds = 0.5, 10, 1<<kindBlockers
			o.FullProbByKind[kindBlockers] = 2
		}},
		{"no kind code", func(o *Options) { o.FullProb, o.CheapSims, o.FullProbKinds = 0.5, 10, 1 }},
		{"kind cheap, no cheap sims", func(o *Options) { o.FullProb, o.FullProbKinds = 1, 1<<kindTarget }},
	} {
		o := DefaultOptions()
		tc.mut(&o)
		if o.Validate() == nil {
			t.Fatalf("%s: validated", tc.name)
		}
	}
	o := DefaultOptions()
	o.FullProb, o.CheapSims, o.FullProbKinds = 0.5, 40, 1<<kindOptional|1<<kindChoose
	o.FullProbByKind[kindChoose] = 1
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	for k := 1; k < NumKinds; k++ {
		if c, ok := KindCode(KindNames[k]); !ok || c != k {
			t.Fatalf("KindCode(%q) = %d, %v", KindNames[k], c, ok)
		}
	}
	if c, ok := KindCode("trigger_optional"); !ok || c != int(kindOptional) {
		t.Fatalf("trigger_optional reads %d, %v", c, ok)
	}
	if _, ok := KindCode("multitarget"); ok {
		t.Fatalf("multitarget is no dirac kind")
	}
}
