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

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// banditInit is a small model whose checkpoint round-trip is the clone the
// PPO tests start every run from.
func banditInit(t *testing.T, value bool) []byte {
	t.Helper()
	m := policynet.NewModel(policynet.TableRows, 8, 16, rand.New(rand.NewPCG(3, 4)))
	if value {
		m.InitValue(8, rand.New(rand.NewPCG(5, 6)))
	}
	var buf bytes.Buffer
	if err := policynet.WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func loadInit(t *testing.T, raw []byte) *policynet.Model {
	t.Helper()
	m, err := policynet.LoadCheckpoint(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// banditCorpus is an on-policy contextual bandit: each decision's state
// carries one of two context rows, three options are offered, and the game
// is won with probability 0.8 when the played option is the context's good
// one (0 for context A, 2 for context B) and 0.2 otherwise. The recorded
// old scores are the init model's own, as the seat would have recorded them;
// the played option cycles so every option is seen. Softmax decisions on
// even indices, subset decisions (the good option in or out) on odd ones.
func banditCorpus(t *testing.T, init *policynet.Model, n int) []policynet.Example {
	t.Helper()
	rng := rand.New(rand.NewPCG(11, 12))
	out := make([]policynet.Example, n)
	for i := range out {
		ctx := i % 2
		st := policynet.State{Dense: make([]float32, policynet.DenseWidth)}
		st.Sparse = []policynet.Feature{{Row: policynet.HashID(fmt.Sprintf("bandit|ctx|%d", ctx)), Value: 1}}
		var opts []policynet.Option
		for j := 0; j < 3; j++ {
			o := policynet.Option{Dense: make([]float32, policynet.OptionDenseWidth), Target: policynet.OptionTarget{Labelled: true}}
			o.Hashed = []policynet.Feature{{Row: policynet.HashID(fmt.Sprintf("bandit|opt|%d", j)), Value: 1}}
			opts = append(opts, o)
		}
		good := 0
		if ctx == 1 {
			good = 2
		}
		subset := (i/2)%2 == 1
		chosen := make([]bool, 3)
		played := (i / 4) % 3
		chosen[played] = true
		if subset && rng.IntN(2) == 0 {
			chosen[(played+1)%3] = true
		}
		p := 0.2
		if chosen[good] {
			p = 0.8
		}
		win := 0.0
		if rng.Float64() < p {
			win = 1
		}
		ex := policynet.Example{Pair: "bandit", GameIndex: i / 3, Seed: uint64(i / 3), Kind: decision.KPriority, State: st, Options: opts,
			Outcome: win, HasOutcome: true}
		if subset {
			ex.Kind = decision.KAttackers
		}
		ex.PPO = &policynet.PPOTarget{Subset: subset, OldScores: init.Score(st, opts), Chosen: chosen}
		out[i] = ex
	}
	return out
}

func banditPPOConfig() PPOConfig {
	return PPOConfig{Epochs: 6, Batch: 16, LR: 0.5, Seed: 7, Holdout: 0.1, Clip: 1,
		PPOClip: 0.2, PPOKL: 0.05, ValueWeight: 0.5, ValueHidden: 8, AdvNorm: true, Baseline: BaselineMean}
}

// goodProb is the mean softmax probability of each context's good option.
func goodProb(m *policynet.Model, examples []policynet.Example) float64 {
	sum, n := 0.0, 0
	for _, ex := range examples {
		if ex.PPO.Subset {
			continue
		}
		good := 0
		if ex.State.Sparse[0].Row == policynet.HashID("bandit|ctx|1") {
			good = 2
		}
		c := make([]bool, 3)
		c[good] = true
		lp, _ := policynet.PPOLogProb(ex.Options, m.Score(ex.State, ex.Options), c, false)
		sum += math.Exp(lp)
		n++
	}
	return sum / float64(n)
}

// TestPPOLearnsTheBandit: one PPO round on the init's own recorded play
// raises the probability of each context's winning option, with the exact
// π_old (no re-score mismatch), and both baselines work.
func TestPPOLearnsTheBandit(t *testing.T) {
	raw := banditInit(t, true)
	for _, baseline := range []string{BaselineMean, BaselineValue} {
		init := loadInit(t, raw)
		corpus := banditCorpus(t, init, 600)
		before := goodProb(init, corpus)
		cfg := banditPPOConfig()
		cfg.Baseline = baseline
		res, err := TrainPPO(corpus, init, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if res.ScoreMismatch != 0 {
			t.Fatalf("%s: %d re-score mismatches on the init's own scores", baseline, res.ScoreMismatch)
		}
		after := goodProb(res.Model, corpus)
		// One round is a TRUST-REGION step: the clip (ε 0.2) stops the
		// surrogate from pushing a played option's ratio much past 1.2, so the
		// good option's probability rises clearly but boundedly.
		if after < before+0.03 || after > 1.35*before {
			t.Fatalf("%s: good-option probability %.3f -> %.3f, want a rise inside the trust region", baseline, before, after)
		}
		if res.Final.KL <= 0 || res.PolicyN == 0 || len(res.ByKind) != 2 {
			t.Fatalf("%s: readout %+v", baseline, res)
		}
	}
}

// TestPPOKindsFilter: a kind outside -ppo-kinds trains no policy term.
func TestPPOKindsFilter(t *testing.T) {
	raw := banditInit(t, false)
	init := loadInit(t, raw)
	corpus := banditCorpus(t, init, 200)
	cfg := banditPPOConfig()
	cfg.Kinds = []decision.Kind{decision.KAttackers}
	cfg.ValueWeight = 0
	res, err := TrainPPO(corpus, init, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range res.ByKind {
		if k.PolicyOn != (k.Kind == decision.KAttackers) {
			t.Fatalf("kind %s policy_on %v", k.Kind, k.PolicyOn)
		}
	}
	for _, e := range res.Epochs {
		if e.Surrogate == 0 {
			t.Fatal("the attackers policy term did not train")
		}
	}
}

// TestPPODeterministic: the same corpus, init and config give a
// byte-identical checkpoint and readout.
func TestPPODeterministic(t *testing.T) {
	raw := banditInit(t, true)
	var ckpts, stats [2][]byte
	for i := range 2 {
		init := loadInit(t, raw)
		res, err := TrainPPO(banditCorpus(t, init, 300), init, banditPPOConfig())
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := policynet.WriteCheckpoint(res.Model, &buf); err != nil {
			t.Fatal(err)
		}
		ckpts[i] = buf.Bytes()
		if stats[i], err = json.Marshal(res); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(ckpts[0], ckpts[1]) || !bytes.Equal(stats[0], stats[1]) {
		t.Fatal("two identical PPO runs differ")
	}
}

// TestPPODetectsAForeignCorpus: a corpus recorded by a different checkpoint
// is reported through the re-score check, not silently trained as on-policy.
func TestPPODetectsAForeignCorpus(t *testing.T) {
	raw := banditInit(t, false)
	init := loadInit(t, raw)
	corpus := banditCorpus(t, init, 60)
	init.OutB += 0.25
	cfg := banditPPOConfig()
	cfg.ValueWeight = 0
	res, err := TrainPPO(corpus, init, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.ScoreMismatch == 0 || math.Abs(res.ScoreMaxDiff-0.25) > 1e-3 {
		t.Fatalf("mismatch %d max %g, want every option 0.25 off", res.ScoreMismatch, res.ScoreMaxDiff)
	}
}

func TestPPORefusals(t *testing.T) {
	raw := banditInit(t, false)
	init := loadInit(t, raw)
	corpus := banditCorpus(t, init, 20)
	cfg := banditPPOConfig()
	cfg.Baseline = BaselineValue
	if _, err := TrainPPO(corpus, init, cfg); err == nil || !strings.Contains(err.Error(), "value head") {
		t.Fatalf("value baseline without a value head: %v", err)
	}
	cfg = banditPPOConfig()
	for i := range corpus {
		corpus[i].HasOutcome = false
	}
	if _, err := TrainPPO(corpus, init, cfg); err == nil {
		t.Fatal("a corpus without outcomes must be refused")
	}
}

// TestPPOCLIEndToEnd runs the -ppo-corpus CLI shape on a corpus written in
// the on-policy record format.
func TestPPOCLIEndToEnd(t *testing.T) {
	dir := t.TempDir()
	raw := banditInit(t, true)
	initPath := filepath.Join(dir, "init.gpol")
	if err := os.WriteFile(initPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	init := loadInit(t, raw)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i, ex := range banditCorpus(t, init, 120) {
		rec := policynet.OnPolicyRecord{RecordType: policynet.OnPolicyRecordType, SchemaVersion: policynet.OnPolicySchemaVersion,
			EncoderHash: fmt.Sprintf("%016x", policynet.EncoderHash()), Pair: ex.Pair, GameIndex: ex.GameIndex, Seed: ex.Seed,
			Kind: ex.Kind, Subset: ex.PPO.Subset, State: policynet.EncodeOnPolicyState(ex.State), Scores: ex.PPO.OldScores,
			Outcome: ex.Outcome, OutcomeKnown: i%17 != 0}
		for j, o := range ex.Options {
			rec.Options = append(rec.Options, policynet.EncodeOnPolicyOption(o, true))
			if ex.PPO.Chosen[j] {
				rec.Chosen = append(rec.Chosen, j)
			}
		}
		if err := enc.Encode(&rec); err != nil {
			t.Fatal(err)
		}
	}
	corpus := filepath.Join(dir, "corpus.jsonl")
	if err := os.WriteFile(corpus, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	out, stats := filepath.Join(dir, "out.gpol"), filepath.Join(dir, "stats.json")
	code := run([]string{"-ppo-corpus", corpus, "-init", initPath, "-out", out, "-stats-out", stats,
		"-epochs", "2", "-lr", "0.3", "-batch", "16", "-value-weight", "0.5"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if _, err := policynet.LoadCheckpointFile(out); err != nil {
		t.Fatal(err)
	}
	var res PPOResult
	data, _ := os.ReadFile(stats)
	if err := json.Unmarshal(data, &res); err != nil || res.NoOutcome != 8 || res.ScoreMismatch != 0 {
		t.Fatalf("stats %+v (%v)", res, err)
	}
	if !strings.Contains(stdout.String(), "final vs pi_old") {
		t.Fatalf("stdout %q", stdout.String())
	}
	if code := run([]string{"-ppo-corpus", corpus, "-corpus", corpus, "-out", out}, &stdout, &stderr); code == 0 {
		t.Fatal("-ppo-corpus with -corpus must be refused")
	}
}

// TestAdmittedSetMirrorsTheSeatVote pins the readout's restatement of the
// seat's admission vote on the shapes seat/policynet_test.go pins for
// admissionThreshold: straddling scores vote on the sign, one-signed scores
// on the decision mean (inclusive), an exact tie admits nothing, and a lone
// option votes on its sign.
func TestAdmittedSetMirrorsTheSeatVote(t *testing.T) {
	for _, tc := range []struct {
		scores []float32
		want   []bool
	}{
		{[]float32{-1, 0.5, 2}, []bool{false, true, true}},
		{[]float32{0, 0.5}, []bool{false, true}},
		{[]float32{3, 5, 4}, []bool{false, true, true}},
		{[]float32{-3, -5, -4}, []bool{true, false, true}},
		{[]float32{2, 2}, []bool{false, false}},
		{[]float32{0.1}, []bool{true}},
		{[]float32{-0.1}, []bool{false}},
	} {
		got := admittedSet(tc.scores, false)
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("admittedSet(%v) = %v, want %v", tc.scores, got, tc.want)
			}
		}
	}
	if got := admittedSet([]float32{3, 5, 4, -1}, true); got[0] != true || got[3] != false {
		t.Fatalf("sign vote %v", got)
	}
	if got := admittedSet([]float32{3, 5, 4}, true); !got[0] || !got[1] || !got[2] {
		t.Fatalf("sign vote on an all-positive decision must admit every option: %v", got)
	}
}

// TestVDWMArmTrains: the VDWM mode runs on the same corpus shape, is
// deterministic, closes the margins as it trains (the mean hinge falls), and
// reports the disagreement mass by kind.
func TestVDWMArmTrains(t *testing.T) {
	raw := banditInit(t, true)
	var stats [2][]byte
	for i := range 2 {
		init := loadInit(t, raw)
		cfg := banditPPOConfig()
		cfg.VDWM, cfg.Epochs = true, 8
		// A consistent recorded answer per context (as the deployed argmax
		// is), so every margin can clear.
		corpus := banditCorpus(t, init, 300)
		for _, ex := range corpus {
			good := 0
			if ex.State.Sparse[0].Row == policynet.HashID("bandit|ctx|1") {
				good = 2
			}
			for k := range ex.PPO.Chosen {
				ex.PPO.Chosen[k] = k == good
			}
		}
		res, err := TrainPPO(corpus, init, cfg)
		if err != nil {
			t.Fatal(err)
		}
		first, last := res.Epochs[0], res.Epochs[len(res.Epochs)-1]
		if first.MarginActive <= 0 || first.Margin <= 0 || last.Margin >= 0.9*first.Margin {
			t.Fatalf("mean hinge %.4f -> %.4f (active %.3f -> %.3f), want a clear fall", first.Margin, last.Margin, first.MarginActive, last.MarginActive)
		}
		for _, k := range res.ByKind {
			if k.MeanDisagree <= 0 || k.MeanAbsAdv <= 0 {
				t.Fatalf("kind %s disagreement readout %+v", k.Kind, k)
			}
		}
		if len(res.Final.ValueByTurn) != 3 {
			t.Fatalf("value-by-turn buckets %+v", res.Final.ValueByTurn)
		}
		var err2 error
		if stats[i], err2 = json.Marshal(res); err2 != nil {
			t.Fatal(err2)
		}
	}
	if !bytes.Equal(stats[0], stats[1]) {
		t.Fatal("two identical VDWM runs differ")
	}
}

func TestAUCKnownAnswer(t *testing.T) {
	if a := auc([]float64{0.1, 0.4, 0.35, 0.8}, []bool{false, false, true, true}); math.Abs(a-0.75) > 1e-12 {
		t.Fatalf("auc %g, want 0.75", a)
	}
	if a := auc([]float64{0.5, 0.5}, []bool{false, true}); a != 0.5 {
		t.Fatalf("tied auc %g", a)
	}
	if a := auc([]float64{0.2}, []bool{true}); a != 0.5 {
		t.Fatalf("one-class auc %g", a)
	}
}

// TestPPOSampledBehaviourCorpus (ticket pn14): a corpus whose answers were
// SAMPLED carries the behaviour log-probability; the round trains through the
// π_new/π_b ratio, still learns the bandit, and the readout counts the
// sampled and off-greedy decisions.
func TestPPOSampledBehaviourCorpus(t *testing.T) {
	raw := banditInit(t, true)
	init := loadInit(t, raw)
	corpus := banditCorpus(t, init, 600)
	for i := range corpus {
		p := corpus[i].PPO
		in := make([]bool, len(p.Chosen))
		for k := range in {
			in[k] = true
		}
		lp, ok := policynet.TemperedLogProb(p.OldScores, in, p.Chosen, p.Subset, 2)
		if !ok {
			t.Fatal("behaviour log-prob undefined")
		}
		p.BehaviourLogP, p.HasBehaviour = lp, true
	}
	before := goodProb(init, corpus)
	res, err := TrainPPO(corpus, init, banditPPOConfig())
	if err != nil {
		t.Fatal(err)
	}
	if after := goodProb(res.Model, corpus); after < before+0.03 {
		t.Fatalf("good-option probability %.3f -> %.3f", before, after)
	}
	for _, k := range res.ByKind {
		if k.Sampled != k.N || k.OffGreedy == 0 || k.MeanPBeh <= 0 || k.MeanPBeh > 1 {
			t.Fatalf("kind readout %+v", k)
		}
	}
}

// TestUpgradeEntityCLI: -upgrade-entity turns an mz checkpoint into an
// entity one that scores identically, refuses a non-mz init, and the PPO
// mode refuses a corpus whose feature set is not the init's.
func TestUpgradeEntityCLI(t *testing.T) {
	dir := t.TempDir()
	m := policynet.NewModel(policynet.TableRows, 8, 16, rand.New(rand.NewPCG(3, 4)))
	v1 := filepath.Join(dir, "v1.gpol")
	if err := m.SaveCheckpoint(v1); err != nil {
		t.Fatal(err)
	}
	m.Features = policynet.FeaturesMZ
	mz := filepath.Join(dir, "mz.gpol")
	if err := m.SaveCheckpoint(mz); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	ent := filepath.Join(dir, "ent.gpol")
	if code := run([]string{"-upgrade-entity", "6", "-init", mz, "-out", ent}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	e, err := policynet.LoadCheckpointFile(ent)
	if err != nil || e.Features != policynet.FeaturesEntity || e.EntK != 6 {
		t.Fatalf("upgraded checkpoint: %v %+v", err, e)
	}
	if code := run([]string{"-upgrade-entity", "6", "-init", v1, "-out", filepath.Join(dir, "x.gpol")}, &stdout, &stderr); code == 0 {
		t.Fatal("upgrading a v1 checkpoint must be refused")
	}
	// A v1 corpus (the bandit's) against the entity init.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, ex := range banditCorpus(t, m, 8) {
		rec := policynet.OnPolicyRecord{RecordType: policynet.OnPolicyRecordType, SchemaVersion: policynet.OnPolicySchemaVersion,
			EncoderHash: fmt.Sprintf("%016x", policynet.EncoderHash()), Kind: ex.Kind, Subset: ex.PPO.Subset,
			State: policynet.EncodeOnPolicyState(ex.State), Scores: ex.PPO.OldScores, Chosen: []int{0}, OutcomeKnown: true}
		for _, o := range ex.Options {
			rec.Options = append(rec.Options, policynet.EncodeOnPolicyOption(o, true))
		}
		if err := enc.Encode(&rec); err != nil {
			t.Fatal(err)
		}
	}
	corpus := filepath.Join(dir, "c.jsonl")
	if err := os.WriteFile(corpus, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code := run([]string{"-ppo-corpus", corpus, "-init", ent, "-out", filepath.Join(dir, "y.gpol")}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "feature set") {
		t.Fatalf("a v1 corpus against an entity init must be refused: %d %s", code, stderr.String())
	}
}
