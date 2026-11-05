package searchtest

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/neteval"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/search"
)

// feat_trace_test.go records the evaluator calls real searches make (FDN
// and vanilla) and replays them against a fresh evaluator: a repeatable
// measure of the featurization cost per evaluation (build with -tags kprof
// for neteval's feat/net split).

type traceEv struct {
	kind uint8 // 0 SetBase, 1 NewWorld, 2 Value, 3 Priors
	st   int32 // index into states
	seat uint8
	d    dirac.Decision
	bot  []bool
}

type tracer struct {
	inner  *neteval.Net
	evs    []traceEv
	states []*dirac.State
	last   *dirac.State
	max    int
}

func (t *tracer) state(s *dirac.State) int32 {
	n := len(t.states)
	if n > 0 && t.last == s {
		if t.states[n-1].SameGame(s) {
			return int32(n - 1)
		}
	}
	c := new(dirac.State)
	c.CopyFrom(s)
	t.states = append(t.states, c)
	t.last = s
	return int32(len(t.states) - 1)
}

func (t *tracer) full() bool { return len(t.evs) >= t.max }

func (t *tracer) HasPrior() bool { return true }
func (t *tracer) NewWorld() {
	if !t.full() {
		t.evs = append(t.evs, traceEv{kind: 1})
	}
	t.inner.NewWorld()
}
func (t *tracer) SetBase(s *dirac.State) {
	if !t.full() {
		t.evs = append(t.evs, traceEv{kind: 0, st: t.state(s)})
	}
	t.inner.SetBase(s)
}
func (t *tracer) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	if !t.full() {
		t.evs = append(t.evs, traceEv{kind: 2, st: t.state(s), seat: seat, d: *d})
	}
	return t.inner.Value(s, d, seat)
}
func (t *tracer) Priors(s *dirac.State, d *dirac.Decision, bot []bool, out []float32) {
	if !t.full() {
		t.evs = append(t.evs, traceEv{kind: 3, st: t.state(s), d: *d, bot: append([]bool(nil), bot...)})
	}
	t.inner.Priors(s, d, bot, out)
}

// traceSet is one matchup's recorded calls and its pool.
type traceSet struct {
	pool *feat.Pool
	tr   *tracer
}

func recordFDNTraces(t testing.TB, w *neteval.Weights, pairs, perPair int) []traceSet {
	c, pool := testCorpus(t)
	opts := search.DefaultOptions()
	var out []traceSet
	for _, p := range oracle.PairingsFrom(pool, 0, pairs, 41) {
		m, tbl, err := oracle.SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		fp, err := oracle.FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		side, err := oracle.RealKernel(tbl, nil, oracle.Config{PassElision: true})(m, p.Seed)
		if err != nil {
			continue
		}
		ks, kd, _ := oracle.KernelStateOf(side)
		tr := &tracer{inner: neteval.New(w, fp), max: perPair}
		sr := search.NewSearcher(opts, tr, search.Heuristic{})
		func() {
			defer func() { recover() }()
			for i := 0; i < 400 && !tr.full(); i++ {
				ks.StepInto(kd)
				if kd.Kind == dirac.DkNone {
					return
				}
				res := sr.Search(ks, p.Seed*31+uint64(i))
				for j := 0; j < res.NActs; j++ {
					ks.Apply(res.Acts[j])
				}
			}
		}()
		out = append(out, traceSet{pool: fp, tr: tr})
	}
	return out
}

// replayTraces replays every set through one fresh evaluator (SetPool per
// set) and returns the wall time and the evaluations.
func replayTraces(w *neteval.Weights, sets []traceSet) (time.Duration, int) {
	n := neteval.New(w, sets[0].pool)
	var out [dirac.MaxOptions]float32
	evals := 0
	t0 := time.Now()
	for _, ts := range sets {
		n.SetPool(ts.pool)
		tr := ts.tr
		for i := range tr.evs {
			ev := &tr.evs[i]
			switch ev.kind {
			case 0:
				n.SetBase(tr.states[ev.st])
			case 1:
				n.NewWorld()
			case 2:
				n.Value(tr.states[ev.st], &ev.d, ev.seat)
				evals++
			case 3:
				n.Priors(tr.states[ev.st], &ev.d, ev.bot, out[:ev.d.N])
				evals++
			}
		}
	}
	return time.Since(t0), evals
}

// TestFeatTraceReplay records FDN search traces and replays them, logging
// the featurization cost (feat ns per evaluation call with -tags kprof).
func TestFeatTraceReplay(t *testing.T) {
	t.Parallel()

	if os.Getenv("KFEAT_TRACE") == "" {
		t.Skip("set KFEAT_TRACE=1 (a measurement, not a check)")
	}
	_ = os.Getenv("KFEAT_RUNID") // a fresh value defeats the test cache
	net := netPath(t)
	w, err := neteval.LoadWeights(net)
	if err != nil {
		t.Fatal(err)
	}
	sets := recordFDNTraces(t, w, 10, 6000)
	evs := 0
	for _, s := range sets {
		evs += len(s.tr.evs)
	}
	replayTraces(w, sets) // warm
	best := time.Duration(1 << 62)
	var bf, bn int64
	var calls int
	reps := 3
	if v := os.Getenv("KFEAT_REPS"); v != "" {
		fmt.Sscan(v, &reps)
	}
	for r := 0; r < reps; r++ {
		neteval.ProfReset()
		d, n := replayTraces(w, sets)
		f, nn := neteval.ProfNanos()
		if d < best {
			best, bf, bn, calls = d, f, nn, n
		}
	}
	t.Logf("%d sets, %d events, %d evaluation calls: %.0f ns/call wall, feat %.0f ns/call, net %.0f ns/call",
		len(sets), evs, calls, float64(best.Nanoseconds())/float64(calls), float64(bf)/float64(calls), float64(bn)/float64(calls))
}

// TestFeatTraceParts replays the traces through a bare encoder (no network,
// no cache): the cost of the keys, the parts and the options per call.
func TestFeatTraceParts(t *testing.T) {
	t.Parallel()

	if os.Getenv("KFEAT_TRACE") == "" {
		t.Skip("set KFEAT_TRACE=1 (a measurement, not a check)")
	}
	_ = os.Getenv("KFEAT_RUNID") // a fresh value defeats the test cache
	net := netPath(t)
	w, err := neteval.LoadWeights(net)
	if err != nil {
		t.Fatal(err)
	}
	sets := recordFDNTraces(t, w, 10, 6000)
	enc := feat.NewEncoder(sets[0].pool)
	enc.Reserve()
	var base [dirac.MaxObj]uint64
	var ohc dirac.ObjHashCache
	var opts []feat.Option = make([]feat.Option, dirac.MaxOptions)
	var tk, tp, to time.Duration
	calls := 0
	hot := os.Getenv("KFEAT_COLD") == ""
	var hs dirac.State
	var sink uint64
	reps := 5
	if v := os.Getenv("KFEAT_REPS"); v != "" {
		fmt.Sscan(v, &reps)
	}
	for r := 0; r < reps; r++ {
		for _, ts := range sets {
			enc.SetPool(ts.pool)
			tr := ts.tr
			var baseEpoch uint32
			for i := range tr.evs {
				ev := &tr.evs[i]
				s := tr.states[ev.st]
				switch ev.kind {
				case 0:
					for o := 0; o < s.NumObjs(); o++ {
						base[o] = s.ObjHash(dirac.Obj(o))
					}
					baseEpoch = s.Epoch()
					enc.SetBase(s)
					continue
				case 1:
					continue
				}
				if hot {
					// a search encodes the State it just stepped: copy it into
					// one buffer so its lines are warm, as they are there
					hs.CopyFrom(s)
					s = &hs
				}
				var bp *[dirac.MaxObj]uint64
				if e := s.Epoch(); e != 0 && e == baseEpoch {
					bp = &base
				}
				seat := ev.seat
				if ev.kind == 3 {
					seat = ev.d.Player
				}
				t0 := time.Now()
				sink += s.ViewHashCached(seat, bp, &ohc)
				if ev.kind == 3 {
					sink += s.DecisionHash(&ev.d)
				}
				t1 := time.Now()
				enc.EncodeParts(s, &ev.d, seat)
				t2 := time.Now()
				if ev.kind == 3 {
					opts = enc.EncodeOptions(&ev.d, opts[:0])
				}
				t3 := time.Now()
				tk += t1.Sub(t0)
				tp += t2.Sub(t1)
				to += t3.Sub(t2)
				calls++
			}
		}
	}
	_ = sink
	per := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / float64(calls) }
	t.Logf("%d calls: keys %.0f ns, parts %.0f ns, options %.0f ns per call (timer overhead included)", calls, per(tk), per(tp), per(to))
}

// TestFeatTraceKeys times the content keys alone over the traces.
func TestFeatTraceKeys(t *testing.T) {
	t.Parallel()

	if os.Getenv("KFEAT_TRACE") == "" {
		t.Skip("set KFEAT_TRACE=1 (a measurement, not a check)")
	}
	_ = os.Getenv("KFEAT_RUNID")
	net := netPath(t)
	w, err := neteval.LoadWeights(net)
	if err != nil {
		t.Fatal(err)
	}
	sets := recordFDNTraces(t, w, 10, 6000)
	var base [dirac.MaxObj]uint64
	var ohc dirac.ObjHashCache
	var sink uint64
	calls := 0
	t0 := time.Now()
	var tv, td time.Duration
	for r := 0; r < 20; r++ {
		for _, ts := range sets {
			tr := ts.tr
			var baseEpoch uint32
			for i := range tr.evs {
				ev := &tr.evs[i]
				s := tr.states[ev.st]
				switch ev.kind {
				case 0:
					for o := 0; o < s.NumObjs(); o++ {
						base[o] = s.ObjHash(dirac.Obj(o))
					}
					baseEpoch = s.Epoch()
					continue
				case 1:
					continue
				}
				var bp *[dirac.MaxObj]uint64
				if e := s.Epoch(); e != 0 && e == baseEpoch {
					bp = &base
				}
				seat := ev.seat
				if ev.kind == 3 {
					seat = ev.d.Player
				}
				a := time.Now()
				sink += s.ViewHashCached(seat, bp, &ohc)
				b := time.Now()
				if ev.kind == 3 {
					sink += s.DecisionHash(&ev.d)
				}
				tv += b.Sub(a)
				td += time.Since(b)
				calls++
			}
		}
	}
	_ = sink
	t.Logf("%d calls: %.0f ns/call (view %.0f, decision %.0f, with timers)", calls, float64(time.Since(t0).Nanoseconds())/float64(calls),
		float64(tv.Nanoseconds())/float64(calls), float64(td.Nanoseconds())/float64(calls))
}
