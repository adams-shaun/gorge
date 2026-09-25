package main

import (
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/internal/policynet"
)

// The value-only trainer (ticket pn17, -value-corpus): a state/outcome dump
// (internal/policynet/statedump.go) has NO labelled options, so Train —
// whose examples each carry an option list and whose loss is a policy loss —
// cannot consume it, and -loss value is still joint policy-example training.
// This path trains ONLY the value head on the records' outcomes with
// BCE/log-loss:
//
//   - data: policynet.LoadStateOutcome, one StateExample per record, encoded
//     under the -features set (redacted v1/mz; the diagnostic mz-opphand /
//     mz-oracle encodings train too but stay measurement-only — see
//     zeroPolicyHead and the CLI's checkpoint refusal, which keeps
//     WriteCheckpoint's diagnostic rejection byte for byte).
//   - holdout: by SEED BLOCK (the default and only unit): whole game seeds
//     are held out, so no record from a held-out seed is ever trained on —
//     a seed's records share shuffles and draws and a per-example split
//     would leak. The split is derived from cfg.Seed alone, like Train's.
//   - loss: w=1 BCE(sigmoid(V(s)), outcome) per record (policynet.ValueGrad);
//     the joint trainer's -value-weight is a term-MIX knob the single-term
//     value-only loss does not need (a uniform scale the per-batch clip
//     normalises away, the RankWeight-in-CE argument at LossConfig).
//   - readout: per-epoch train log loss; final holdout log loss and Brier
//     beside the train-split base-rate predictor (the ValueStat contract),
//     and holdout log loss and AUC by turn bucket 1–3 / 4–6 / 7+ — the
//     oracle dump's buckets, which differ from the PPO readout's 1–6/7–12/
//     13+ (ValueBucket/auc are shared; the bucket table is local).
//
// Same seed + same dump ⇒ byte-identical checkpoint. The wall clock is
// never read (the trainer discipline in train.go's header).

// ValueOnlyConfig is one value-only run's knobs — the -value-corpus flag
// surface (the shared flags -epochs/-lr/-batch/-seed/-holdout/-embed/
// -hidden/-value-hidden/-clip map one to one).
type ValueOnlyConfig struct {
	Epochs      int
	Batch       int
	LR          float64
	Seed        int64
	Holdout     float64 // seed-block holdout fraction, [0,1)
	Embed       int
	Hidden      int
	ValueHidden int
	Clip        float64
	// OracleCheckpoint (-oracle-checkpoint, ticket pn17-a1) writes a
	// FeaturesMZOppHand model as an oracle checkpoint for the search seat's
	// omniscient leaf instead of the diagnostic run's no-checkpoint readout.
	// It never touches training.
	OracleCheckpoint bool
	Log              io.Writer
}

// ValueOnlyEpochStat is one epoch's readout: the mean train BCE (log loss)
// over the train split, and the holdout log loss (the same arithmetic on
// the seed-block holdout).
type ValueOnlyEpochStat struct {
	Epoch          int
	TrainLogLoss   float64
	HoldoutLogLoss float64
}

// ValueOnlyResult is one value-only run's model and readouts.
type ValueOnlyResult struct {
	Model    *policynet.Model
	TrainN   int
	HoldoutN int
	// HoldoutSeeds is the number of held-out seed blocks (the split unit).
	HoldoutSeeds int
	Epochs       []ValueOnlyEpochStat
	// Holdout is the final holdout readout beside the base-rate predictor.
	Holdout *ValueStat
	// ByTurn is the holdout log loss and AUC by turn bucket 1–3 / 4–6 / 7+.
	ByTurn []ValueBucket
}

// valueOnlyTurnBuckets are the turn buckets the value-only readout splits
// by (the oracle dump's requested 1–3 / 4–6 / 7+). Deliberately NOT the PPO
// readout's valueTurnBuckets (1–6 / 7–12 / 13+), whose output and goldens
// must not move.
var valueOnlyTurnBuckets = []struct {
	name   string
	lo, hi int32
}{{"1-3", 1, 3}, {"4-6", 4, 6}, {"7+", 7, math.MaxInt32}}

func valueOnlyTurnBucket(turn int32) int {
	for i, b := range valueOnlyTurnBuckets {
		if turn >= b.lo && turn <= b.hi {
			return i
		}
	}
	return len(valueOnlyTurnBuckets) - 1
}

// valueSplitRNG is the value-only trainer's split/shuffle rng: seeded from
// the run seed, a PCG pair distinct from Train's main stream and valueRNG's,
// so a value-only run's split draws move nothing else.
func valueSplitRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), 0x76616c7565^0x6f6e6c79^uint64(seed)))
}

// splitBySeedBlock is the value-only holdout: records are grouped by Seed
// (the map is only a lookup, never ranged), the GROUP order is shuffled
// with the run's rng, and whole seed blocks are held out until at least
// frac·n records are held — the splitCorpusByGame shape with the seed as
// the unit, so no record from one seed can leak across train/holdout.
func splitBySeedBlock(recs []policynet.StateExample, frac float64, rng *rand.Rand) split {
	lookup := map[uint64]int{}
	var groups [][]int
	for i := range recs {
		g, ok := lookup[recs[i].Seed]
		if !ok {
			g = len(groups)
			lookup[recs[i].Seed] = g
			groups = append(groups, nil)
		}
		groups[g] = append(groups[g], i)
	}
	rng.Shuffle(len(groups), func(i, j int) { groups[i], groups[j] = groups[j], groups[i] })
	target := frac * float64(len(recs))
	var sp split
	for _, g := range groups {
		if float64(len(sp.hold)) < target {
			sp.hold = append(sp.hold, g...)
		} else {
			sp.train = append(sp.train, g...)
		}
	}
	return sp
}

// splitByRecord is the leaky per-record split (the splitCorpus shape over
// state records): first ⌈frac·n⌉ shuffled records held out. Used only by the
// tests to demonstrate what the seed block prevents.
func splitByRecord(recs []policynet.StateExample, frac float64, rng *rand.Rand) split {
	n := len(recs)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	rng.Shuffle(n, func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
	hn := int(frac * float64(n))
	return split{train: idx[hn:], hold: idx[:hn]}
}

// TrainValueOnly fits a value head to a state/outcome dump and returns the
// model with the per-epoch report. The corpus order is the loader's record
// order; the split and the per-epoch batch order are derived from cfg.Seed
// alone. The model's policy-only blocks are left at NewModel's init here —
// the CLI zeroes them before checkpointing (zeroPolicyHead) so the written
// value-only checkpoint cannot be mistaken for a trained policy read.
func TrainValueOnly(recs []policynet.StateExample, cfg ValueOnlyConfig) (*ValueOnlyResult, error) {
	switch {
	case len(recs) == 0:
		return nil, fmt.Errorf("policytrain: empty state dump")
	case cfg.Epochs < 1 || cfg.Batch < 1:
		return nil, fmt.Errorf("policytrain: epochs %d / batch %d < 1", cfg.Epochs, cfg.Batch)
	case cfg.LR <= 0:
		return nil, fmt.Errorf("policytrain: learning rate %g <= 0", cfg.LR)
	case cfg.Embed < 1 || cfg.Hidden < 1:
		return nil, fmt.Errorf("policytrain: geometry embed %d hidden %d", cfg.Embed, cfg.Hidden)
	case cfg.Holdout < 0 || cfg.Holdout >= 1:
		return nil, fmt.Errorf("policytrain: holdout fraction %g outside [0,1)", cfg.Holdout)
	case cfg.ValueHidden < 1:
		return nil, fmt.Errorf("policytrain: value hidden %d < 1 (the value-only run trains nothing else)", cfg.ValueHidden)
	}
	// Every record the loader returns has a known outcome (the loader skips
	// outcome_known false and counts it); the target range is re-checked here
	// so a hand-built caller cannot train a value head on a foreign target.
	for i := range recs {
		if recs[i].Outcome != 0 && recs[i].Outcome != 0.5 && recs[i].Outcome != 1 {
			return nil, fmt.Errorf("policytrain: record %d outcome %g outside {0, 0.5, 1}", i, recs[i].Outcome)
		}
	}
	usable := recs
	if len(usable) == 0 {
		return nil, fmt.Errorf("policytrain: no state record has a known outcome")
	}
	res := &ValueOnlyResult{}

	// The trainer's own rng stream (a distinct PCG pair from Train's and
	// valueRNG's, so the three trainers' same-seed runs draw different
	// sequences); the value head's init draws come from valueRNG (train.go),
	// the same separate stream the joint trainer uses.
	rng := valueSplitRNG(cfg.Seed)
	sp := splitBySeedBlock(usable, cfg.Holdout, rng)
	heldSeeds := map[uint64]bool{}
	for _, ix := range sp.hold {
		heldSeeds[usable[ix].Seed] = true
	}
	if len(sp.train) == 0 {
		return nil, fmt.Errorf("policytrain: holding out %g of %d records takes every seed block, leaving nothing to train on", cfg.Holdout, len(usable))
	}
	res.TrainN, res.HoldoutN, res.HoldoutSeeds = len(sp.train), len(sp.hold), len(heldSeeds)

	model := policynet.NewModel(policynet.TableRows, cfg.Embed, cfg.Hidden, rng)
	model.InitValue(cfg.ValueHidden, valueRNG(cfg.Seed))
	res.Model = model
	grads := model.NewGrads()

	// The base-rate predictor (train split's mean outcome) and the final
	// holdout readout, beside which the value head must earn its place.
	rate := 0.0
	for _, ix := range sp.train {
		rate += usable[ix].Outcome
	}
	rate /= float64(len(sp.train))
	vs := ValueStat{TrainN: len(sp.train), HoldoutN: len(sp.hold), BaseRate: rate}

	order := make([]int, len(sp.train))
	for epoch := 1; epoch <= cfg.Epochs; epoch++ {
		copy(order, sp.train)
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		es := ValueOnlyEpochStat{Epoch: epoch}
		for base := 0; base < len(order); base += cfg.Batch {
			end := min(base+cfg.Batch, len(order))
			grads.Reset()
			for _, ix := range order[base:end] {
				es.TrainLogLoss += model.ValueGrad(usable[ix].State, usable[ix].Outcome, grads)
			}
			if cfg.Clip > 0 {
				grads.Clip(cfg.Clip)
			}
			model.ApplyGrads(grads, float32(cfg.LR/float64(end-base)))
			if block, index, value, bad := firstNonFiniteParameter(model); bad {
				return nil, fmt.Errorf("policytrain: non-finite parameter after value-only update at epoch %d: %s[%d]=%g", epoch, block, index, value)
			}
		}
		es.TrainLogLoss /= float64(len(order))
		for _, ix := range sp.hold {
			es.HoldoutLogLoss += model.ValueLogLoss(usable[ix].State, usable[ix].Outcome)
		}
		if len(sp.hold) > 0 {
			es.HoldoutLogLoss /= float64(len(sp.hold))
		}
		res.Epochs = append(res.Epochs, es)
		if cfg.Log != nil {
			fmt.Fprintf(cfg.Log, "epoch %d/%d train bce %.6f | holdout log loss %.6f\n", epoch, cfg.Epochs, es.TrainLogLoss, es.HoldoutLogLoss)
		}
	}

	// Final holdout readout: log loss + Brier vs the base-rate predictor, and
	// the per-turn-bucket AUC readout on the same split.
	for _, ix := range sp.hold {
		v := float64(model.Value(usable[ix].State))
		t := usable[ix].Outcome
		vs.LogLoss += model.ValueLogLoss(usable[ix].State, t)
		vs.Brier += (v - t) * (v - t)
		vs.BaseLogLoss += bceProb(rate, t)
		vs.BaseBrier += (rate - t) * (rate - t)
	}
	if len(sp.hold) > 0 {
		n := float64(len(sp.hold))
		vs.LogLoss /= n
		vs.Brier /= n
		vs.BaseLogLoss /= n
		vs.BaseBrier /= n
	}
	res.Holdout = &vs
	res.ByTurn = valueOnlyByTurn(model, usable, sp)
	return res, nil
}

// valueOnlyByTurn is the final value head's holdout readout per turn bucket
// (the valueByTurn shape at the oracle dump's buckets; the bucket base rate
// is the train split's mean outcome in the bucket, so a head that only
// knows "late states are decided" gains nothing).
func valueOnlyByTurn(m *policynet.Model, recs []policynet.StateExample, sp split) []ValueBucket {
	nb := len(valueOnlyTurnBuckets)
	sum, cnt := make([]float64, nb), make([]int, nb)
	for _, ix := range sp.train {
		b := valueOnlyTurnBucket(recs[ix].Turn)
		sum[b] += recs[ix].Outcome
		cnt[b]++
	}
	out := make([]ValueBucket, nb)
	scores := make([][]float64, nb)
	wins := make([][]bool, nb)
	for b := range out {
		out[b].Turns = valueOnlyTurnBuckets[b].name
		out[b].BaseRate = 0.5
		if cnt[b] > 0 {
			out[b].BaseRate = sum[b] / float64(cnt[b])
		}
	}
	for _, ix := range sp.hold {
		r := recs[ix]
		b := valueOnlyTurnBucket(r.Turn)
		out[b].N++
		out[b].LogLoss += m.ValueLogLoss(r.State, r.Outcome)
		out[b].BaseLogLoss += bceProb(out[b].BaseRate, r.Outcome)
		if r.Outcome == 0 || r.Outcome == 1 {
			scores[b] = append(scores[b], float64(m.Value(r.State)))
			wins[b] = append(wins[b], r.Outcome == 1)
		}
	}
	for b := range out {
		if out[b].N > 0 {
			out[b].LogLoss /= float64(out[b].N)
			out[b].BaseLogLoss /= float64(out[b].N)
		}
		out[b].AUC = auc(scores[b], wins[b])
	}
	return out
}

// zeroPolicyHead writes the value-only checkpoint's POLICY read as zero
// before it is saved: the policy-only blocks (HidW, HidB, OutW, OutB,
// ResidualW). The shared trunk blocks (Table, StateW, StateB) are NOT
// zeroed — the value head trains them (ValueGrad's trunk backward), and the
// value head reads them. This is the deliberate checkpoint representation
// for pn17-a1's value consumer: the file is an ORDINARY policynet checkpoint
// (GPOL, schema 3, the feature set's encoder hash), so the consumer loads it
// with policynet.LoadCheckpointFile and reads Model.Value(state) exactly as
// it reads every existing value-bearing checkpoint; the zeroed policy read
// marks it as value-only in the file itself (every policy score is the same
// constant, argmax degenerate) rather than in a side channel. A diagnostic
// feature set reaches this path only under -oracle-checkpoint, and is then
// written through WriteOracleCheckpoint; WriteCheckpoint's rejection stays
// byte for byte.
func zeroPolicyHead(m *policynet.Model) {
	for i := range m.HidW {
		m.HidW[i] = 0
	}
	for i := range m.HidB {
		m.HidB[i] = 0
	}
	for i := range m.OutW {
		m.OutW[i] = 0
	}
	m.OutB = 0
	m.ResidualW = 0
}

// runValueOnly is the -value-corpus mode's CLI: load the state/outcome dump
// under the -features set, train the value head, print the holdout readout
// and — unless the feature set is diagnostic — write the value-only
// checkpoint (zeroPolicyHead).
func runValueOnly(corpora, out string, lo policynet.LoadOptions, cfg ValueOnlyConfig, stdout, stderr io.Writer) int {
	if out == "" {
		fmt.Fprintln(stderr, "policytrain: value-only mode needs -out")
		return 2
	}
	if lo.Joint {
		fmt.Fprintln(stderr, "policytrain: the joint action encoding is a policy encoding; the value-only trainer offers no options")
		return 2
	}
	if lo.Features == policynet.FeaturesEntity {
		fmt.Fprintln(stderr, "policytrain: the entity feature set is not supported by the value-only trainer (a value-only model carries no entity encoder)")
		return 2
	}
	if cfg.OracleCheckpoint && lo.Features != policynet.FeaturesMZOppHand {
		fmt.Fprintf(stderr, "policytrain: -oracle-checkpoint needs -features %s (the search leaf reads the opponent hand off the omniscient view; %s is not that set)\n", policynet.FeaturesMZOppHand, lo.Features)
		return 2
	}
	var recs []policynet.StateExample
	for _, path := range splitCorpora(corpora) {
		rs, stats, err := policynet.LoadStateOutcome(path, lo.Features)
		if err != nil {
			fmt.Fprintf(stderr, "policytrain: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "state dump %s: %d records, %d skipped (no outcome), %d loaded, features %s\n",
			path, stats.Records, stats.Skipped, stats.Loaded, lo.Features)
		recs = append(recs, rs...)
	}
	if len(recs) == 0 {
		fmt.Fprintln(stderr, "policytrain: no state record loaded")
		return 1
	}
	cfg.Log = stdout
	res, err := TrainValueOnly(recs, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 1
	}
	res.Model.Features = lo.Features
	final := res.Epochs[len(res.Epochs)-1]
	vs := res.Holdout
	fmt.Fprintf(stdout, "value-only training: %d train / %d holdout records (%d held-out seed blocks, split by seed block; the value head is hidden %d wide, features %s)\n",
		res.TrainN, res.HoldoutN, res.HoldoutSeeds, res.Model.ValueHidden, lo.Features)
	fmt.Fprintf(stdout, "  %-22s %10s %10s\n", "predictor", "log-loss", "brier")
	fmt.Fprintf(stdout, "  %-22s %10.6f %10.6f\n", "value head", vs.LogLoss, vs.Brier)
	fmt.Fprintf(stdout, "  %-22s %10.6f %10.6f\n", fmt.Sprintf("base rate %.4f", vs.BaseRate), vs.BaseLogLoss, vs.BaseBrier)
	fmt.Fprintf(stdout, "  holdout by turn (AUC of V(s) against won/lost; draws left out):\n")
	fmt.Fprintf(stdout, "  %-8s %6s %10s %10s %10s %8s\n", "turns", "n", "log-loss", "base-rate", "base-ll", "auc")
	for _, b := range res.ByTurn {
		fmt.Fprintf(stdout, "  %-8s %6d %10.4f %10.4f %10.4f %8.4f\n", b.Turns, b.N, b.LogLoss, b.BaseRate, b.BaseLogLoss, b.AUC)
	}
	fmt.Fprintf(stdout, "train %d records, holdout %d; final train bce %.6f, holdout log loss %.6f\n",
		res.TrainN, res.HoldoutN, final.TrainLogLoss, final.HoldoutLogLoss)
	if cfg.OracleCheckpoint {
		// The oracle checkpoint (pn17-a1): the diagnostic model, written
		// through the oracle writer only, so the ordinary loader still refuses
		// it and only a sampled-world search leaf can read it.
		zeroPolicyHead(res.Model)
		if err := res.Model.SaveOracleCheckpoint(out); err != nil {
			fmt.Fprintf(stderr, "policytrain: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "oracle checkpoint %s: value-only, features %s (policynet.LoadOracleCheckpointFile; botbench -search-oracle-checkpoint), encoder hash %#016x\n",
			out, lo.Features, policynet.EncoderHashFor(lo.Features))
		return 0
	}
	if lo.Features.Diagnostic() {
		// A diagnostic feature set reads hidden information: the run is a
		// measurement, never a checkpoint — WriteCheckpoint's rejection is
		// untouched and the diagnostic mode writes nothing here.
		fmt.Fprintf(stdout, "feature set %s is a measurement only: no checkpoint written\n", lo.Features)
		return 0
	}
	zeroPolicyHead(res.Model)
	if err := res.Model.SaveCheckpoint(out); err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 1
	}
	size := int64(-1)
	if info, err := os.Stat(out); err == nil {
		size = info.Size()
	}
	fmt.Fprintf(stdout, "checkpoint %s: value-only (policy read zeroed; pn17 consumer reads Model.Value), rows=%d h=%d hidden=%d value-hidden=%d (%d bytes), encoder hash %#016x\n",
		out, res.Model.Rows, res.Model.H, res.Model.Hidden, res.Model.ValueHidden, size, policynet.EncoderHashFor(res.Model.Features))
	return 0
}

// splitCorpora splits a comma-separated corpus list (trimming spaces).
func splitCorpora(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
