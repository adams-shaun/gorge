package main

import (
	"fmt"
	"io"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/internal/policynet"
)

// The training loop for the per-option scorer (plan L9b). Deliberately
// SINGLE-THREADED and deterministic (R1 §3.3): a Hogwild-style sharded
// update is a data race under the Go memory model, and parallel float
// accumulation is order-dependent, so a multi-worker trainer would produce a
// checkpoint that cannot be rebuilt byte-for-byte from its seed and corpus —
// a foreign object in this repo. The -workers idea was considered and
// REJECTED: sharding only the forward pass would keep bit-identity only by
// re-serialising the reduction, and the single-threaded step measured at the
// production geometry (4,201 examples/s, BenchmarkLossGradStep) is not the
// bottleneck for a 14.6k-decision corpus (≈3.5 s/epoch).
//
// The wall clock is deliberately NOT read here (spec D16's clock discipline,
// the cmd/policytune precedent): wall/throughput figures for the report
// come from the test/benchmark harness, never from the trainer, so no run's
// timing can reach a checkpoint byte. Everything here is plain sequential
// float32 SGD over shuffled batches; the only randomness is the
// caller-seeded math/rand/v2 source, and the shuffle order is part of the
// seed's contract (same seed + same corpus ⇒ byte-identical checkpoint,
// pinned by TestTrainerDeterministic).

// Config is one training run's knobs — the cmd/policytrain flag surface.
type Config struct {
	Epochs     int
	Batch      int
	LR         float64
	Seed       int64
	Holdout    float64 // fraction of examples held out of the update, [0, 1)
	Embed      int     // H, the shared embedding width
	Hidden     int     // hidden layer width
	RankWeight float64
	HuberDelta float64
	// Log receives one line per epoch (nil discards).
	Log io.Writer
}

// EpochStat is one epoch's reported readout.
type EpochStat struct {
	Epoch       int
	TrainLoss   float64 // mean combined loss over the training examples
	TrainTop1   float64 // argmax-in-preferred agreement over eligible examples
	HoldoutLoss float64
	HoldoutTop1 float64
}

// Result carries the trained model and the run's bookkeeping.
type Result struct {
	Model    *policynet.Model
	Epochs   []EpochStat
	TrainN   int
	HoldoutN int
	Skipped  int // examples with no labelled option (never trained on)
}

type split struct {
	train []int
	hold  []int
}

// Train fits a Model to examples and returns it with the per-epoch report.
// The corpus order is the caller's (the loader's record order); the split
// and the per-epoch batch order are derived from cfg.Seed alone.
func Train(examples []policynet.Example, cfg Config) (*Result, error) {
	switch {
	case len(examples) == 0:
		return nil, fmt.Errorf("policytrain: empty corpus")
	case cfg.Epochs < 1:
		return nil, fmt.Errorf("policytrain: epochs %d < 1", cfg.Epochs)
	case cfg.Batch < 1:
		return nil, fmt.Errorf("policytrain: batch %d < 1", cfg.Batch)
	case cfg.LR <= 0:
		return nil, fmt.Errorf("policytrain: learning rate %g <= 0", cfg.LR)
	case cfg.Embed < 1 || cfg.Hidden < 1:
		return nil, fmt.Errorf("policytrain: geometry embed %d hidden %d", cfg.Embed, cfg.Hidden)
	case cfg.Holdout < 0 || cfg.Holdout >= 1:
		return nil, fmt.Errorf("policytrain: holdout fraction %g outside [0,1)", cfg.Holdout)
	}

	// Examples with no labelled option cannot contribute to either loss
	// term; drop them up front and count them.
	usable := make([]policynet.Example, 0, len(examples))
	skipped := 0
	for i := range examples {
		labelled := false
		for j := range examples[i].Options {
			if examples[i].Options[j].Target.Labelled {
				labelled = true
				break
			}
		}
		if labelled {
			usable = append(usable, examples[i])
		} else {
			skipped++
		}
	}
	if len(usable) == 0 {
		return nil, fmt.Errorf("policytrain: corpus has no labelled example")
	}

	// math/rand/v2 with an explicit seeded source (the repo forbids v1; the
	// PCG seed pair is a fixed deterministic function of cfg.Seed).
	rng := rand.New(rand.NewPCG(uint64(cfg.Seed), 0x9E3779B97F4A7C15^uint64(cfg.Seed)))
	sp := splitCorpus(usable, cfg.Holdout, rng)

	model := policynet.NewModel(policynet.TableRows, cfg.Embed, cfg.Hidden, rng)
	grads := model.NewGrads()
	lc := policynet.LossConfig{HuberDelta: cfg.HuberDelta, RankWeight: cfg.RankWeight}

	res := &Result{Model: model, TrainN: len(sp.train), HoldoutN: len(sp.hold), Skipped: skipped}
	order := make([]int, len(sp.train))
	for epoch := 1; epoch <= cfg.Epochs; epoch++ {
		copy(order, sp.train)
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })

		sum, agree, eligible := 0.0, 0, 0
		for base := 0; base < len(order); base += cfg.Batch {
			end := base + cfg.Batch
			if end > len(order) {
				end = len(order)
			}
			grads.Reset()
			batchLoss := 0.0
			for _, ix := range order[base:end] {
				st := model.LossGrad(usable[ix], lc, grads)
				batchLoss += st.Loss
				if st.Eligible {
					eligible++
					if st.Agree {
						agree++
					}
				}
			}
			model.ApplyGrads(grads, float32(cfg.LR/float64(end-base)))
			sum += batchLoss / float64(end-base)
		}
		batches := (len(order) + cfg.Batch - 1) / cfg.Batch
		trainLoss := sum / float64(batches)
		trainTop1 := ratio(agree, eligible)

		holdLoss, holdTop1 := evaluate(model, usable, sp.hold, lc)
		res.Epochs = append(res.Epochs, EpochStat{
			Epoch: epoch, TrainLoss: trainLoss, TrainTop1: trainTop1,
			HoldoutLoss: holdLoss, HoldoutTop1: holdTop1,
		})
		if cfg.Log != nil {
			fmt.Fprintf(cfg.Log, "epoch %d/%d train loss %.6f top1 %.3f | holdout loss %.6f top1 %.3f\n",
				epoch, cfg.Epochs, trainLoss, trainTop1, holdLoss, holdTop1)
		}
	}
	return res, nil
}

// splitCorpus shuffles the index space with the run's rng and takes the
// first ⌈frac·n⌉ as holdout. Called in the fixed rng sequence (before the
// model's init draws), it is deterministic given the seed.
func splitCorpus(examples []policynet.Example, frac float64, rng *rand.Rand) split {
	n := len(examples)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	rng.Shuffle(n, func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })
	hn := int(frac * float64(n))
	return split{train: idx[hn:], hold: idx[:hn]}
}

// evaluate runs the forward-only loss over a set of examples.
func evaluate(m *policynet.Model, examples []policynet.Example, idx []int, lc policynet.LossConfig) (loss float64, top1 float64) {
	if len(idx) == 0 {
		return 0, 0
	}
	sum, agree, eligible := 0.0, 0, 0
	for _, ix := range idx {
		st := m.Loss(examples[ix], lc)
		sum += st.Loss
		if st.Eligible {
			eligible++
			if st.Agree {
				agree++
			}
		}
	}
	return sum / float64(len(idx)), ratio(agree, eligible)
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
