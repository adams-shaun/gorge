package gonet

// The graph network's parity test: the Go forward must reproduce GraphNet's
// option logits and value logit on real encoded states. The reference is dumped
// by draft-zero's `python -m dzg.gnnref` from a pack, along with the encoded
// states themselves, so this test reads one JSON and needs no .npy parser.
//
// The committed fixture is a TINY GraphNet (d=16, heads=2, ff=32, d_emb=4,
// passes=2, global_layers=2; ~400 KB) with every weight randomised, so the
// zero-initialised heads are live and every path carries signal. Regenerate it
// from draft-zero's gorge/ directory:
//
//	PYTHONPATH=. python <gorge>/bots/gonet/testdata/gnn/mktiny.py /tmp/tiny.pt
//	python -m dzg.export /tmp/tiny.pt <gorge>/bots/gonet/testdata/gnn/gnn.dzgw
//	python -m dzg.gnnref /tmp/tiny.pt <pack dir> <gorge>/bots/gonet/testdata/gnn/ref.json --n 6 --device cpu
//
// GORGE_GNN_NET and GORGE_GNN_REF point the test at a real checkpoint's export
// and dump instead (a trained network exported at this build's RAW_W).

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
)

type gnnRefFile struct {
	Count  int `json:"count"`
	States []struct {
		Record int          `json:"record"`
		Dense  []float32    `json:"dense"`
		Sparse []gnnRefFeat `json:"sparse"`
		Cards  []struct {
			Group int          `json:"group"`
			Raw   []float32    `json:"raw"`
			Rows  []gnnRefFeat `json:"rows"`
		} `json:"cards"`
		Options []struct {
			Slots  []gnnRefFeat `json:"slots"`
			Hashed []gnnRefFeat `json:"hashed"`
			Dense  []float32    `json:"dense"`
			Bot    int          `json:"bot"`
			EA     int          `json:"ea"`
			EB     int          `json:"eb"`
		} `json:"options"`
	} `json:"states"`
	Records []struct {
		Record     int       `json:"record"`
		NOpt       int       `json:"nopt"`
		Scores     []float32 `json:"scores"`
		ValueLogit float32   `json:"value_logit"`
	} `json:"records"`
}

type gnnRefFeat struct {
	Row   int     `json:"row"`
	Value float32 `json:"value"`
}

func refFeatures(f []gnnRefFeat) []policynet.Feature {
	out := make([]policynet.Feature, len(f))
	for i, x := range f {
		out[i] = policynet.Feature{Row: uint16(x.Row), Value: x.Value}
	}
	return out
}

const gnnParityTol = 1e-3

type gnnCase struct {
	record int
	state  policynet.State
	opts   []policynet.Option
	scores []float32 // python's option logits
	value  float32   // python's value logit
}

// loadGNNRef loads the network and the reference cases (GORGE_GNN_NET /
// GORGE_GNN_REF, defaulting to the committed tiny fixture).
func loadGNNRef(tb testing.TB) (*Net, []gnnCase) {
	tb.Helper()
	netPath := os.Getenv("GORGE_GNN_NET")
	if netPath == "" {
		netPath = "testdata/gnn/gnn.dzgw"
	}
	refPath := os.Getenv("GORGE_GNN_REF")
	if refPath == "" {
		refPath = "testdata/gnn/ref.json"
	}
	raw, err := os.ReadFile(refPath)
	if err != nil {
		tb.Fatalf("reference dump: %v", err)
	}
	var ref gnnRefFile
	if err := json.Unmarshal(raw, &ref); err != nil {
		tb.Fatalf("%s: %v", refPath, err)
	}
	if len(ref.States) == 0 || len(ref.States) != len(ref.Records) {
		tb.Fatalf("%s: %d states, %d record results", refPath, len(ref.States), len(ref.Records))
	}
	g, err := Load(netPath)
	if err != nil {
		tb.Fatalf("%s: %v", netPath, err)
	}
	if g.gnn == nil {
		tb.Fatalf("%s: not a graph network", netPath)
	}
	var cases []gnnCase
	for si, st := range ref.States {
		rec := ref.Records[si]
		if rec.Record != st.Record || len(st.Options) != rec.NOpt {
			tb.Fatalf("state %d: record %d with %d options, results for record %d with %d", si, st.Record, len(st.Options), rec.Record, rec.NOpt)
		}
		state := policynet.State{Dense: st.Dense, Sparse: refFeatures(st.Sparse)}
		for _, c := range st.Cards {
			state.Cards = append(state.Cards, policynet.EntityCard{
				Group: uint8(c.Group), Raw: c.Raw, Rows: refFeatures(c.Rows)})
		}
		opts := make([]policynet.Option, len(st.Options))
		for i, o := range st.Options {
			// the dump's ea/eb are 0-based card indexes, -1 for none; EntA/EntB are 1-based, 0 for none
			opts[i] = policynet.Option{
				Slots: refFeatures(o.Slots), Hashed: refFeatures(o.Hashed), Dense: o.Dense,
				BotPick: o.Bot > 0, EntA: int32(max(o.EA, -1) + 1), EntB: int32(max(o.EB, -1) + 1),
			}
		}
		cases = append(cases, gnnCase{record: st.Record, state: state, opts: opts, scores: rec.Scores, value: rec.ValueLogit})
	}
	return g, cases
}

// TestGNNParity is the contract for the graph forward: every option logit and
// the value logit within 1e-3 of PyTorch's, on real encoded states, through
// the SIMD kernels and through their scalar twins (ForceScalar).
func TestGNNParity(t *testing.T) {
	for _, scalar := range []bool{false, true} {
		name := "simd"
		if scalar {
			name = "scalar"
		}
		t.Run(name, func(t *testing.T) {
			ForceScalar = scalar
			defer func() { ForceScalar = false }()
			gnnParity(t)
		})
	}
}

func gnnParity(t *testing.T) {
	g, cases := loadGNNRef(t)
	var worstScore, worstValue float64
	optsChecked := 0
	for _, c := range cases {
		scores := make([]float32, len(c.opts))
		value := g.Eval(&c.state, c.opts, scores)
		for i := range scores {
			d := math.Abs(float64(scores[i] - c.scores[i]))
			worstScore = max(worstScore, d)
			optsChecked++
			if d > gnnParityTol {
				t.Errorf("record %d option %d: Go %.6f, python %.6f (|d| %.3g)", c.record, i, scores[i], c.scores[i], d)
			}
		}
		// the logit, not the sigmoid: a saturated sigmoid would hide a trunk error
		dv := math.Abs(float64(value - c.value))
		worstValue = max(worstValue, dv)
		if dv > gnnParityTol {
			t.Errorf("record %d: value logit Go %.6f, python %.6f (|d| %.3g)", c.record, value, c.value, dv)
		}
		// the cached policynet.External path answers exactly what Eval does
		if got := g.Score(c.state, c.opts); !slices.Equal(got, scores) {
			t.Errorf("record %d: Score %v, Eval %v", c.record, got, scores)
		}
		if got := g.Value(c.state); got != Sigmoid32(value) {
			t.Errorf("record %d: Value %v, Eval %v", c.record, got, Sigmoid32(value))
		}
	}
	// a state with no cards, no sparse rows and no options still has a value
	if v := g.Value(policynet.State{Dense: cases[0].state.Dense}); !(v > 0 && v < 1) {
		t.Errorf("empty state: value %v", v)
	}
	t.Logf("%d states, %d option logits: worst |d| score %.3g, value logit %.3g (tol %g)", len(cases), optsChecked, worstScore, worstValue, gnnParityTol)
}

// TestGNNEvalBatchMatchesEval holds the batched forward to the single-state
// one: every state's value and option logits, at B = 1, 3 and 8 over a mix
// of card counts (a state with no cards, states with no options), equal
// Eval's within 1e-5, and in practice bit for bit.
func TestGNNEvalBatchMatchesEval(t *testing.T) {
	for _, scalar := range []bool{false, true} {
		ForceScalar = scalar
		g, cases := loadGNNRef(t)
		var pool []gnnCase
		for i, c := range cases {
			pool = append(pool, c)
			// a value-only twin of each, and (once) a state with no cards
			pool = append(pool, gnnCase{state: c.state})
			if i == 0 {
				st := c.state
				st.Cards = nil
				pool = append(pool, gnnCase{state: st, opts: dropEntities(c.opts)})
			}
		}
		worst, exact, total := 0.0, 0, 0
		for _, B := range []int{1, 3, 8} {
			for start := 0; start < len(pool); start += B {
				sts := make([]*policynet.State, B)
				opts := make([][]policynet.Option, B)
				scores := make([][]float32, B)
				values := make([]float32, B)
				for i := range sts {
					c := pool[(start+i)%len(pool)]
					st := c.state
					sts[i], opts[i], scores[i] = &st, c.opts, make([]float32, len(c.opts))
				}
				g.EvalBatch(sts, opts, scores, values)
				for i := range sts {
					want := make([]float32, len(opts[i]))
					wv := g.Eval(sts[i], opts[i], want)
					cmp := func(what string, got, w float32) {
						total++
						if got == w {
							exact++
						}
						d := math.Abs(float64(got - w))
						worst = max(worst, d)
						if d > 1e-5 {
							t.Errorf("scalar=%v B=%d state %d %s: batch %v, Eval %v", scalar, B, i, what, got, w)
						}
					}
					cmp("value", values[i], wv)
					for j := range want {
						cmp("score", scores[i][j], want[j])
					}
				}
			}
		}
		t.Logf("scalar=%v: %d/%d outputs bit-identical, worst |d| %.3g", scalar, exact, total, worst)
	}
	ForceScalar = false
}

// dropEntities clears the options' card references (for a state without cards).
func dropEntities(opts []policynet.Option) []policynet.Option {
	out := slices.Clone(opts)
	for i := range out {
		out[i].EntA, out[i].EntB = 0, 0
	}
	return out
}

// BenchmarkGNNEvalBatch is one uncached batched forward over B states (the
// reference states reused cyclically), reported per state.
func BenchmarkGNNEvalBatch(b *testing.B) {
	g, cases := loadGNNRef(b)
	for _, B := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("B%d", B), func(b *testing.B) {
			sts := make([]*policynet.State, B)
			opts := make([][]policynet.Option, B)
			scores := make([][]float32, B)
			values := make([]float32, B)
			for i := range sts {
				c := cases[i%len(cases)]
				st := c.state
				sts[i], opts[i], scores[i] = &st, c.opts, make([]float32, len(c.opts))
			}
			b.ReportAllocs()
			for b.Loop() {
				g.EvalBatch(sts, opts, scores, values)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*B)/1e6, "ms/state")
		})
	}
}

// BenchmarkGNNEval is one uncached forward (trunk and every option) on the
// first reference state.
func BenchmarkGNNEval(b *testing.B) {
	g, cases := loadGNNRef(b)
	c := cases[0]
	scores := make([]float32, len(c.opts))
	b.ReportAllocs()
	for b.Loop() {
		g.Eval(&c.state, c.opts, scores)
	}
}
