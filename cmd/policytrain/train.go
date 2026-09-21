package main

import (
	"fmt"
	"io"
	"math/rand/v2"
	"sort"

	"github.com/adams-shaun/gorge/decision"
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
	Epochs  int
	Batch   int
	LR      float64
	Seed    int64
	Holdout float64 // fraction of examples held out of the update, [0, 1)
	Embed   int     // H, the shared embedding width
	Hidden  int     // hidden layer width
	Mode    policynet.LossMode
	// RankWeight is the ranking term's weight against the value term (a
	// TERM-MIX weight, see policynet.LossConfig). In pure CE mode the rank
	// term is the whole loss, so a non-zero, non-one RankWeight is a uniform
	// scale on a single gradient direction: the per-batch clip normalises
	// it away whenever the cap binds (batches under the cap see it through
	// as an lr rescale on that batch), and with the clip disabled it is
	// equivalent to rescaling lr. Either way it cannot tune anything, so
	// Train WARNS when it is set in CE mode; RankWeight 0 (the off switch)
	// and 1 (the default) are the only settings with a distinct meaning
	// there. In hybrid mode the weight is a real direction knob and is never
	// warned about. Pin: TestCERankWeightInertWhileTheClipBinds.
	RankWeight float64
	HuberDelta float64
	// ResidualInit is the fixed weight of the bot-prior residual head
	// (policynet.Model.ResidualW): an option the bot's own answer contains
	// scores ResidualInit higher for the whole run, so with a positive value
	// the model starts at the bot baseline and the learned head only supplies
	// the overrides. 0 disables the residual (pure CE / hybrid / value). It is
	// a PRIOR, never trained, so it cannot be decayed away by the loss.
	ResidualInit float64
	// Clip is the per-batch GLOBAL-L2 cap on the summed gradient, applied
	// before the lr/batch scale (Grads.Clip; <= 0 disables; the CLI default
	// is 1). Direction-preserving by design: when the cap binds, every
	// uniform loss scale is normalised away and the step's worst-case norm
	// is lr·cap/batchSize (batches already under the cap see a uniform
	// scale through, as an lr rescale on that batch). The cap is not
	// cosmetic: an uncapped CE run drives an
	// exponential embedding→hidden→embedding feedback loop (backprop
	// through the hidden layer multiplies the two blocks' magnitudes into
	// each other), the weights overflow float32 within the epoch budget,
	// and the diverged model's NaN scores tie-break to the first option on
	// every decision — exactly the first-option baseline. Pin:
	// TestClipZeroDivergesToTheFirstOptionBaseline.
	Clip float64
	// OverrideWeight multiplies every example whose label records a teacher
	// override of the bot (Example.TeacherChoice != Example.BotIndex): the
	// decisions that carry information. 1 means no reweighting, >1 up-weights
	// the overrides, and <= 0 is treated as 1.
	OverrideWeight float64
	// KindModes overrides Mode for specific decision kinds. The attackers kind
	// is trained with the binary logistic loss (LossBCE) so its score level is
	// calibrated at 0, which is what the seat's per-option admission rule
	// reads; priority keeps argmax CE. A nil map applies Mode to every kind.
	// cmd/policytrain's default is {"attackers": LossBCE} under the global `ce`.
	KindModes map[decision.Kind]policynet.LossMode

	// ExtraW is the experimental per-option augmentation width (Option.Extra):
	// 0 reproduces the shipped encoder geometry byte for byte; a feature-family
	// experiment sets it to the width of the augmentation it built. Such a
	// model cannot be checkpointed (the format has no ExtraW field), which is
	// deliberate -- it is a measurement vehicle, not a deployable scorer.
	ExtraW int
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
	// ByKind is the honest per-decision-kind readout on the HOLDOUT split:
	// the model's top-1 agreement beside the three baselines computed from
	// the same split. Sorted by kind name. The blended EpochStat.Top1 mixes
	// kinds whose difficulty differs wildly (an attackers decision where a
	// first-index pick already agrees most of the time, a cast decision where
	// it does not), so it must be labelled as blended wherever it is shown.
	ByKind []KindStat
}

// KindStat is one decision kind's holdout agreement and the baselines it
// must beat. Eligible counts the examples with a teacher-preferred option
// (a decision with none can neither agree nor disagree).
type KindStat struct {
	Kind       decision.Kind
	Eligible   int
	ModelTop1  float64 // argmax over labelled options is teacher-preferred
	BotTop1    float64 // pick the bot's candidate: teacher kept the bot
	FirstTop1  float64 // pick the first labelled option
	RandomTop1 float64 // expected agreement of a uniform pick; #pref/#labelled averaged
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
	case cfg.ResidualInit < 0:
		return nil, fmt.Errorf("policytrain: residual init %g < 0 (a positive bot-prior weight is the residual; 0 disables)", cfg.ResidualInit)
	}
	mode, err := policynet.ParseLossMode(string(cfg.Mode))
	if err != nil {
		return nil, fmt.Errorf("policytrain: %v", err)
	}
	cfg.Mode = mode

	// The rank-weight/CE contract (ticket policytrain-clip-rankweight-
	// interaction): in pure CE mode a non-trivial RankWeight is a uniform
	// scale on the single gradient direction. Under an active per-batch clip
	// it is normalised away; with the clip DISABLED (-clip 0) it merely
	// rescales the step like lr would, so it is NOT inert there and is not
	// warned about. 0 (the off switch) and 1 (the default) are distinct and
	// never warned about either.
	if cfg.Mode == policynet.LossCE && cfg.Clip > 0 && cfg.RankWeight != 0 && cfg.RankWeight != 1 && cfg.Log != nil {
		fmt.Fprintf(cfg.Log, "policytrain: note: -rank-weight %g is inert in pure CE mode while the per-batch gradient clip is active — the clip normalises any uniform loss scale away whenever the cap binds (batches under the cap see it through as an lr rescale). Tune -lr and -clip instead; RankWeight is a term-mix weight and CE has one term.\n", cfg.RankWeight)
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

	model := policynet.NewModelExtra(policynet.TableRows, cfg.Embed, cfg.Hidden, cfg.ExtraW, rng)
	model.ResidualW = float32(cfg.ResidualInit)
	grads := model.NewGrads()
	lc := policynet.LossConfig{Mode: cfg.Mode, HuberDelta: cfg.HuberDelta, RankWeight: cfg.RankWeight, OverrideWeight: cfg.OverrideWeight, KindModes: cfg.KindModes}

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
			if cfg.Clip > 0 {
				grads.Clip(cfg.Clip)
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
	res.ByKind = evaluateByKind(model, usable, sp.hold, lc)
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

// evaluateByKind computes the honest per-kind holdout readout: the model's
// top-1 agreement and the bot-copy, first-labelled and random-pick baselines,
// all over the same eligible examples. An example is eligible when it has a
// teacher-preferred option (nothing to agree or disagree with otherwise).
// Kinds are visited in sorted order so the output never depends on map
// iteration order.
func evaluateByKind(m *policynet.Model, examples []policynet.Example, idx []int, lc policynet.LossConfig) []KindStat {
	type acc struct {
		eligible int
		model    int
		bot      int
		first    int
		random   float64
	}
	byKind := map[decision.Kind]*acc{}
	for _, ix := range idx {
		ex := examples[ix]
		labelled := make([]int, 0, len(ex.Options))
		for i := range ex.Options {
			if ex.Options[i].Target.Labelled {
				labelled = append(labelled, i)
			}
		}
		prefLabelled, prefTotal := 0, 0
		hasPref := false
		for _, i := range labelled {
			prefTotal++
			if ex.Options[i].Target.Preferred {
				prefLabelled++
				hasPref = true
			}
		}
		if len(labelled) == 0 || !hasPref {
			continue
		}
		a := byKind[ex.Kind]
		if a == nil {
			a = &acc{}
			byKind[ex.Kind] = a
		}
		a.eligible++
		st := m.Loss(ex, lc)
		if st.Agree {
			a.model++
		}
		if ex.TeacherChoice == ex.BotIndex {
			a.bot++
		}
		if ex.Options[labelled[0]].Target.Preferred {
			a.first++
		}
		a.random += ratio(prefLabelled, prefTotal)
	}
	kinds := make([]decision.Kind, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	out := make([]KindStat, 0, len(kinds))
	for _, k := range kinds {
		a := byKind[k]
		out = append(out, KindStat{
			Kind:       k,
			Eligible:   a.eligible,
			ModelTop1:  ratio(a.model, a.eligible),
			BotTop1:    ratio(a.bot, a.eligible),
			FirstTop1:  ratio(a.first, a.eligible),
			RandomTop1: a.random / float64(a.eligible),
		})
	}
	return out
}
