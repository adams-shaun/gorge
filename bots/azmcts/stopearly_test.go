package azmcts

import (
	"context"
	"reflect"
	"testing"
)

type stopFn struct{ f func([]int, int, int) bool }

func (s *stopFn) Stop(v []int, c, a int) bool { return s.f(v, c, a) }

func stopFunc(f func([]int, int, int) bool) EarlyStopper { return &stopFn{f} }

// StopEarly is asked after every simulation with the root visits so far and
// ends the loop when it says so, counted in EarlyStops; nil runs every
// simulation with the same visits as before.
func TestStopEarly(t *testing.T) {
	g := &fakeGame{nodes: map[string]fakeNode{
		"":   {keys: []Key{"a", "b"}, prior: []float64{0.5, 0.5}, value: 0.5},
		"/a": {value: 0.9, terminal: true},
		"/b": {value: 0.1, terminal: true},
	}}
	run := func(stop func([]int, int, int) bool) (TreeResult, Stats) {
		opts := treeOpts(20)
		if stop != nil {
			opts.StopEarly = stopFunc(stop)
		}
		var st Stats
		tr, err := RunTree(context.Background(), rootOf(g), &fakeSource{g: g}, opts, &st)
		if err != nil {
			t.Fatal(err)
		}
		return tr, st
	}
	full, fst := run(nil)
	if fst.EarlyStops != 0 || fst.Simulations != 20 {
		t.Fatalf("nil hook: %d early stops, %d sims", fst.EarlyStops, fst.Simulations)
	}
	var calls []int
	tr, st := run(func(v []int, completed, attempted int) bool {
		calls = append(calls, attempted)
		if completed != attempted {
			t.Errorf("completed %d != attempted %d", completed, attempted)
		}
		sum := 0
		for _, n := range v {
			sum += n
		}
		if sum != completed {
			t.Errorf("visits %v sum %d, want %d", v, sum, completed)
		}
		return attempted == 5
	})
	if st.EarlyStops != 1 || st.Simulations != 5 || len(calls) != 5 {
		t.Fatalf("early stops %d sims %d calls %v", st.EarlyStops, st.Simulations, calls)
	}
	if tr.Visits[0]+tr.Visits[1] != 5 {
		t.Errorf("visits %v after 5 sims", tr.Visits)
	}
	never, nst := run(func([]int, int, int) bool { return false })
	if nst.EarlyStops != 0 || !reflect.DeepEqual(never.Visits, full.Visits) {
		t.Errorf("a hook that never stops changed the run: %v vs %v (%d stops)", never.Visits, full.Visits, nst.EarlyStops)
	}
	// The last simulation is never an early stop.
	_, lst := run(func(_ []int, _, attempted int) bool { return attempted == 20 })
	if lst.EarlyStops != 0 {
		t.Errorf("the final simulation counted as an early stop")
	}
}
