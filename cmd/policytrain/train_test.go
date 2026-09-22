package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// syntheticCorpus builds a tiny labelled corpus with a learnable rule the
// model must overfit: the teacher's preferred option always carries the
// shared "syn|preferred" hashed marker row (plus every option its own
// identity row and every state a unique identity row, so memorisation is
// also available), and the value targets separate the preferred option
// (0.9) from the rest (0.3). Both losses have signal: the value term pulls
// the preferred option's score up, the ranking term (margin 0.3) pushes it
// above the other labelled options in the softmax.
func syntheticCorpus(n int) []policynet.Example {
	rng := rand.New(rand.NewPCG(42, 1))
	out := make([]policynet.Example, n)
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		for d := range st.Dense {
			if d < 4 {
				st.Dense[d] = float32(rng.NormFloat64())
			}
		}
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("syn|state|%d", i)), Value: 1},
			policynet.Feature{Row: policynet.HashID("syn|noiseshared"), Value: 0.5},
		)
		ex := policynet.Example{
			Pair:          "synthetic",
			GameIndex:     i,
			Seed:          uint64(1000 + i),
			Sequence:      uint64(i),
			Kind:          "choose",
			Turn:          int32(5 + i%20),
			Margin:        0.1,
			TeacherChoice: 1,
			State:         st,
		}
		pref := i % 3
		for j := 0; j < 3; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			o.Hashed = append(o.Hashed,
				policynet.Feature{Row: policynet.HashID(fmt.Sprintf("syn|pick|%d", j)), Value: 1})
			if j == pref {
				o.Hashed = append(o.Hashed,
					policynet.Feature{Row: policynet.HashID("syn|preferred"), Value: 1})
				o.Target = policynet.OptionTarget{Labelled: true, Preferred: true, Value: 0.9}
			} else {
				o.Target = policynet.OptionTarget{Labelled: true, Value: 0.1}
			}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

func testConfig(seed int64) Config {
	return Config{
		Epochs: 400, Batch: 16, LR: 0.5, Seed: seed, Holdout: 0.125,
		Embed: 16, Hidden: 64, RankWeight: 1.0, HuberDelta: 0.25,
	}
}

// TestTrainerOverfitsTinyCorpus is the brief's overfit gate: on the tiny
// synthetic corpus the loss must come DOWN and the top-1 agreement with the
// teacher must go UP — decisively, on both the training split and the
// holdout (whose preferred options share the marker feature, so the fit
// generalises rather than memorises). Also measures the wall time and
// throughput the report must quote.
//
// The loss gate is 2x, not "collapse to zero", because THIS loss has an
// analytic floor above zero: the ranking softmax wants unbounded separation
// while the value term pins the preferred/other score gap at 0.8, and at
// margin 0.1 their optimum is value ≈ 0.004 + rank ≈ 0.056 ≈ 0.06 (measured
// run reaches 0.06–0.08). Top-1 saturation (1.000 by epoch 2 on both
// splits) is the unambiguous overfit signal; the loss factor pins that the
// optimizer actually descends rather than lucking into the argmax.
func TestTrainerOverfitsTinyCorpus(t *testing.T) {
	corpus := syntheticCorpus(96)
	cfg := testConfig(1)
	wallStart := time.Now()
	res, err := Train(corpus, cfg)
	wall := time.Since(wallStart)
	if err != nil {
		t.Fatalf("Train: %v", err)
	}
	if res.TrainN+res.HoldoutN != len(corpus) || res.HoldoutN != 12 {
		t.Fatalf("split: train %d + holdout %d, want 96 total with holdout 12", res.TrainN, res.HoldoutN)
	}
	first, final := res.Epochs[0], res.Epochs[len(res.Epochs)-1]
	if final.TrainLoss >= first.TrainLoss {
		t.Fatalf("train loss did not fall: %.6f -> %.6f", first.TrainLoss, final.TrainLoss)
	}
	if final.TrainLoss > 0.5*first.TrainLoss {
		t.Fatalf("train loss only %.1fx better (%.6f -> %.6f)", first.TrainLoss/final.TrainLoss, first.TrainLoss, final.TrainLoss)
	}
	if final.TrainTop1 < 0.95 {
		t.Fatalf("train top-1 %.3f < 0.95", final.TrainTop1)
	}
	if final.TrainTop1 <= first.TrainTop1 {
		t.Fatalf("train top-1 did not rise: %.3f -> %.3f", first.TrainTop1, final.TrainTop1)
	}
	if final.HoldoutTop1 < 0.9 {
		t.Fatalf("holdout top-1 %.3f < 0.9", final.HoldoutTop1)
	}
	if final.HoldoutLoss >= first.HoldoutLoss {
		t.Fatalf("holdout loss did not fall: %.6f -> %.6f", first.HoldoutLoss, final.HoldoutLoss)
	}
	throughput := float64(cfg.Epochs*res.TrainN) / wall.Seconds()
	t.Logf("overfit: train loss %.6f -> %.6f, train top1 %.3f -> %.3f, holdout top1 %.3f; wall %s, %.0f examples/s",
		first.TrainLoss, final.TrainLoss, first.TrainTop1, final.TrainTop1, final.HoldoutTop1, wall, throughput)
}

// TestTrainerDeterministic is the brief's determinism gate: the same seed
// and the same corpus produce a BYTE-IDENTICAL checkpoint across two runs
// (and identical epoch statistics).
func TestTrainerDeterministic(t *testing.T) {
	corpus := syntheticCorpus(96)
	cfg := testConfig(2)

	resA, err := Train(corpus, cfg)
	if err != nil {
		t.Fatalf("Train A: %v", err)
	}
	resB, err := Train(corpus, cfg)
	if err != nil {
		t.Fatalf("Train B: %v", err)
	}
	if len(resA.Epochs) != len(resB.Epochs) {
		t.Fatalf("epoch counts differ: %d vs %d", len(resA.Epochs), len(resB.Epochs))
	}
	for i := range resA.Epochs {
		if resA.Epochs[i] != resB.Epochs[i] {
			t.Fatalf("epoch %d statistics differ: %+v vs %+v", i+1, resA.Epochs[i], resB.Epochs[i])
		}
	}
	var bufA, bufB bytes.Buffer
	if err := policynet.WriteCheckpoint(resA.Model, &bufA); err != nil {
		t.Fatalf("write A: %v", err)
	}
	if err := policynet.WriteCheckpoint(resB.Model, &bufB); err != nil {
		t.Fatalf("write B: %v", err)
	}
	if !bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Fatalf("checkpoints differ: %d vs %d bytes — the trainer is not reproducible from its seed", bufA.Len(), bufB.Len())
	}

	// A DIFFERENT seed must move something (the seed is load-bearing).
	resC, err := Train(corpus, func() Config { c := testConfig(3); c.Epochs = 3; return c }())
	if err != nil {
		t.Fatalf("Train C: %v", err)
	}
	var bufC bytes.Buffer
	if err := policynet.WriteCheckpoint(resC.Model, &bufC); err != nil {
		t.Fatalf("write C: %v", err)
	}
	if bytes.Equal(bufA.Bytes()[:2048], bufC.Bytes()[:2048]) {
		// Only the first 2 KiB compared (early weights): a different seed at
		// epoch 3 has almost surely diverged in the table by then.
		t.Fatal("a different seed produced identical early weights — the seed is not reaching the run")
	}
}

// TestTrainerRejectsBadConfig pins the config validation.
func TestTrainerRejectsBadConfig(t *testing.T) {
	corpus := syntheticCorpus(8)
	for _, c := range []struct {
		name  string
		nilEx bool
		mut   func(*Config)
	}{
		{"empty corpus", true, func(c *Config) {}},
		{"zero epochs", false, func(c *Config) { c.Epochs = 0 }},
		{"zero batch", false, func(c *Config) { c.Batch = 0 }},
		{"zero lr", false, func(c *Config) { c.LR = 0 }},
		{"holdout 1", false, func(c *Config) { c.Holdout = 1 }},
		{"negative holdout", false, func(c *Config) { c.Holdout = -0.1 }},
	} {
		cfg := testConfig(1)
		c.mut(&cfg)
		in := corpus
		if c.nilEx {
			in = nil
		}
		if _, err := Train(in, cfg); err == nil {
			t.Fatalf("%s: accepted", c.name)
		}
	}
	// An all-unlabelled corpus is empty for training purposes.
	unlabelled := syntheticCorpus(4)
	for i := range unlabelled {
		for j := range unlabelled[i].Options {
			unlabelled[i].Options[j].Target = policynet.OptionTarget{}
		}
	}
	if _, err := Train(unlabelled, testConfig(1)); err == nil {
		t.Fatal("all-unlabelled corpus accepted")
	}
}

// realisticCorpus builds examples at the production feature density
// (R1 §3.3's nnzS=200 state rows, 4 options × ~12 active entries) so the
// throughput benchmark measures the shape a real corpus has.
func realisticCorpus(n int) []policynet.Example {
	rng := rand.New(rand.NewPCG(77, 77+1))
	out := make([]policynet.Example, n)
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		for d := range st.Dense {
			st.Dense[d] = float32(rng.NormFloat64()) * 0.3
		}
		for k := 0; k < 200; k++ {
			st.Sparse = append(st.Sparse, policynet.Feature{
				Row:   uint16(rng.IntN(policynet.TableRows)),
				Value: 1,
			})
		}
		ex := policynet.Example{Kind: "choose", Margin: 0.3, State: st}
		pref := i % 4
		for j := 0; j < 4; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			for k := 0; k < 10; k++ {
				o.Slots = append(o.Slots, policynet.Feature{Row: uint16(rng.IntN(policynet.OptionSlotWidth)), Value: 1})
			}
			o.Hashed = append(o.Hashed, policynet.Feature{Row: uint16(rng.IntN(policynet.TableRows)), Value: 1})
			o.Target = policynet.OptionTarget{Labelled: true, Preferred: j == pref, Value: 0.9 - 0.15*float64(j)}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

// BenchmarkLossGradStep measures one batch's forward+backward+apply at the
// production geometry (H=128, hidden=128, TableRows, batch 1024, nnzS=200,
// 4 options × 11 active) — the single-threaded SGD step R1 §3.3 priced at
// 34.33 ms/step (29,828 rows/s) on this box. Run explicitly:
//
//	go test ./cmd/policytrain -run XXX -bench BenchmarkLossGradStep -benchtime 20x
func BenchmarkLossGradStep(b *testing.B) {
	corpus := realisticCorpus(1024)
	m := policynet.NewModel(policynet.TableRows, 128, 128, rand.New(rand.NewPCG(5, 6)))
	g := m.NewGrads()
	lc := policynet.DefaultLossConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Reset()
		var loss float64
		for j := range corpus {
			loss += m.LossGrad(corpus[j], lc, g).Loss
		}
		m.ApplyGrads(g, 0.1/1024)
		if loss == 0 {
			b.Fatal("zero loss")
		}
	}
}

// corpusRecord mirrors the loader's expected label record shape (the writer
// is package main over in cmd/searchteacher, so the JSON is spelled out
// here — field names must match cmd/searchteacher/labels.go's tags).
type corpusRecord struct {
	RecordType    string          `json:"record_type"`
	SchemaVersion int             `json:"schema_version"`
	Pair          string          `json:"pair"`
	GameIndex     int             `json:"game_index"`
	Seed          uint64          `json:"seed"`
	Sequence      uint64          `json:"decision_sequence"`
	Seat          int             `json:"seat"`
	Kind          string          `json:"kind"`
	Turn          int             `json:"turn"`
	View          json.RawMessage `json:"view"`
	Options       []struct {
		Index int    `json:"index"`
		Kind  string `json:"kind"`
		Label string `json:"label"`
	} `json:"options"`
	Candidates []struct {
		Choices []int   `json:"choices"`
		Bot     bool    `json:"bot"`
		Value   float64 `json:"value"`
		Worlds  int     `json:"worlds"`
	} `json:"candidates"`
	TeacherChoice int     `json:"teacher_choice"`
	BotIndex      int     `json:"bot_index"`
	Margin        float64 `json:"margin"`
	Worlds        int     `json:"worlds"`
}

// writeJSONLCorpus writes n records in the loader's shape: every record
// offers three options, the teacher's candidate picks the preferred one
// (cycling i%3), the bot's candidate picks option 0.
func writeJSONLCorpus(t *testing.T, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "labels.jsonl")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i := 0; i < n; i++ {
		r := corpusRecord{
			RecordType: "label-v1", SchemaVersion: 1,
			Pair: "syn-vs-syn", GameIndex: i, Seed: uint64(i), Sequence: uint64(i),
			Seat: 0, Kind: "choose", Turn: 5,
			View: json.RawMessage(`{"viewer":0}`),
		}
		for j := 0; j < 3; j++ {
			r.Options = append(r.Options, struct {
				Index int    `json:"index"`
				Kind  string `json:"kind"`
				Label string `json:"label"`
			}{j, "cast", fmt.Sprintf("opt%d", j)})
		}
		pref := i % 3
		r.Candidates = append(r.Candidates,
			struct {
				Choices []int   `json:"choices"`
				Bot     bool    `json:"bot"`
				Value   float64 `json:"value"`
				Worlds  int     `json:"worlds"`
			}{[]int{0}, true, 0.3, 16},
			struct {
				Choices []int   `json:"choices"`
				Bot     bool    `json:"bot"`
				Value   float64 `json:"value"`
				Worlds  int     `json:"worlds"`
			}{[]int{pref}, false, 0.9, 16},
		)
		r.TeacherChoice, r.BotIndex, r.Margin, r.Worlds = 1, 0, 0.6, 16
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestEndToEndRunsTheCLIShape drives run() exactly as main does: flags →
// corpus load → train → atomic checkpoint → exit 0, and the written
// checkpoint loads back through the policynet loader with the pinned
// encoder hash. This is the glue the unit tests above cannot see.
func TestEndToEndRunsTheCLIShape(t *testing.T) {
	path := writeJSONLCorpus(t, 48)
	out := filepath.Join(t.TempDir(), "model.bin")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-corpus", path, "-out", out,
		"-epochs", "5", "-embed", "8", "-hidden", "8", "-batch", "8",
		"-seed", "9", "-holdout", "0.2",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "epoch 5/5") || !strings.Contains(stdout.String(), "48 records") {
		t.Fatalf("stdout missing expected lines:\n%s", stdout.String())
	}
	m, err := policynet.LoadCheckpointFile(out)
	if err != nil {
		t.Fatalf("checkpoint written by the CLI does not load: %v", err)
	}
	if m.H != 8 || m.Hidden != 8 {
		t.Fatalf("checkpoint geometry h %d hidden %d, want 8/8", m.H, m.Hidden)
	}
	// Missing/extra args exit non-zero without touching the filesystem.
	if code := run([]string{"-corpus", path}, &stdout, &stderr); code == 0 {
		t.Fatal("run without -out accepted")
	}
	if code := run([]string{"-corpus", filepath.Join(t.TempDir(), "nope.jsonl"), "-out", out}, &stdout, &stderr); code == 0 {
		t.Fatal("run with a missing corpus accepted")
	}
}

// gridCorpus builds a compact synthetic corpus with real value signal (a
// per-option value spread) and an option-discriminating feature, enough to
// make the rank term and the value term both nonzero. Used by the numerical
// stability gate.
func gridCorpus(n, nopts int) []policynet.Example {
	out := make([]policynet.Example, n)
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		st.Dense[0] = float32(i%5) * 0.2
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("g|s|%d", i%7)), Value: 1},
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("g|b|%d", i%5)), Value: 1},
		)
		ex := policynet.Example{Kind: "attackers", Margin: 0.2, TeacherChoice: 1, BotIndex: 0, State: st}
		pref := i % nopts
		for j := 0; j < nopts; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			o.Hashed = append(o.Hashed, policynet.Feature{Row: policynet.HashID(fmt.Sprintf("g|p|%d", j)), Value: 1})
			v := 0.5 + 0.05*float64(j)
			if j == pref {
				o.Hashed = append(o.Hashed, policynet.Feature{Row: policynet.HashID("g|good"), Value: 1})
				v = 0.75
			}
			o.Target = policynet.OptionTarget{Labelled: true, Preferred: j == pref, Value: v}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

// TestTrainerStableAcrossRankGrid is the numerical-stability gate the brief
// names: every rank-weight/lr combination in the reported grid trains to a
// finite loss and a finite checkpoint. The pre-fix loss is NaN at the top of
// the grid (rank-weight 50) because its log-sum-exp saw raw ±Inf scores and
// computed Inf − Inf; the score clamp plus the max-subtracting log-sum-exp
// keep every value finite.
func TestTrainerStableAcrossRankGrid(t *testing.T) {
	corpus := gridCorpus(200, 3)
	for _, rw := range []float64{5, 10, 25, 50} {
		for _, lr := range []float64{0.1, 0.01, 0.001} {
			cfg := Config{Epochs: 6, Batch: 32, LR: lr, Seed: 7, Holdout: 0.15,
				Embed: 16, Hidden: 16, RankWeight: rw, HuberDelta: 0.1}
			res, err := Train(corpus, cfg)
			if err != nil {
				t.Fatalf("rw=%v lr=%v: Train: %v", rw, lr, err)
			}
			for _, e := range res.Epochs {
				if math.IsNaN(e.TrainLoss) || math.IsInf(e.TrainLoss, 0) ||
					math.IsNaN(e.HoldoutLoss) || math.IsInf(e.HoldoutLoss, 0) ||
					math.IsNaN(e.TrainTop1) || math.IsNaN(e.HoldoutTop1) {
					t.Fatalf("rw=%v lr=%v epoch %d: non-finite stat %+v", rw, lr, e.Epoch, e)
				}
			}
			params := [][]float32{res.Model.Table, res.Model.StateW, res.Model.StateB,
				res.Model.HidW, res.Model.HidB, res.Model.OutW}
			for name, p := range params {
				for i, v := range p {
					if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
						t.Fatalf("rw=%v lr=%v: non-finite parameter block %d [%d] = %v", rw, lr, name, i, v)
					}
				}
			}
			if math.IsNaN(float64(res.Model.OutB)) || math.IsInf(float64(res.Model.OutB), 0) {
				t.Fatalf("rw=%v lr=%v: non-finite output bias %v", rw, lr, res.Model.OutB)
			}
		}
	}
}

// TestTrainerLearnsOverridesOverBotImitation is the collapse regression gate.
// The corpus: the teacher keeps the bot's answer (option 0) on 90% of
// decisions and overrides to an option-feature-marked option on the other
// 10%; the override decisions carry a tiny margin, so an unweighted trainer
// drowns their rank signal and learns to copy the bot. The correct option is
// determined by the option feature "bi|good" (gated by the "bi|ov" state
// cue), and the first-labelled baseline is 0.90 because option 0 is first.
//
// Measured: unweighted (the pre-fix behaviour) holdout top-1 = 0.867, BELOW
// the 0.900 first-labelled baseline — the collapse signature. With
// -override-weight 1000 (the new signal weighting) it reaches 1.000, clearly
// above the baseline. The chosen 1000 compensates the 200x margin ratio this
// synthetic uses to stand in for the real corpus's 75%/25% agreement/override
// split.
func TestTrainerLearnsOverridesOverBotImitation(t *testing.T) {
	corpus := botImitationTrainCorpus(600, 3, 0.9, 0.2, 0.001)

	unweighted, err := Train(corpus, Config{Epochs: 30, Batch: 32, LR: 0.05, Seed: 7,
		Holdout: 0.15, Embed: 32, Hidden: 64, RankWeight: 5, HuberDelta: 0.1, OverrideWeight: 1})
	if err != nil {
		t.Fatalf("Train (unweighted): %v", err)
	}
	weighted, err := Train(corpus, Config{Epochs: 30, Batch: 32, LR: 0.05, Seed: 7,
		Holdout: 0.15, Embed: 32, Hidden: 64, RankWeight: 5, HuberDelta: 0.1, OverrideWeight: 1000})
	if err != nil {
		t.Fatalf("Train (weighted): %v", err)
	}
	u := unweighted.ByKind[0]
	w := weighted.ByKind[0]
	if w.Kind != u.Kind {
		t.Fatalf("kind mismatch: %q vs %q", w.Kind, u.Kind)
	}
	if u.FirstTop1 < 0.85 || u.FirstTop1 > 0.95 {
		t.Fatalf("first-labelled baseline %.3f outside the expected ~0.90", u.FirstTop1)
	}
	if u.ModelTop1 > u.FirstTop1 {
		t.Fatalf("unweighted model top-1 %.3f should be at/below the first-labelled baseline %.3f (bot imitation)",
			u.ModelTop1, u.FirstTop1)
	}
	if w.ModelTop1 <= w.FirstTop1 {
		t.Fatalf("weighted model top-1 %.3f not above the first-labelled baseline %.3f", w.ModelTop1, w.FirstTop1)
	}
	if w.ModelTop1 < 0.95 {
		t.Fatalf("weighted model top-1 %.3f < 0.95", w.ModelTop1)
	}
	t.Logf("first-labelled %.3f; unweighted %.3f; override-weighted %.3f", w.FirstTop1, u.ModelTop1, w.ModelTop1)
}

// botImitationTrainCorpus builds the bot-imitation corpus the regression gate
// trains on (see TestTrainerLearnsOverridesOverBotImitation).
func botImitationTrainCorpus(n, nopts int, keepFrac, keepMargin, ovMargin float64) []policynet.Example {
	out := make([]policynet.Example, n)
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		st.Sparse = append(st.Sparse, policynet.Feature{Row: policynet.HashID(fmt.Sprintf("bi|s|%d", i%7)), Value: 1})
		ex := policynet.Example{Kind: "attackers", Margin: keepMargin, BotIndex: 0, State: st}
		override := float64(i)/float64(n) >= keepFrac
		pref := 0
		if override {
			pref = 1
			ex.TeacherChoice = 1
			ex.Margin = ovMargin
			st.Sparse = append(st.Sparse, policynet.Feature{Row: policynet.HashID("bi|ov"), Value: 1})
			ex.State = st
		}
		for j := 0; j < nopts; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			o.Hashed = append(o.Hashed, policynet.Feature{Row: policynet.HashID(fmt.Sprintf("bi|p|%d", j)), Value: 1})
			if j == 1 {
				o.Hashed = append(o.Hashed, policynet.Feature{Row: policynet.HashID("bi|good"), Value: 1})
			}
			o.Target = policynet.OptionTarget{Labelled: true, Preferred: j == pref, Value: 0.5}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

// argmaxCECorpus is the L9b-fix2 gate corpus: the correct option is
// determined ENTIRELY by the option feature "ce|good" (every third decision's
// option 2 carries it, and the teacher always overrides to it), the labelled
// values are ALL TIED at 0.5 (the value term is skipped — no within-decision
// preference signal at all), and the override margin is 0.001, a hundredth of
// ONE sampled world (the measured real-corpus scale: median within-decision
// value spread 0.062 = 1/16, dominated by rollout noise). Under the pre-fix
// default loss (hybrid: margin-weighted rank + value) the rank signal is
// scaled by that 0.001 margin and the trainer ends BELOW the first-option
// baseline; under pure argmax CE (weight RankWeight, margin not applied) the
// feature is learned to saturation.
func argmaxCECorpus(n, nopts int, margin float64) []policynet.Example {
	out := make([]policynet.Example, n)
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("ce|s|%d", i%7)), Value: 1})
		marked := i%3 == 0
		pref := 0
		if marked {
			pref = 2
		}
		ex := policynet.Example{Kind: "attackers", TeacherChoice: pref, BotIndex: 0, State: st}
		if marked {
			ex.Margin = margin
		}
		for j := 0; j < nopts; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			o.Hashed = append(o.Hashed,
				policynet.Feature{Row: policynet.HashID(fmt.Sprintf("ce|p|%d", j)), Value: 1})
			if marked && j == 2 {
				o.Hashed = append(o.Hashed,
					policynet.Feature{Row: policynet.HashID("ce|good"), Value: 1})
			}
			o.Target = policynet.OptionTarget{Labelled: true, Preferred: j == pref, Value: 0.5}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

// TestTrainerArgmaxCEBeatsValueRegression is the brief's gate: on the corpus
// above, the pure-CE default must reach holdout per-kind top-1 CLEARLY above
// the first-option baseline while the pre-fix default loss (the zero Mode =
// hybrid arm, exactly today's pre-change default) stays at or below it.
//
// Measured at seed 7 (deterministic): first-labelled baseline 0.600,
// hybrid 0.167, value-only 0.167, pure CE 1.000. The hybrid arm FAILS ON
// TODAY'S DEFAULT LOSS — the assertion on it is the part that could not even
// be expressed before the mode existed, and it is the collapse reproduced in
// miniature: the margin-weighted rank term is silenced by a one-world margin
// and the value term (tied values here) carries nothing.
func TestTrainerArgmaxCEBeatsValueRegression(t *testing.T) {
	corpus := argmaxCECorpus(600, 3, 0.001)
	run := func(mode policynet.LossMode) KindStat {
		t.Helper()
		cfg := Config{Epochs: 10, Batch: 32, LR: 0.1, Seed: 7, Holdout: 0.15,
			Embed: 32, Hidden: 64, Mode: mode, RankWeight: 1, HuberDelta: 0.1}
		res, err := Train(corpus, cfg)
		if err != nil {
			t.Fatalf("mode %q: Train: %v", mode, err)
		}
		if len(res.ByKind) != 1 {
			t.Fatalf("mode %q: %d kinds, want 1", mode, len(res.ByKind))
		}
		return res.ByKind[0]
	}

	ce := run(policynet.LossCE)
	if ce.FirstTop1 < 0.5 || ce.FirstTop1 > 0.7 {
		t.Fatalf("first-option baseline %.3f outside the expected ~0.667", ce.FirstTop1)
	}
	if ce.ModelTop1 < 0.95 {
		t.Fatalf("pure-CE top-1 %.3f not clearly above the first-option baseline %.3f (want >= 0.95)",
			ce.ModelTop1, ce.FirstTop1)
	}
	hybrid := run("") // the pre-fix default geometry, byte-equal to LossHybrid
	if hybrid.ModelTop1 > hybrid.FirstTop1 {
		t.Fatalf("pre-fix default loss top-1 %.3f beat the baseline %.3f — the gate corpus no longer reproduces the collapse",
			hybrid.ModelTop1, hybrid.FirstTop1)
	}
	if ce.ModelTop1 < hybrid.ModelTop1+0.3 {
		t.Fatalf("pure CE %.3f not clearly above the pre-fix default %.3f", ce.ModelTop1, hybrid.ModelTop1)
	}
	t.Logf("first-option %.3f | pre-fix default (hybrid) %.3f | value-only %.3f | pure CE %.3f",
		hybrid.FirstTop1, hybrid.ModelTop1, run(policynet.LossValue).ModelTop1, ce.ModelTop1)
}

// TestTrainerCENoNaNGrid is the brief's stability grid under the NEW default
// loss (pure CE): every rank-weight {0,5,20,50} × lr {0.001,0.01,0.1}
// combination trains to a finite loss, finite per-epoch statistics and a
// finite checkpoint, with the global gradient clip active (the CLI default).
// rank-weight 0 makes the CE term identically zero — the degenerate corner —
// and must still come out finite and (trivially) NaN-free.
func TestTrainerCENoNaNGrid(t *testing.T) {
	corpus := gridCorpus(200, 3)
	for _, rw := range []float64{0, 5, 20, 50} {
		for _, lr := range []float64{0.001, 0.01, 0.1} {
			cfg := Config{Epochs: 6, Batch: 32, LR: lr, Seed: 7, Holdout: 0.15,
				Embed: 16, Hidden: 16, Mode: policynet.LossCE, RankWeight: rw, HuberDelta: 0.1, Clip: 1}
			res, err := Train(corpus, cfg)
			if err != nil {
				t.Fatalf("rw=%v lr=%v: Train: %v", rw, lr, err)
			}
			for _, e := range res.Epochs {
				if math.IsNaN(e.TrainLoss) || math.IsInf(e.TrainLoss, 0) ||
					math.IsNaN(e.HoldoutLoss) || math.IsInf(e.HoldoutLoss, 0) ||
					math.IsNaN(e.TrainTop1) || math.IsNaN(e.HoldoutTop1) {
					t.Fatalf("rw=%v lr=%v epoch %d: non-finite stat %+v", rw, lr, e.Epoch, e)
				}
			}
			params := [][]float32{res.Model.Table, res.Model.StateW, res.Model.StateB,
				res.Model.HidW, res.Model.HidB, res.Model.OutW}
			for name, p := range params {
				for i, v := range p {
					if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
						t.Fatalf("rw=%v lr=%v: non-finite parameter block %d [%d] = %v", rw, lr, name, i, v)
					}
				}
			}
			if math.IsNaN(float64(res.Model.OutB)) || math.IsInf(float64(res.Model.OutB), 0) {
				t.Fatalf("rw=%v lr=%v: non-finite output bias %v", rw, lr, res.Model.OutB)
			}
			if rw == 0 {
				// Degenerate corner: zero CE weight ⇒ zero loss, zero movement.
				if res.Epochs[0].TrainLoss != 0 {
					t.Fatalf("rw=0: train loss %g, want identically 0", res.Epochs[0].TrainLoss)
				}
			}
		}
	}
}

// TestTrainerResidualPriorDoesNotBlockLearningOverrides is the L9b-fix2
// item-3 gate at the trainer level: the residual prior is a fixed +ResidualInit
// on the BOT's option (here the WRONG one on the feature-decided decisions),
// and CE must still learn the feature-determined override on top of it. A
// prior that blocked learning would cap this at the bot baseline; the
// measured top-1 is ~1.000.
func TestTrainerResidualPriorDoesNotBlockLearningOverrides(t *testing.T) {
	corpus := argmaxCECorpus(600, 3, 0.001)
	bot := 0
	for i := range corpus {
		corpus[i].Options[bot].BotPick = true // the bot's answer is always option 0
	}
	res, err := Train(corpus, Config{Epochs: 8, Batch: 32, LR: 0.1, Seed: 7, Holdout: 0.15,
		Embed: 32, Hidden: 64, Mode: policynet.LossCE, RankWeight: 1, HuberDelta: 0.1, ResidualInit: 5})
	if err != nil {
		t.Fatalf("Train: %v", err)
	}
	k := res.ByKind[0]
	if res.Model.ResidualW != 5 {
		t.Fatalf("ResidualW = %g, want the configured prior 5", res.Model.ResidualW)
	}
	if k.ModelTop1 < 0.95 {
		t.Fatalf("model top-1 %.3f: the residual prior blocked the feature-determined override (want >= 0.95)", k.ModelTop1)
	}
	t.Logf("residual prior 5 on the wrong option: model %.3f, first %.3f, bot %.3f", k.ModelTop1, k.FirstTop1, k.BotTop1)
}

// TestTrainerResidualReproducesBotWhenNothingIsLearnable pins the residual's
// purpose: when the bot's own option IS the teacher's choice and no feature
// separates the options, the model reproduces the bot baseline exactly. The
// bot option is option 1 (NOT the first option) and carries no discriminating
// feature, so the teacher-kept baseline is a non-trivial 1.0 that a
// featureless head cannot reach; the prior alone does. ResidualInit 0 is
// measurably worse, proving the wiring is live rather than a coincidence.
func TestTrainerResidualReproducesBotWhenNothingIsLearnable(t *testing.T) {
	corpus := make([]policynet.Example, 300)
	for i := range corpus {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("nothing|s|%d", i%7)), Value: 1})
		ex := policynet.Example{Kind: "attackers", TeacherChoice: 0, BotIndex: 0, State: st}
		for j := 0; j < 3; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			// No feature distinguishes the options: the only signal is the prior.
			o.Target = policynet.OptionTarget{Labelled: true, Value: 0.5, Preferred: j == 1}
			o.BotPick = j == 1 // the bot's answer is a non-first option
			ex.Options = append(ex.Options, o)
		}
		corpus[i] = ex
	}
	run := func(ri float64) KindStat {
		t.Helper()
		res, err := Train(corpus, Config{Epochs: 6, Batch: 32, LR: 0.1, Seed: 7, Holdout: 0.15,
			Embed: 32, Hidden: 64, Mode: policynet.LossCE, RankWeight: 1, HuberDelta: 0.1, ResidualInit: ri})
		if err != nil {
			t.Fatalf("Train (residual %g): %v", ri, err)
		}
		return res.ByKind[0]
	}
	on := run(5)
	if on.BotTop1 != 1 || on.ModelTop1 != 1 {
		t.Fatalf("residual 5: bot %.3f model %.3f, want both 1.000 (the prior alone must reproduce the bot)", on.BotTop1, on.ModelTop1)
	}
	if off := run(0); off.ModelTop1 == 1 {
		t.Fatalf("residual 0 model top-1 1.000 — the no-prior arm should not already be perfect, or the wiring is not being exercised")
	}
}

// clipPinCorpus builds a corpus for the clip-semantics pins: per-example
// gradient norms straddle the pin cap, the per-batch summed norm exceeds it
// on every batch (asserted in the tests), values are untied so the hybrid
// value term carries signal (CE does not read values at all).
func clipPinCorpus(n int) []policynet.Example {
	out := make([]policynet.Example, n)
	rng := rand.New(rand.NewPCG(11, 7))
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		for d := 0; d < 6; d++ {
			st.Dense[d] = float32(rng.NormFloat64())
		}
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("cp|s|%d", i%11)), Value: 2},
			policynet.Feature{Row: policynet.HashID("cp|shared"), Value: 1},
		)
		pref := i % 3
		ex := policynet.Example{Kind: "attackers", TeacherChoice: pref, BotIndex: 0, State: st, Margin: 0.2}
		for j := 0; j < 3; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			for d := 0; d < 6; d++ {
				o.Dense[d] = float32(rng.NormFloat64())
			}
			o.Hashed = append(o.Hashed,
				policynet.Feature{Row: policynet.HashID(fmt.Sprintf("cp|p|%d", j)), Value: 2})
			if j == pref {
				o.Hashed = append(o.Hashed, policynet.Feature{Row: policynet.HashID("cp|good"), Value: 2})
			}
			o.Target = policynet.OptionTarget{Labelled: true, Preferred: j == pref, Value: 0.5 + 0.05*float64(j)}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

// clipPinBinding exercises two epochs of updates and reports the batch
// gradient norms over that window, so the test can assert the cap actually
// binds on live gradients (maxBatch > cap) and log the observed range.
func clipPinBinding(t *testing.T, examples []policynet.Example, cfg Config, cap float64) (minBatch, maxBatch float64) {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, cfg.Embed, cfg.Hidden, rand.New(rand.NewPCG(5, 6)))
	grads := m.NewGrads()
	lc := policynet.LossConfig{Mode: cfg.Mode, HuberDelta: cfg.HuberDelta, RankWeight: cfg.RankWeight, OverrideWeight: cfg.OverrideWeight}
	minBatch = math.Inf(1)
	for epoch := 0; epoch < 2; epoch++ {
		for base := 0; base < len(examples); base += cfg.Batch {
			end := base + cfg.Batch
			if end > len(examples) {
				end = len(examples)
			}
			grads.Reset()
			for _, ex := range examples[base:end] {
				m.LossGrad(ex, lc, grads)
			}
			if n := grads.Norm(); n < minBatch {
				minBatch = n
			}
			if n := grads.Norm(); n > maxBatch {
				maxBatch = n
			}
			m.ApplyGrads(grads, float32(cfg.LR/float64(end-base)))
		}
	}
	if maxBatch <= cap {
		t.Fatalf("pin premise lost: largest batch gradient norm %.3g <= cap %g over the measured window — the corpus no longer exercises the binding regime", maxBatch, cap)
	}
	return minBatch, maxBatch
}

// modelRelDiff is the largest over-blocks of (max|a−b| / max|a|) — the
// scale on which two trained models are compared here.
func modelRelDiff(a, b *policynet.Model) float64 {
	worst := 0.0
	for _, pair := range [][2][]float32{
		{a.Table, b.Table}, {a.StateW, b.StateW}, {a.StateB, b.StateB},
		{a.HidW, b.HidW}, {a.HidB, b.HidB}, {a.OutW, b.OutW},
	} {
		ma, md := 0.0, 0.0
		for i := range pair[0] {
			if v := math.Abs(float64(pair[0][i])); v > ma {
				ma = v
			}
			if d := math.Abs(float64(pair[0][i]) - float64(pair[1][i])); d > md {
				md = d
			}
		}
		if ma > 0 {
			if r := md / ma; r > worst {
				worst = r
			}
		}
	}
	return worst
}

// TestCERankWeightInertWhileTheClipBinds pins the rank-weight/clip
// interaction (ticket policytrain-clip-rankweight-interaction). The corpus
// is one where the two settings MUST differ — with the cap disabled, on
// live gradients, rank-weight 20 takes twenty-fold steps and lands far from
// rank-weight 1 — and the pin is that the per-batch clip erases exactly
// that difference:
//
//   - In pure CE mode a uniform RankWeight scales the single term's
//     gradient only; the per-batch clip divides that back out whenever it
//     binds, so with the cap on, rank-weight 1 and 20 must land on the same
//     model — to float32 rounding, ~1e-7 measured (NOT byte-identical: the
//     fused multiply-add in the gradient accumulation rounds differently
//     for a scaled addend; the ulp bound is the honest one).
//   - The same flag in hybrid mode IS a direction knob (the rank-vs-value
//     term mix; the value term runs here because the values are untied), so
//     the two weights must land on clearly different models — the flag is
//     not globally dead, only CE-inert.
//   - The trainer logs its warning on the inert setting.
func TestCERankWeightInertWhileTheClipBinds(t *testing.T) {
	corpus := clipPinCorpus(600)
	cfg := Config{Epochs: 2, Batch: 64, LR: 0.1, Seed: 1, Holdout: 0.15,
		Embed: 32, Hidden: 64, HuberDelta: 0.1, Mode: policynet.LossCE, RankWeight: 1, Clip: 0.5}
	// The premise helper measures the gradients the pin actually trains on,
	// so its LossConfig must carry the same mode and weight the primary run
	// arm chooses (CE at rank-weight 1, the shipped default).
	minB, maxB := clipPinBinding(t, corpus, cfg, 0.5)
	t.Logf("batch gradient norms over the measured window: [%.3g, %.3g], cap 0.5", minB, maxB)

	run := func(rw float64, mode policynet.LossMode, clip float64, log io.Writer) *policynet.Model {
		t.Helper()
		c := cfg
		c.Mode, c.RankWeight, c.Clip, c.Log = mode, rw, clip, log
		res, err := Train(corpus, c)
		if err != nil {
			t.Fatalf("Train(rw=%v mode=%v clip=%v): %v", rw, mode, clip, err)
		}
		return res.Model
	}
	var logBuf bytes.Buffer
	ce1 := run(1, policynet.LossCE, 0.5, nil)
	ce20 := run(20, policynet.LossCE, 0.5, &logBuf)
	if rel := modelRelDiff(ce1, ce20); rel > 1e-5 {
		t.Fatalf("CE rank-weight 1 vs 20 landed %.3g apart with the cap active — the uniform scale leaked into the step", rel)
	}
	if !strings.Contains(logBuf.String(), "inert in pure CE mode") {
		t.Fatalf("rank-weight 20 in CE mode produced no inertness warning; log:\n%s", logBuf.String())
	}
	logBuf.Reset()
	_ = run(1, policynet.LossCE, 0.5, &logBuf)
	if strings.Contains(logBuf.String(), "inert in pure CE mode") {
		t.Fatalf("rank-weight 1 (the default) must not warn; log:\n%s", logBuf.String())
	}
	ce20Free := run(20, policynet.LossCE, 0, nil)
	relFree := modelRelDiff(ce1, ce20Free)
	if relFree < 0.05 {
		t.Fatalf("with the cap DISABLED, CE rank-weight 20 landed only %.3g from rank-weight 1 — the corpus no longer separates the two settings, so this pin proves nothing", relFree)
	}
	// The warning is scoped to the clip-active arm: with -clip 0 a uniform
	// RankWeight is a real lr rescale and must NOT be called inert.
	logBuf.Reset()
	_ = run(20, policynet.LossCE, 0, &logBuf)
	if strings.Contains(logBuf.String(), "inert in pure CE mode") {
		t.Fatalf("rank-weight 20 with -clip 0 must not warn (it is an lr rescale there, not inert); log:\n%s", logBuf.String())
	}
	t.Logf("CE rank-weight 1 vs 20: cap on %.2g apart, cap off %.3g apart — the clip erases the uniform scale", modelRelDiff(ce1, ce20), relFree)

	h1 := run(1, policynet.LossHybrid, 0.5, nil)
	h20 := run(20, policynet.LossHybrid, 0.5, nil)
	relHybrid := modelRelDiff(h1, h20)
	if relHybrid < 0.05 {
		t.Fatalf("hybrid rank-weight 1 vs 20 landed only %.3g apart — the term-mix weight should steer a hybrid run visibly", relHybrid)
	}
	t.Logf("hybrid rank-weight 1 vs 20 (term mix): %.3g apart — the flag steers a hybrid run", relHybrid)
}

// collapseCorpus is the divergence pin's corpus: the preferred option is
// always the marker-carrying option 2 EXCEPT on conflict examples, whose
// inputs are identical to their neighbours' shape but whose label differs —
// an irreducible CE floor, the persistent gradient that keeps the
// uncapped embedding→hidden→embedding feedback loop compounding instead of
// starving it the way a converging corpus does.
func collapseCorpus(n int) []policynet.Example {
	out := make([]policynet.Example, n)
	rng := rand.New(rand.NewPCG(11, 7))
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		for d := 0; d < 6; d++ {
			st.Dense[d] = float32(rng.NormFloat64())
		}
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("x|s|%d", i%11)), Value: 2},
			policynet.Feature{Row: policynet.HashID("x|shared"), Value: 1},
		)
		conflict := rng.Float64() < 0.3
		marker := 2
		pref := marker
		if conflict {
			pref = i % 2
		}
		ex := policynet.Example{Kind: "attackers", TeacherChoice: pref, BotIndex: 0, State: st, Margin: 0.2}
		nopts := 3 + i%3
		for j := 0; j < nopts; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
			for d := 0; d < 6; d++ {
				o.Dense[d] = float32(rng.NormFloat64())
			}
			o.Hashed = append(o.Hashed,
				policynet.Feature{Row: policynet.HashID(fmt.Sprintf("x|p|%d", j)), Value: 2})
			if j == marker {
				o.Hashed = append(o.Hashed, policynet.Feature{Row: policynet.HashID("x|good"), Value: 2})
			}
			o.Target = policynet.OptionTarget{Labelled: true, Preferred: j == pref, Value: 0.5}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	for i := range out {
		for j := range out[i].Options {
			for k := range out[i].Options[j].Hashed {
				out[i].Options[j].Hashed[k].Value *= 2
			}
		}
	}
	return out
}

// TestClipZeroRejectsNonFiniteModel proves that the uncapped diagnosis mode
// fails at the update that first produces a non-finite learned parameter,
// while the same reproducer remains a valid, learning run with the default cap.
func TestClipZeroRejectsNonFiniteModel(t *testing.T) {
	corpus := collapseCorpus(600)
	cfg := Config{Epochs: 12, Batch: 64, LR: 1, Seed: 1, Holdout: 0.15,
		Embed: 32, Hidden: 64, Mode: policynet.LossCE, RankWeight: 1, HuberDelta: 0.1}

	uncapped := cfg
	uncapped.Clip = 0
	dead, err := Train(corpus, uncapped)
	if dead != nil {
		t.Fatalf("clip=0 returned a successful result after divergence: %+v", dead)
	}
	if err == nil {
		t.Fatal("clip=0: expected non-finite parameter error")
	}
	for _, want := range []string{"epoch ", "batch ", "[", "="} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("clip=0 error %q missing %q", err, want)
		}
	}
	blockNamed := false
	for _, block := range []string{"Table", "StateW", "StateB", "HidW", "HidB", "OutW", "OutB"} {
		blockNamed = blockNamed || strings.Contains(err.Error(), block+"[") || strings.Contains(err.Error(), block+"=")
	}
	if !blockNamed {
		t.Fatalf("clip=0 error %q does not identify a learned block", err)
	}

	capped := cfg
	capped.Clip = 1
	alive, err := Train(corpus, capped)
	if err != nil {
		t.Fatalf("clip=1: %v", err)
	}
	if alive == nil || len(alive.ByKind) == 0 {
		t.Fatal("clip=1: expected a successful result with per-kind readout")
	}
	k := alive.ByKind[0]
	if k.ModelTop1 < k.FirstTop1+0.2 {
		t.Fatalf("clip=1: model top-1 %.3f not clearly above first-option baseline %.3f", k.ModelTop1, k.FirstTop1)
	}
	if k.ModelTop1 == k.FirstTop1 {
		t.Fatalf("clip=1: model and first-option values unexpectedly equal at %.3f", k.ModelTop1)
	}
	t.Logf("clip=0 rejected: %v | clip=1: model %.3f > first %.3f", err, k.ModelTop1, k.FirstTop1)
}

func extraFeatureCorpus(n, nopts int) []policynet.Example {
	out := make([]policynet.Example, n)
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("xf|s|%d", i%7)), Value: 1})
		marked := i%3 == 0
		pref := 0
		if marked {
			pref = 2
		}
		ex := policynet.Example{Kind: "attackers", TeacherChoice: pref, BotIndex: 0, Margin: 0.02, State: st}
		for j := 0; j < nopts; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth), Extra: []float32{0}}
			o.Hashed = append(o.Hashed,
				policynet.Feature{Row: policynet.HashID(fmt.Sprintf("xf|p|%d", j)), Value: 1})
			if marked && j == 2 {
				o.Extra[0] = 1
			}
			o.Target = policynet.OptionTarget{Labelled: true, Preferred: j == pref, Value: 0.5}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

// TestTrainerLearnsFromExtraFeature proves the Extra channel is a trainable
// input, not inert plumbing: on a corpus whose ONLY discriminative signal is
// Option.Extra[0], pure CE must fit it to saturation. This is the synthetic
// control for the real-corpus result that even a leaky Extra feature is not
// exploited — if this test passes, the real-corpus underfit is a data/
// optimisation interaction, not a broken augmentation path.
func TestTrainerLearnsFromExtraFeature(t *testing.T) {
	corpus := extraFeatureCorpus(600, 3)
	res, err := Train(corpus, Config{Epochs: 10, Batch: 32, LR: 0.1, Seed: 7, Holdout: 0.15,
		Embed: 32, Hidden: 64, Mode: policynet.LossCE, RankWeight: 1, HuberDelta: 0.1, ExtraW: 1})
	if err != nil {
		t.Fatalf("Train: %v", err)
	}
	if len(res.ByKind) != 1 {
		t.Fatalf("%d kinds, want 1", len(res.ByKind))
	}
	k := res.ByKind[0]
	if k.FirstTop1 < 0.5 || k.FirstTop1 > 0.7 {
		t.Fatalf("first-option baseline %.3f outside the expected ~0.667", k.FirstTop1)
	}
	if k.ModelTop1 < 0.95 {
		t.Fatalf("pure CE on an Extra-only signal: top-1 %.3f < 0.95 — the Extra channel is not trainable", k.ModelTop1)
	}
	if res.Model.ExtraW != 1 || res.Model.InW != 2*32+128+24+1 {
		t.Fatalf("model geometry: ExtraW=%d InW=%d, want 1 / %d", res.Model.ExtraW, res.Model.InW, 2*32+128+24+1)
	}
	t.Logf("first-option %.3f | Extra-only pure CE %.3f", k.FirstTop1, k.ModelTop1)
}

// bceCalibrationCorpus builds the shape the attackers head must learn: ONE
// labelled option per decision (the majority of the label corpus's attackers
// decisions), whose inclusion the teacher decides from an option feature. A
// softmax loss has ZERO gradient on a one-option decision (the softmax over a
// single score is flat), so the absolute level is untrained and the seat has
// no calibrated zero; the per-option binary loss trains it directly.
func bceCalibrationCorpus(n int) []policynet.Example {
	out := make([]policynet.Example, n)
	for i := 0; i < n; i++ {
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		// A state feature correlated with the label, so the model has something
		// to condition the absolute score on (a bias alone cannot separate the
		// classes on the holdout).
		cls := i % 2
		st.Sparse = append(st.Sparse,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("bce|s|%d", cls)), Value: 1})
		ex := policynet.Example{Kind: "attackers", TeacherChoice: 0, BotIndex: 0, State: st}
		o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth)}
		o.Hashed = append(o.Hashed,
			policynet.Feature{Row: policynet.HashID(fmt.Sprintf("bce|p|%d", cls)), Value: 1})
		o.Target = policynet.OptionTarget{Labelled: true, Preferred: cls == 1, Value: 0.5}
		ex.Options = []policynet.Option{o}
		out[i] = ex
	}
	return out
}

// TestTrainerBCEAttackersHeadIsCalibratedAtZero is the trainer half of the
// L9d gate. On the one-option corpus above, an attackers head trained with
// the per-kind binary loss (KindModes: attackers=bce) must produce a score
// SIGN that separates the classes: preferred options above 0, non-preferred
// below. That is the calibrated boundary the seat's admission rule reads.
//
// The plain-ce arm is the same corpus under today's loss: a one-option
// softmax is flat, so it cannot move the absolute level and its score range
// stays one-signed — the state that made `score > 0` meaningless and shipped
// the L9d bug.
func TestTrainerBCEAttackersHeadIsCalibratedAtZero(t *testing.T) {
	corpus := bceCalibrationCorpus(400)
	train := func(kindModes map[decision.Kind]policynet.LossMode) *policynet.Model {
		t.Helper()
		res, err := Train(corpus, Config{Epochs: 30, Batch: 32, LR: 0.1, Seed: 7, Holdout: 0.15,
			Embed: 32, Hidden: 64, Mode: policynet.LossCE, RankWeight: 1, HuberDelta: 0.1,
			KindModes: kindModes})
		if err != nil {
			t.Fatalf("Train: %v", err)
		}
		return res.Model
	}
	sep := func(m *policynet.Model) (posPreferred, posOther int) {
		for i := range corpus {
			s := m.Score(corpus[i].State, corpus[i].Options)[0]
			if corpus[i].Options[0].Target.Preferred {
				if s > 0 {
					posPreferred++
				}
			} else if s > 0 {
				posOther++
			}
		}
		return posPreferred, posOther
	}

	bce := train(map[decision.Kind]policynet.LossMode{decision.KAttackers: policynet.LossBCE})
	bp, bo := sep(bce)
	if bp < len(corpus)/2 {
		t.Fatalf("bce head: only %d of %d preferred options scored > 0 — the absolute level is not calibrated at zero", bp, len(corpus)/2)
	}
	if bo > len(corpus)/10 {
		t.Fatalf("bce head: %d of %d NON-preferred options scored > 0 — the calibrated boundary does not separate", bo, len(corpus)/2)
	}

	ce := train(nil) // today's loss: the softmax cannot train a one-option decision
	cp, co := sep(ce)
	// The CE arm CANNOT separate the classes: with a one-option softmax there
	// is no within-decision gradient, so the absolute level is whatever the
	// shared trunk happens to be and every option lands on one side of zero.
	// That is the L9d state reproduced.
	half := len(corpus) / 2
	if cp > 8*half/10 && co < half/10 {
		t.Fatalf("plain-ce arm: preferred>0 %d/%d, other>0 %d/%d — the one-option softmax separated the classes, so this gate no longer pins the L9d state",
			cp, half, co, half)
	}
	t.Logf("bce: preferred>0 %d/%d, other>0 %d/%d | ce (one-signed): preferred>0 %d/%d, other>0 %d/%d",
		bp, half, bo, half, cp, half, co, half)
}
