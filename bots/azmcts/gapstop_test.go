package azmcts

import (
	"testing"
)

// gapGame plays the seat test game (mono-red-prowess vs mono-blue-tempo, the
// clairvoyant test source) with az at seat 0 under sc, and returns the chain
// head and every searched decision's Diag.
func gapGame(t *testing.T, sc SeatConfig, maxIntents int) (string, []Diag) {
	t.Helper()
	cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", testSeed)
	sc.Source = testSeatSource
	var diags []Diag
	prev := Watch
	Watch = func(d Diag) {
		if d.Searched {
			diags = append(diags, d)
		}
	}
	defer func() { Watch = prev }()
	az, err := NewSeat(cfg.Seed^1, nil, sc)
	if err != nil {
		t.Fatal(err)
	}
	head, err := playAZ(t, cfg, az, maxIntents)
	if err != nil {
		t.Fatal(err)
	}
	return head, diags
}

// GapStop never changes the move: every searched root of the same game,
// searched in full at 100 simulations and again under GapStop at the same
// seed, plays the same root index, so the two games are one game; and the
// stop did fire, on decisions whose search then ran fewer simulations.
func TestGapStopNeverChangesTheMove(t *testing.T) {
	sc := DefaultSeatConfig()
	if sc.Search.Sims != 100 {
		t.Fatalf("DefaultSeatConfig().Search.Sims = %d, want the hosted 100", sc.Search.Sims)
	}
	fullHead, full := gapGame(t, sc, 300)
	sc.Search.StopEarly = &GapStop{Sims: sc.Search.Sims}
	gapHead, gap := gapGame(t, sc, 300)

	if len(full) == 0 {
		t.Fatal("the seat searched no decision: nothing compared")
	}
	if len(gap) != len(full) {
		t.Fatalf("%d searched decisions under GapStop, %d in full", len(gap), len(full))
	}
	stops, saved := 0, 0
	for i := range full {
		f, g := full[i], gap[i]
		if f.Seq != g.Seq || f.Kind != g.Kind || f.Candidates != g.Candidates {
			t.Fatalf("decision %d: seq/kind/candidates %d/%s/%d in full, %d/%s/%d under GapStop", i, f.Seq, f.Kind, f.Candidates, g.Seq, g.Kind, g.Candidates)
		}
		if f.Stats.Simulations != 100 || f.Stats.EarlyStops != 0 {
			t.Fatalf("seq %d: the full search ran %d simulations with %d early stops", f.Seq, f.Stats.Simulations, f.Stats.EarlyStops)
		}
		if f.Choice != g.Choice {
			t.Errorf("seq %d (%s): full search played root %d, GapStop played %d", f.Seq, f.Kind, f.Choice, g.Choice)
		}
		if g.Stats.EarlyStops > 0 {
			stops++
			saved += 100 - g.Stats.Simulations
			if g.Stats.Simulations >= 100 {
				t.Errorf("seq %d: an early stop that ran %d simulations", g.Seq, g.Stats.Simulations)
			}
		}
	}
	if stops == 0 {
		t.Fatalf("GapStop fired on none of %d searched decisions", len(gap))
	}
	if gapHead != fullHead {
		t.Fatalf("GapStop game head %s, full game head %s", gapHead, fullHead)
	}
	t.Logf("%d searched decisions, %d stopped early, %d of %d simulations saved", len(full), stops, saved, 100*len(full))
}

// countStop is a StopEarly that never stops and counts how often it is asked.
type countStop struct{ asked int }

func (c *countStop) Stop([]int, int, int) bool { c.asked++; return false }

// A sampled move or a noised root is not the full search's argmax, so the
// seat never consults StopEarly there; the control (argmax, no noise) does.
func TestGapStopOffWhenSampling(t *testing.T) {
	cases := []struct {
		name    string
		tune    func(*SeatConfig)
		consult bool
	}{
		{"argmax", func(*SeatConfig) {}, true},
		{"sample", func(sc *SeatConfig) { sc.Explore, sc.NoNoise, sc.ExploreTurns = true, true, 1000 }, false},
		{"noise", func(sc *SeatConfig) { sc.Explore, sc.ExploreTurns = true, 0 }, false},
		{"temp", func(sc *SeatConfig) { sc.Search.Temp = 1 }, false},
		{"second", func(sc *SeatConfig) { sc.Search.Second = 0.5 }, false},
	}
	// One test, no subtests: the cases share the game and fail by name.
	for _, c := range cases {
		sc := DefaultSeatConfig()
		sc.Search.Sims = 8
		c.tune(&sc)
		stop := &countStop{}
		sc.Search.StopEarly = stop
		_, diags := gapGame(t, sc, 150)
		if len(diags) == 0 {
			t.Fatalf("%s: the seat searched no decision", c.name)
		}
		if got := stop.asked > 0; got != c.consult {
			t.Errorf("%s: StopEarly asked %d times over %d searched decisions, want consulted=%v", c.name, stop.asked, len(diags), c.consult)
		}
	}
}

// GapStop stops exactly when the runner-up cannot reach the leader, and
// exactStop rebinds it to the decision's own budget.
func TestGapStopRule(t *testing.T) {
	g := &GapStop{Sims: 10}
	for _, c := range []struct {
		visits    []int
		attempted int
		want      bool
	}{
		{[]int{4, 1}, 5, false},    // gap 3, 5 left
		{[]int{6, 0}, 6, true},     // gap 6, 4 left
		{[]int{1, 5, 0}, 6, false}, // gap 4, 4 left: a tie is still reachable
		{[]int{1, 6, 0}, 7, true},  // gap 5, 3 left
		{[]int{3}, 3, false},       // one child: gap 3, 7 left
	} {
		if got := g.Stop(c.visits, c.attempted, c.attempted); got != c.want {
			t.Errorf("Stop(%v, %d) = %v, want %v", c.visits, c.attempted, got, c.want)
		}
	}
	o := Options{Sims: 40, StopEarly: g}
	exactStop(&o)
	if r, ok := o.StopEarly.(*GapStop); !ok || r.Sims != 40 {
		t.Fatalf("exactStop left %#v for a 40-sim search", o.StopEarly)
	}
}
