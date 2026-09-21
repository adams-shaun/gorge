package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
