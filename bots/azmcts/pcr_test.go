package azmcts

import (
	"math"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The playout-cap draw is a pure, seeded coin: the same decision seed always
// draws the same way, its frequency is p, a decision full at p is full at any
// larger p (one uniform per decision), and p >= 1 / p <= 0 are the constants.
func TestPCRFullIsAPureCoin(t *testing.T) {
	const n = 20000
	for _, p := range []float64{0.25, 0.5, 0.65} {
		full := 0
		for seq := uint64(0); seq < n; seq++ {
			seed := DecisionSeed(testSeed^1, seq)
			got := PCRFull(seed, p)
			if PCRFull(seed, p) != got {
				t.Fatalf("p %g seq %d: the draw is not a function of its seed", p, seq)
			}
			if got && !PCRFull(seed, math.Min(1, p+0.2)) {
				t.Fatalf("p %g seq %d: full at p but cheap at a larger p", p, seq)
			}
			if !PCRFull(seed, 1) || PCRFull(seed, 0) || PCRFull(seed, math.NaN()) {
				t.Fatalf("seq %d: p 1 must be full, p 0 and NaN cheap", seq)
			}
			if got {
				full++
			}
		}
		if f := float64(full) / n; math.Abs(f-p) > 0.015 {
			t.Fatalf("p %g: %d of %d full (%.3f)", p, full, n, f)
		}
	}
}

// pcrSeat is a seat for rootOptions alone (no game is played).
func pcrSeat(t *testing.T, mut func(*SeatConfig)) *Seat {
	t.Helper()
	sc := DefaultSeatConfig()
	sc.Search.Sims = 50
	sc.Source = testSeatSource
	sc.Explore = true
	sc.SimsByKind = map[decision.Kind]int{decision.KPriority: 80}
	mut(&sc)
	s, err := NewSeat(testSeed^1, nil, sc)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// legacyRootOptions is DecideSearch's option setup before playout-cap
// randomization: the reference rootOptions must reproduce with it off.
func legacyRootOptions(s *Seat, d *decision.Decision, turn int32) Options {
	opts := s.cfg.Search
	if n, ok := s.cfg.SimsByKind[d.Kind]; ok && n > 0 {
		opts.Sims = n
	}
	opts.Seed = DecisionSeed(s.seed, d.Seq)
	opts.Noise, opts.Sample = false, false
	if s.cfg.Explore {
		opts.Noise = !s.cfg.NoNoise
		opts.Sample = turn <= s.cfg.ExploreTurns
	}
	return opts
}

// rootOptions with FullProb 0 (whatever CheapSims says) or 1 is the seat's
// usual setup at every decision; at FullProb 0.5 a cheap decision differs
// from it only in CheapSims, no noise and no sampling (same seed), exactly
// where PCRFull says so; FullProbByKind overrides FullProb per searched kind.
func TestPCRRootOptions(t *testing.T) {
	kinds := []decision.Kind{decision.KPriority, decision.KAttackers}
	check := func(name string, s *Seat, wantCheap func(seq uint64, k decision.Kind) bool) (full, cheap int) {
		t.Helper()
		for seq := uint64(0); seq < 400; seq++ {
			for _, k := range kinds {
				d := &decision.Decision{Kind: k, Seq: seq}
				turn := int32(seq % 9)
				got, c := s.rootOptions(d, turn)
				want := legacyRootOptions(s, d, turn)
				if wc := wantCheap(seq, k); c != wc {
					t.Fatalf("%s: seq %d %s: cheap %v, want %v", name, seq, k, c, wc)
				}
				if c {
					cheap++
					want.Sims, want.Noise, want.Sample = s.cfg.CheapSims, false, false
				} else {
					full++
				}
				if got != want {
					t.Fatalf("%s: seq %d %s (cheap %v): options %+v, want %+v", name, seq, k, c, got, want)
				}
			}
		}
		return full, cheap
	}
	never := func(uint64, decision.Kind) bool { return false }
	check("off", pcrSeat(t, func(c *SeatConfig) { c.CheapSims = 7 }), never)
	check("p=1", pcrSeat(t, func(c *SeatConfig) { c.FullProb, c.CheapSims = 1, 7 }), never)

	half := pcrSeat(t, func(c *SeatConfig) { c.FullProb, c.CheapSims = 0.5, 7 })
	full, cheap := check("p=0.5", half, func(seq uint64, _ decision.Kind) bool {
		return !PCRFull(DecisionSeed(half.seed, seq), 0.5)
	})
	if full < 300 || cheap < 300 {
		t.Fatalf("p=0.5: %d full, %d cheap of 800", full, cheap)
	}

	byKind := pcrSeat(t, func(c *SeatConfig) {
		c.FullProb, c.CheapSims, c.FullProbByKind = 1, 7, map[string]float64{"priority": 0}
	})
	check("priority=0", byKind, func(_ uint64, k decision.Kind) bool { return k == decision.KPriority })
	mixed := pcrSeat(t, func(c *SeatConfig) {
		c.FullProb, c.CheapSims, c.FullProbByKind = 0.3, 7, map[string]float64{"attackers": 1}
	})
	check("attackers=1", mixed, func(seq uint64, k decision.Kind) bool {
		return k == decision.KPriority && !PCRFull(DecisionSeed(mixed.seed, seq), 0.3)
	})
}

// pcrGame plays one recorded game of the az seat (seat 0) under sc and
// returns its chain head, its records and its searched diags.
func pcrGame(t *testing.T, sc SeatConfig) (string, []policynet.VisitRecord, []Diag, *Seat) {
	t.Helper()
	cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", testSeed)
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
	var recs []policynet.VisitRecord
	az.SetRecorder(func(r policynet.VisitRecord) { recs = append(recs, r) })
	head, err := playAZ(t, cfg, az, 300)
	if err != nil {
		t.Fatal(err)
	}
	return head, recs, diags, az
}

// With FullProb 0 the seat is the seat without playout-cap randomization,
// whatever CheapSims says -- the same game, byte for byte, and the same
// records -- and FullProb 1 (every decision full) is too.
func TestPCROffLeavesTheGameUnchanged(t *testing.T) {
	sc := DefaultSeatConfig()
	sc.Search.Sims = 3
	sc.Source = testSeatSource
	sc.Explore = true
	want, wantRecs, _, _ := pcrGame(t, sc)
	if len(wantRecs) == 0 {
		t.Fatal("the plain seat recorded nothing")
	}
	for _, p := range []float64{0, 1} {
		pc := sc
		pc.FullProb, pc.CheapSims = p, 1
		got, recs, diags, az := pcrGame(t, pc)
		if got != want || len(recs) != len(wantRecs) {
			t.Fatalf("FullProb %g: head %s with %d records, plain head %s with %d", p, got, len(recs), want, len(wantRecs))
		}
		for i := range recs {
			if recs[i].Sequence != wantRecs[i].Sequence || recs[i].Sims != 3 {
				t.Fatalf("FullProb %g: record %d sequence %d sims %d, plain sequence %d", p, i, recs[i].Sequence, recs[i].Sims, wantRecs[i].Sequence)
			}
		}
		for _, d := range diags {
			if d.Cheap {
				t.Fatalf("FullProb %g: a cheap decision", p)
			}
		}
		full, cheap := az.PCRCounts()
		if wantFull := map[float64]int{0: 0, 1: len(diags)}[p]; full != wantFull || cheap != 0 {
			t.Fatalf("FullProb %g: PCRCounts %d full %d cheap, want %d and 0", p, full, cheap, wantFull)
		}
	}
}

// At FullProb 0.5 a cheap decision runs CheapSims and is never recorded; a
// full one runs Sims and is, exactly at the sequences PCRFull draws full; the
// counts agree with the diags, and the game replays exactly.
func TestPCRCheapDecisionsAreNotRecorded(t *testing.T) {
	sc := DefaultSeatConfig()
	sc.Search.Sims = 4
	sc.Source = testSeatSource
	sc.Explore = true
	sc.FullProb, sc.CheapSims = 0.5, 2
	head, recs, diags, az := pcrGame(t, sc)
	var full, cheap, recordable int
	for _, d := range diags {
		want := 4
		if d.Cheap {
			cheap++
			want = 2
		} else {
			full++
			if d.Stats.AllFailed == 0 && d.Refused == "" {
				recordable++
			}
		}
		if d.Stats.Simulations != want {
			t.Fatalf("a %s decision (cheap %v) ran %d simulations, want %d", d.Kind, d.Cheap, d.Stats.Simulations, want)
		}
	}
	if full == 0 || cheap == 0 {
		t.Fatalf("%d full and %d cheap searched decisions: want both", full, cheap)
	}
	if f, c := az.PCRCounts(); f != full || c != cheap {
		t.Fatalf("PCRCounts %d full %d cheap, diags %d and %d", f, c, full, cheap)
	}
	if len(recs) != recordable {
		t.Fatalf("%d records for %d recordable full decisions (%d cheap)", len(recs), recordable, cheap)
	}
	for _, r := range recs {
		if r.Sims != 4 || !PCRFull(DecisionSeed(testSeed^1, r.Sequence), 0.5) {
			t.Fatalf("record at sequence %d (sims %d) is not a full decision", r.Sequence, r.Sims)
		}
	}
	again, recs2, _, _ := pcrGame(t, sc)
	if again != head || len(recs2) != len(recs) {
		t.Fatalf("replay diverged: %s (%d records) vs %s (%d)", again, len(recs2), head, len(recs))
	}
	t.Logf("%d full, %d cheap searched decisions, %d records", full, cheap, len(recs))
}

func TestNewSeatValidatesPCR(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*SeatConfig)
		want string
	}{
		{"negative", func(c *SeatConfig) { c.FullProb, c.CheapSims = -0.1, 10 }, "FullProb"},
		{"above one", func(c *SeatConfig) { c.FullProb, c.CheapSims = 1.5, 10 }, "FullProb"},
		{"NaN", func(c *SeatConfig) { c.FullProb, c.CheapSims = math.NaN(), 10 }, "FullProb"},
		{"no cheap sims", func(c *SeatConfig) { c.FullProb = 0.5 }, "CheapSims"},
		{"kind without prob", func(c *SeatConfig) { c.CheapSims, c.FullProbByKind = 10, map[string]float64{"priority": 0.5} }, "needs FullProb"},
		{"unknown kind", func(c *SeatConfig) {
			c.FullProb, c.CheapSims, c.FullProbByKind = 0.5, 10, map[string]float64{"trigger_optional": 0.5}
		}, "not a searched kind"},
		{"kind above one", func(c *SeatConfig) {
			c.FullProb, c.CheapSims, c.FullProbByKind = 0.5, 10, map[string]float64{"blockers": 2}
		}, "blockers"},
		{"kind cheap, no cheap sims", func(c *SeatConfig) { c.FullProb, c.FullProbByKind = 1, map[string]float64{"target": 0.2} }, "CheapSims"},
	} {
		sc := DefaultSeatConfig()
		sc.Source = testSeatSource
		tc.mut(&sc)
		if _, err := NewSeat(1, nil, sc); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: NewSeat error %v, want one mentioning %q", tc.name, err, tc.want)
		}
	}
	for _, mut := range []func(*SeatConfig){
		func(c *SeatConfig) { c.CheapSims = 0 },                    // off
		func(c *SeatConfig) { c.FullProb = 1 },                     // never cheap
		func(c *SeatConfig) { c.FullProb, c.CheapSims = 0.25, 40 }, // KataGo's share
		func(c *SeatConfig) {
			c.FullProb, c.CheapSims, c.FullProbByKind = 0.5, 40, map[string]float64{"optional": 0, "multitarget": 1}
		},
	} {
		sc := DefaultSeatConfig()
		sc.Source = testSeatSource
		mut(&sc)
		if _, err := NewSeat(1, nil, sc); err != nil {
			t.Fatal(err)
		}
	}
}
