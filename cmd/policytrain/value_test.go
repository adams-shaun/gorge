package main

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/policynet"
)

// policyDigest is an FNV-1a 64 over every POLICY parameter block's float32
// bits, the per-epoch statistics and the per-kind holdout table: everything
// a value-off run produces.
func policyDigest(res *Result) uint64 {
	h := fnv.New64a()
	var b [4]byte
	put := func(fs []float32) {
		for _, f := range fs {
			u := math.Float32bits(f)
			b[0], b[1], b[2], b[3] = byte(u), byte(u>>8), byte(u>>16), byte(u>>24)
			h.Write(b[:])
		}
	}
	m := res.Model
	put(m.Table)
	put(m.StateW)
	put(m.StateB)
	put(m.HidW)
	put(m.HidB)
	put(m.OutW)
	put([]float32{m.OutB, m.ResidualW})
	// The pre-value-head EpochStat shape: the value readout fields (zero on
	// a value-off run) are left out so the digest reads exactly the bytes the
	// pre-change trainer's fmt.Fprint(res.Epochs) produced.
	type legacyEpoch struct {
		Epoch                                          int
		TrainLoss, TrainTop1, HoldoutLoss, HoldoutTop1 float64
	}
	eps := make([]legacyEpoch, len(res.Epochs))
	for i, e := range res.Epochs {
		eps[i] = legacyEpoch{e.Epoch, e.TrainLoss, e.TrainTop1, e.HoldoutLoss, e.HoldoutTop1}
	}
	fmt.Fprint(h, eps, res.ByKind, res.TrainN, res.HoldoutN, res.Skipped)
	return h.Sum64()
}

// digestConfigs are the two value-off runs the pre-value-head trainer was
// measured at (by-example and by-game split, residual prior on in the
// second). The second sets ValueHidden 32 — the CLI default — with weight 0,
// which must build no value head.
func digestConfigs() []Config {
	a := testConfig(2)
	a.Epochs = 12
	b := testConfig(5)
	b.Epochs = 12
	b.HoldoutBy = HoldoutByGame
	b.ResidualInit = 2
	b.ValueHidden = 32
	return []Config{a, b}
}

// TestValueOffIsBitIdenticalToPreValueTrainer pins step 3's contract: with
// the value weight 0 every policy block, every epoch statistic and the
// per-kind holdout table are bit-identical to the trainer BEFORE the value
// head existed. The digests were measured on the pre-change trainer (base
// 216a3314) with this exact policyDigest and these configs.
func TestValueOffIsBitIdenticalToPreValueTrainer(t *testing.T) {
	corpus := gameCorpus(96)
	want := []uint64{0x1c2901484ee22394, 0x3b46bbe57eda4aa8}
	for i, cfg := range digestConfigs() {
		res, err := Train(corpus, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if res.Model.HasValue() || res.Value != nil {
			t.Fatalf("config %d: value weight 0 built a value head", i)
		}
		if got := policyDigest(res); got != want[i] {
			t.Fatalf("config %d: policy digest %#016x, want the pre-value-head trainer's %#016x", i, got, want[i])
		}
	}
}

// valueCorpus is gameCorpus with outcomes that a state feature predicts: the
// game's first dense state scalar's sign carries a win with probability 0.85
// (a noisy but learnable signal), and every example of one game shares its
// state sign and outcome. Teacher values track the outcome loosely.
func valueCorpus(n int) []policynet.Example {
	ex := gameCorpus(n)
	rng := rand.New(rand.NewPCG(123, 456))
	type game struct {
		sign float32
		win  float64
	}
	games := map[gameKey]game{}
	for i := range ex {
		k := gameKey{ex[i].Pair, ex[i].Seed, ex[i].GameIndex}
		g, ok := games[k]
		if !ok {
			g.sign = 1
			if rng.IntN(2) == 0 {
				g.sign = -1
			}
			pWin := 0.15
			if g.sign > 0 {
				pWin = 0.85
			}
			if rng.Float64() < pWin {
				g.win = 1
			}
			games[k] = g
		}
		ex[i].State.Dense[0] = g.sign * (0.5 + 0.5*float32(rng.Float64()))
		ex[i].Outcome, ex[i].HasOutcome = g.win, true
		ex[i].TeacherValue, ex[i].HasTeacherValue = 0.3+0.4*g.win, true
	}
	return ex
}

func valueConfig(seed int64) Config {
	c := testConfig(seed)
	c.Epochs = 40
	c.HoldoutBy = HoldoutByGame
	c.Holdout = 0.3
	c.ValueHidden = 8
	c.ValueWeight = 1
	return c
}

// TestValueHeadBeatsBaseRate: on a corpus whose outcome a state feature
// predicts, the by-game holdout value log loss and Brier fall below the
// base-rate predictor's; the split is the value-off run's split (the value
// init draws from its own rng), and the run is deterministic.
func TestValueHeadBeatsBaseRate(t *testing.T) {
	corpus := valueCorpus(240)
	var log bytes.Buffer
	cfg := valueConfig(4)
	cfg.Log = &log
	res, err := Train(corpus, cfg)
	if err != nil {
		t.Fatal(err)
	}
	v := res.Value
	if v == nil || !res.Model.HasValue() || res.Model.ValueHidden != 8 {
		t.Fatal("value weight 1 trained no value head")
	}
	if v.HoldoutN == 0 || v.HoldoutN != res.HoldoutN || v.TrainN != res.TrainN {
		t.Fatalf("value readout n: train %d holdout %d, want %d/%d", v.TrainN, v.HoldoutN, res.TrainN, res.HoldoutN)
	}
	t.Logf("value holdout: log loss %.4f brier %.4f | base rate %.3f log loss %.4f brier %.4f",
		v.LogLoss, v.Brier, v.BaseRate, v.BaseLogLoss, v.BaseBrier)
	if !(v.LogLoss < v.BaseLogLoss) || !(v.Brier < v.BaseBrier) {
		t.Fatalf("value head did not beat the base rate: log loss %.4f vs %.4f, brier %.4f vs %.4f", v.LogLoss, v.BaseLogLoss, v.Brier, v.BaseBrier)
	}
	last := res.Epochs[len(res.Epochs)-1]
	if last.HoldoutValueLogLoss != v.LogLoss || last.HoldoutValueBrier != v.Brier {
		t.Fatal("final epoch stat and Result.Value disagree")
	}
	if !strings.Contains(log.String(), "value holdout n ") || !strings.Contains(log.String(), "base rate") {
		t.Fatalf("log lacks the per-epoch value line:\n%s", log.String())
	}

	// Same split as the value-off run: the value init did not touch the main
	// rng's split draw.
	off := cfg
	off.ValueWeight, off.Epochs, off.Log = 0, 1, nil
	resOff, err := Train(corpus, off)
	if err != nil {
		t.Fatal(err)
	}
	if resOff.TrainN != res.TrainN || resOff.HoldoutN != res.HoldoutN {
		t.Fatalf("value-on split %d/%d, value-off %d/%d", res.TrainN, res.HoldoutN, resOff.TrainN, resOff.HoldoutN)
	}

	// Deterministic, and the value head survives the checkpoint.
	cfg.Log = nil
	res2, err := Train(corpus, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	if err := policynet.WriteCheckpoint(res.Model, &a); err != nil {
		t.Fatal(err)
	}
	if err := policynet.WriteCheckpoint(res2.Model, &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) || *res.Value != *res2.Value {
		t.Fatal("value-on training is not reproducible from its seed")
	}
	m, err := policynet.LoadCheckpoint(bytes.NewReader(a.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	st := corpus[0].State
	if m.ValueHidden != 8 || m.Value(st) != res.Model.Value(st) {
		t.Fatal("value head changed across the checkpoint")
	}
}

// TestValueBlendTrainsOnTeacherValue: blend 1 needs no outcome (targets are
// the teacher values), blend 0.5 skips the outcome-less examples.
func TestValueBlendTrainsOnTeacherValue(t *testing.T) {
	corpus := valueCorpus(96)
	for i := range corpus {
		if i%2 == 0 {
			corpus[i].HasOutcome = false
		}
	}
	cfg := valueConfig(6)
	cfg.Epochs = 3
	cfg.ValueBlend = 1
	res, err := Train(corpus, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Value.HoldoutN != res.HoldoutN {
		t.Fatalf("blend 1: %d holdout targets of %d", res.Value.HoldoutN, res.HoldoutN)
	}
	cfg.ValueBlend = 0.5
	res, err = Train(corpus, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Value.HoldoutN == 0 || res.Value.HoldoutN >= res.HoldoutN {
		t.Fatalf("blend 0.5: %d holdout targets of %d, want the outcome-bearing subset", res.Value.HoldoutN, res.HoldoutN)
	}
}

// TestValueConfigRejected pins the value flags' validation.
func TestValueConfigRejected(t *testing.T) {
	corpus := valueCorpus(16)
	for name, mut := range map[string]func(*Config){
		"negative weight":   func(c *Config) { c.ValueWeight = -1 },
		"blend below 0":     func(c *Config) { c.ValueBlend = -0.1 },
		"blend above 1":     func(c *Config) { c.ValueBlend = 1.5 },
		"weight, no hidden": func(c *Config) { c.ValueHidden = 0 },
	} {
		cfg := valueConfig(1)
		mut(&cfg)
		if _, err := Train(corpus, cfg); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

// TestEndToEndValueFlags drives the CLI with the value flags: the checkpoint
// carries the value head and the final table prints.
func TestEndToEndValueFlags(t *testing.T) {
	path := writeJSONLCorpus(t, 48)
	out := filepath.Join(t.TempDir(), "model.bin")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-corpus", path, "-out", out,
		"-epochs", "2", "-embed", "8", "-hidden", "8", "-batch", "8",
		"-seed", "9", "-holdout", "0.2", "-holdout-by", "game",
		"-value-weight", "1", "-value-hidden", "4", "-value-blend", "1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "value head holdout") || !strings.Contains(stdout.String(), "value-hidden=4") {
		t.Fatalf("stdout missing the value table:\n%s", stdout.String())
	}
	m, err := policynet.LoadCheckpointFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if m.ValueHidden != 4 {
		t.Fatalf("checkpoint value hidden %d, want 4", m.ValueHidden)
	}
}
