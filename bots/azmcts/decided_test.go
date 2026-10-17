package azmcts

import (
	"math"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// The decided run: a root value on the run's side extends it, one on the
// other side starts a new run of 1, and one inside the band (both bounds
// strict) ends it.
func TestDecidedRun(t *testing.T) {
	const hi = 0.9
	var d decidedState
	for i, s := range []struct {
		v    float64
		side int8
		run  int
	}{
		{0.95, 1, 1}, {0.91, 1, 2}, {0.9, 0, 0}, {0.05, -1, 1}, {0, -1, 2}, {0.1, 0, 0},
		{0.99, 1, 1}, {0.02, -1, 1}, {0.03, -1, 2}, {0.04, -1, 3}, {0.5, 0, 0}, {1, 1, 1},
	} {
		d.observe(s.v, hi)
		if d.side != s.side || d.run != s.run {
			t.Fatalf("step %d (v %g): side %d run %d, want %d %d", i, s.v, d.side, d.run, s.side, s.run)
		}
	}
}

// decided needs DecidedHigh > 0 and a run of at least DecidedN values; a
// decided search's options are the full search's with DecidedSims, no
// noise, no sampling and no forced playouts, the seed unchanged.
func TestDecidedOptions(t *testing.T) {
	s := pcrSeat(t, func(c *SeatConfig) {
		c.DecidedHigh, c.DecidedN, c.DecidedSims, c.DecidedWeight = 0.9, 3, 7, 0.1
		c.Search.ForcedK = 2
	})
	for _, tc := range []struct {
		side int8
		run  int
		want bool
	}{{0, 0, false}, {1, 2, false}, {1, 3, true}, {-1, 3, true}, {-1, 9, true}, {0, 5, false}} {
		s.dec.side, s.dec.run = tc.side, tc.run
		if got := s.decided(); got != tc.want {
			t.Fatalf("side %d run %d: decided %v, want %v", tc.side, tc.run, got, tc.want)
		}
	}
	off := pcrSeat(t, func(c *SeatConfig) { c.DecidedN = 1 })
	off.dec.side, off.dec.run = 1, 5
	if off.decided() {
		t.Fatal("DecidedHigh 0: decided")
	}
	d := &decision.Decision{Kind: decision.KPriority, Seq: 11}
	full, cheap := s.rootOptions(d, 2)
	if cheap || !full.Noise || !full.Sample || full.ForcedK != 2 {
		t.Fatalf("full options %+v", full)
	}
	got := s.decidedOptions(full)
	want := full
	want.Sims, want.Noise, want.Sample, want.ForcedK = 7, false, false, 0
	if got != want {
		t.Fatalf("decided options %+v, want %+v", got, want)
	}
}

// decidedConfig is pcrGame's seat with decided-game searches at threshold
// hi, a run of n, 2 simulations and weight w (DecidedHigh 0 with hi 0).
func decidedConfig(hi float64, n int, w float64) SeatConfig {
	sc := DefaultSeatConfig()
	sc.Search.Sims = 4
	sc.Source = testSeatSource
	sc.Explore = true
	sc.DecidedHigh, sc.DecidedN, sc.DecidedSims, sc.DecidedWeight = hi, n, 2, w
	return sc
}

// With DecidedHigh 0 the seat is the plain seat whatever the other decided
// fields say, and so is a seat whose run can never reach DecidedN: the same
// game, byte for byte, and the same records, none weighted.
func TestDecidedOffLeavesTheGameUnchanged(t *testing.T) {
	want, wantRecs, _, _ := pcrGame(t, decidedConfig(0, 0, 0))
	if len(wantRecs) == 0 {
		t.Fatal("the plain seat recorded nothing")
	}
	for _, sc := range []SeatConfig{decidedConfig(0, 1, 0.5), decidedConfig(0.55, 100000, 0.1)} {
		got, recs, diags, az := pcrGame(t, sc)
		if got != want || len(recs) != len(wantRecs) {
			t.Fatalf("DecidedHigh %g N %d: head %s with %d records, plain head %s with %d", sc.DecidedHigh, sc.DecidedN, got, len(recs), want, len(wantRecs))
		}
		for i, r := range recs {
			w := wantRecs[i]
			if r.Sequence != w.Sequence || r.Sims != 4 || r.Weight != 0 || r.RootValue != w.RootValue || r.Choice != w.Choice {
				t.Fatalf("DecidedHigh %g: record %d (seq %d sims %d weight %g) differs from the plain seat's (seq %d)", sc.DecidedHigh, i, r.Sequence, r.Sims, r.Weight, w.Sequence)
			}
		}
		for _, d := range diags {
			if d.Decided {
				t.Fatalf("DecidedHigh %g N %d: a decided search", sc.DecidedHigh, sc.DecidedN)
			}
		}
		if n, w := az.DecidedCounts(); n != 0 || w != 0 {
			t.Fatalf("DecidedCounts %d %d, want 0 0", n, w)
		}
	}
}

// Once DecidedN completed searches in a row valued the position on one side
// of the band, the seat's searches run DecidedSims and are recorded at
// DecidedWeight, until a value leaves the band: the run replayed over the
// records (every completed search is recorded here) predicts exactly which
// records are weighted. The weight never changes the game: weight 0 records
// only the other searches, weight 1 records the decided ones unweighted.
func TestDecidedSearchesAreCheapAndWeighted(t *testing.T) {
	const hi, n = 0.7, 2
	head, recs, diags, az := pcrGame(t, decidedConfig(hi, n, 0.25))
	var run decidedState
	weighted, winning := 0, 0
	for i, r := range recs {
		want := run.side != 0 && run.run >= n
		if got := r.Weight != 0; got != want {
			t.Fatalf("record %d (seq %d, root value %.3f): weighted %v, the run (side %d, %d) says %v", i, r.Sequence, r.RootValue, got, run.side, run.run, want)
		}
		switch {
		case want && (r.Weight != 0.25 || r.Sims != 2):
			t.Fatalf("record %d: decided with weight %g and %d sims, want 0.25 and 2", i, r.Weight, r.Sims)
		case !want && r.Sims != 4:
			t.Fatalf("record %d: full with %d sims, want 4", i, r.Sims)
		case want:
			weighted++
			if run.side > 0 {
				winning++
			}
		}
		run.observe(r.RootValue, hi)
	}
	decided := 0
	for _, d := range diags {
		want := 4
		if d.Decided {
			decided++
			want = 2
		}
		if d.Stats.Simulations != want {
			t.Fatalf("a %s search (decided %v) ran %d simulations, want %d", d.Kind, d.Decided, d.Stats.Simulations, want)
		}
	}
	if weighted == 0 || weighted == len(recs) {
		t.Fatalf("%d of %d records weighted: want some decided searches and some full ones", weighted, len(recs))
	}
	if dn, dw := az.DecidedCounts(); dn != decided || dn < weighted || dw < winning {
		t.Fatalf("DecidedCounts %d (%d winning), diags %d decided, records %d weighted (%d winning)", dn, dw, decided, weighted, winning)
	}
	again, recs2, _, _ := pcrGame(t, decidedConfig(hi, n, 0.25))
	if again != head || len(recs2) != len(recs) {
		t.Fatalf("replay diverged: %s (%d records) vs %s (%d)", again, len(recs2), head, len(recs))
	}
	h0, recs0, _, _ := pcrGame(t, decidedConfig(hi, n, 0))
	h1, recs1, _, _ := pcrGame(t, decidedConfig(hi, n, 1))
	if h0 != head || h1 != head {
		t.Fatalf("the record weight changed the game: heads %s / %s / %s", h0, head, h1)
	}
	if len(recs0) != len(recs)-weighted || len(recs1) != len(recs) {
		t.Fatalf("weight 0 kept %d and weight 1 %d of %d records (%d decided)", len(recs0), len(recs1), len(recs), weighted)
	}
	for _, r := range append(recs0, recs1...) {
		if r.Weight != 0 {
			t.Fatalf("weight 0 or 1: a record with weight %g", r.Weight)
		}
	}
	t.Logf("%d records, %d decided (%d winning), %d decided searches", len(recs), weighted, winning, decided)
}

// Under playout-cap randomization a cheap decision stays cheap when the game
// is decided; only full ones become decided searches.
func TestDecidedWithPCR(t *testing.T) {
	sc := decidedConfig(0.7, 2, 0.25)
	sc.FullProb, sc.CheapSims = 0.5, 1
	_, recs, diags, az := pcrGame(t, sc)
	var cheap, decided int
	for _, d := range diags {
		want := 4
		switch {
		case d.Cheap && d.Decided:
			t.Fatal("a cheap decided search")
		case d.Cheap:
			cheap++
			want = 1
		case d.Decided:
			decided++
			want = 2
		}
		if d.Stats.Simulations != want {
			t.Fatalf("a %s search (cheap %v, decided %v) ran %d simulations, want %d", d.Kind, d.Cheap, d.Decided, d.Stats.Simulations, want)
		}
	}
	if cheap == 0 || decided == 0 {
		t.Fatalf("%d cheap and %d decided searches: want both", cheap, decided)
	}
	if f, c := az.PCRCounts(); c != cheap || f != len(diags)-cheap {
		t.Fatalf("PCRCounts %d full %d cheap; diags %d searched, %d cheap", f, c, len(diags), cheap)
	}
	for _, r := range recs {
		if r.Sims == 1 {
			t.Fatal("a cheap search was recorded")
		}
	}
}

func TestNewSeatValidatesDecided(t *testing.T) {
	dec := func(hi float64, n, sims int, w float64) func(*SeatConfig) {
		return func(c *SeatConfig) { c.DecidedHigh, c.DecidedN, c.DecidedSims, c.DecidedWeight = hi, n, sims, w }
	}
	for _, tc := range []struct {
		name string
		mut  func(*SeatConfig)
		want string
	}{
		{"half", dec(0.5, 3, 40, 0), "DecidedHigh"},
		{"one", dec(1, 3, 40, 0), "DecidedHigh"},
		{"below half", dec(0.3, 3, 40, 0), "DecidedHigh"},
		{"negative", dec(-0.9, 3, 40, 0), "DecidedHigh"},
		{"NaN", dec(math.NaN(), 3, 40, 0), "DecidedHigh"},
		{"no run", dec(0.9, 0, 40, 0), "DecidedN"},
		{"no sims", dec(0.9, 3, 0, 0), "DecidedSims"},
		{"weight above one", dec(0.9, 3, 40, 1.5), "DecidedWeight"},
		{"negative weight", dec(0.9, 3, 40, -0.1), "DecidedWeight"},
		{"NaN weight", dec(0.9, 3, 40, math.NaN()), "DecidedWeight"},
	} {
		sc := DefaultSeatConfig()
		sc.Source = testSeatSource
		tc.mut(&sc)
		if _, err := NewSeat(1, nil, sc); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: NewSeat error %v, want one mentioning %q", tc.name, err, tc.want)
		}
	}
	for _, mut := range []func(*SeatConfig){
		dec(0, -1, -1, 7), // off: the rest ignored
		dec(0.9, 3, 40, 0.1),
		dec(0.51, 1, 1, 0), // weight 0: decided searches unrecorded
		dec(0.99, 9, 400, 1),
	} {
		sc := DefaultSeatConfig()
		sc.Source = testSeatSource
		mut(&sc)
		if _, err := NewSeat(1, nil, sc); err != nil {
			t.Fatal(err)
		}
	}
}
