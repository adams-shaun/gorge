package search

import (
	"math"
	"testing"
)

func TestRootPick(t *testing.T) {
	res := &Result{N: 4}
	res.Visits = [MaxCands]int32{60, 30, 5, 1}
	res.Q = [MaxCands]float64{0.50, 0.55, 0.90, 0.99}
	o := &Options{RootQ: true}
	if got := rootPick(res, o, 0, false); got != 1 { // minN = 6: child 2 (5 visits) ineligible
		t.Fatalf("rootq f=0.1: got %d, want 1", got)
	}
	o.RootQFrac = 0.05 // minN = 3: child 2 eligible, child 3 not
	if got := rootPick(res, o, 0, false); got != 2 {
		t.Fatalf("rootq f=0.05: got %d, want 2", got)
	}
	o = &Options{RootLCB: true, RootQFrac: 0.05}
	// lcb: 0.50-1.96*sqrt(.25/60)=.373, 0.55-1.96*sqrt(.2475/30)=.372, 0.90-1.96*sqrt(.09/5)=.637
	if got := rootPick(res, o, 0, false); got != 2 {
		t.Fatalf("rootlcb: got %d, want 2", got)
	}
	res.Q[2] = 0.6
	if got := rootPick(res, o, 0, false); got != 0 {
		t.Fatalf("rootlcb low-n: got %d, want 0", got)
	}
	// a NaN or non-positive frac or z is the default (a host refuses them;
	// a NaN floor would admit every child, the unvisited ones too)
	res.Q[2] = 0.90
	for _, f := range []float64{math.NaN(), -1} {
		o = &Options{RootQ: true, RootQFrac: f}
		if got := rootPick(res, o, 0, false); got != 1 {
			t.Fatalf("rootq f=%v: got %d, want 1 (the f=0.1 pick)", f, got)
		}
		o = &Options{RootLCB: true, RootQFrac: 0.05, RootLCBZ: f}
		if got := rootPick(res, o, 0, false); got != 2 {
			t.Fatalf("rootlcb z=%v: got %d, want 2 (the z=1.96 pick)", f, got)
		}
	}
}

// TestRootPickLandRoot: at a LandSearch land root (the land candidate 0)
// rootPick keeps landDraw's rule -- a line other than the land stands only
// with strictly more visits than the land -- so it never defers a drop the
// argmax plays.
func TestRootPickLandRoot(t *testing.T) {
	res := &Result{N: 4}
	// the land is the argmax; child 2 has the best Q with fewer visits
	res.Visits = [MaxCands]int32{40, 30, 20, 10}
	res.Q = [MaxCands]float64{0.50, 0.55, 0.80, 0.40}
	o := &Options{RootQ: true}
	if got := rootPick(res, o, 0, false); got != 2 {
		t.Fatalf("not a land root: got %d, want 2", got)
	}
	if got := rootPick(res, o, 0, true); got != 0 {
		t.Fatalf("land root, land the argmax: got %d, want the land 0", got)
	}
	// the argmax (child 1) defers the drop: Q may still pick among the
	// land and the lines above it, never child 2 (fewer visits than the land)
	res.Visits = [MaxCands]int32{30, 40, 20, 35}
	res.Q = [MaxCands]float64{0.50, 0.45, 0.90, 0.60}
	if got := rootPick(res, o, 1, true); got != 3 {
		t.Fatalf("land root, deferred: got %d, want 3", got)
	}
	res.Q[3] = 0.40
	if got := rootPick(res, o, 1, true); got != 0 {
		t.Fatalf("land root, the land's Q best: got %d, want the land 0", got)
	}
	o = &Options{RootLCB: true}
	res.Q = [MaxCands]float64{0.50, 0.45, 0.95, 0.40}
	if got := rootPick(res, o, 1, true); got != 0 {
		t.Fatalf("land root rootlcb: got %d, want the land 0", got)
	}
}
