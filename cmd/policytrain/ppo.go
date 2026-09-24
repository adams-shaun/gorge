package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"slices"
	"sort"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// The on-policy PPO mode (ticket pn13, -ppo-corpus): fine-tune a checkpoint
// on its OWN games (cmd/botbench -onpolicy-corpus) with the clipped
// surrogate + KL anchor (internal/policynet/ppo.go). Per round:
//
//   - init: the checkpoint that played the corpus (-init). The trainer first
//     re-scores every recorded decision with it and compares against the
//     recorded scores: they must match bit for bit (the same encoded floats,
//     the same float32 forward), which proves the corpus is this checkpoint's
//     own play and makes π_old exact rather than a recorded approximation.
//   - advantage: outcome − V_old(s), V_old the init checkpoint's value head
//     (frozen for the round, so the baseline cannot chase its own targets);
//     "mean" baseline (outcome − the train split's mean outcome) when the
//     init has no value head. Normalised per batch (mean 0, sd 1) over the
//     batch's policy examples.
//   - value head: trained jointly on the same outcomes (BCE, -value-weight),
//     from the init's head (or a fresh one when the init has none).
//   - holdout: whole games (by pair/seed/game) for the value log loss and
//     the KL readouts; never updated on.
//
// Same flags + same corpus + same init ⇒ byte-identical checkpoint and stats
// (TestPPODeterministic). Like Train, it never reads the clock.

// PPOConfig is one PPO round's knobs.
type PPOConfig struct {
	Epochs  int
	Batch   int
	LR      float64
	Seed    int64
	Holdout float64 // by-game holdout fraction, [0,1)
	Clip    float64 // global L2 cap on the summed batch gradient (<= 0 off)
	PPOClip float64 // surrogate clip ε (<= 0: unclipped importance-weighted PG)
	PPOKL   float64 // KL anchor weight β
	// ValueWeight trains the value head on the outcomes (0 = off);
	// ValueHidden is the width of a FRESH head, used only when the init
	// checkpoint has none.
	ValueWeight float64
	ValueHidden int
	// AdvNorm normalises the advantages per batch.
	AdvNorm bool
	// Baseline is "value" (outcome − V_old(s); needs a value head on the
	// init) or "mean" (outcome − train mean outcome).
	Baseline string
	// Kinds restricts the policy update to these kinds (nil = every kind);
	// an excluded kind's examples still train the value head.
	Kinds []decision.Kind
	// VDWM trains the value-disagreement-weighted margin loss instead of
	// the PPO objective (policynet/vdwm.go; the pn13 addendum's second arm)
	// on the same corpus: weight (1 − p_old)·|outcome − baseline|, normalised
	// per batch; PPOClip/PPOKL/AdvNorm are then unused. VDWMMargin is m (≤ 0:
	// the default 1).
	VDWM       bool
	VDWMMargin float64
	Log        io.Writer
}

// PPO baselines.
const (
	BaselineValue = "value"
	BaselineMean  = "mean"
)

// PPOKindStat is one decision kind's corpus readout.
type PPOKindStat struct {
	Kind        decision.Kind `json:"kind"`
	N           int           `json:"n"`
	Deviated    int           `json:"deviated"`
	DeviatedPct float64       `json:"deviated_pct"`
	WinRate     float64       `json:"win_rate"`   // mean outcome
	MeanAdv     float64       `json:"mean_adv"`   // mean raw advantage (outcome − baseline)
	MeanPOld    float64       `json:"mean_p_old"` // mean π_old(chosen)
	// MeanDisagree is the mean VDWM disagreement mass Σ(1 − q) per decision
	// (policynet.VDWMRowWeight; 1 − p_old(y) for the softmax kinds) and
	// MeanAbsAdv the mean |outcome − baseline|: their product is the VDWM
	// row weight, so a kind whose MeanDisagree is ~0 cannot train under it.
	MeanDisagree float64 `json:"mean_disagree"`
	MeanAbsAdv   float64 `json:"mean_abs_adv"`
	PolicyOn     bool    `json:"policy_on"`  // the kind trains the policy term
	FinalKL      float64 `json:"final_kl"`   // mean KL(π_old‖π_final)
	FinalClip    float64 `json:"final_clip"` // share of examples with r outside [1−ε, 1+ε]
	// FinalFlip is the share of decisions whose answer under the final model
	// differs from π_old's: the argmax for the softmax kinds; for the subset
	// kinds, the seat's per-option admission vote (admittedSet, before the
	// seat's legality repair). FinalAdmits is the subset kinds' mean number of
	// options whose admission changed.
	FinalFlip   float64 `json:"final_flip"`
	FinalAdmits float64 `json:"final_admits"`
}

// PPOEpochStat is one epoch's training readout.
type PPOEpochStat struct {
	Epoch        int     `json:"epoch"`
	Surrogate    float64 `json:"surrogate"` // mean policy loss (surrogate + β·KL) over policy examples
	ClipFrac     float64 `json:"clip_frac"` // share of policy examples on the clipped branch
	KL           float64 `json:"kl"`        // mean KL(π_old‖π) at the step
	ValueLogLoss float64 `json:"value_log_loss"`
	// VDWM only: the share of policy examples whose margin was not yet
	// cleared, and their mean unweighted hinge.
	MarginActive float64 `json:"margin_active,omitempty"`
	Margin       float64 `json:"margin,omitempty"`
}

// PPOResult is one round's model and readouts.
type PPOResult struct {
	Model *policynet.Model `json:"-"`

	Examples      int     `json:"examples"`
	NoOutcome     int     `json:"no_outcome"` // dropped: stalled games
	TrainN        int     `json:"train_n"`
	HoldoutN      int     `json:"holdout_n"`
	PolicyN       int     `json:"policy_n"` // train examples with the policy term on
	Deviated      int     `json:"deviated"`
	ScoreMismatch int     `json:"score_mismatch"` // recorded vs init re-scored, options differing
	ScoreMaxDiff  float64 `json:"score_max_diff"`
	Baseline      string  `json:"baseline"`
	MeanAdv       float64 `json:"mean_adv"` // mean raw advantage over the train split

	ByKind []PPOKindStat   `json:"by_kind"`
	Epochs []PPOEpochStat  `json:"epochs"`
	Final  PPOFinalReadout `json:"final"`
}

// PPOFinalReadout compares the final model with π_old over every example.
type PPOFinalReadout struct {
	KL       float64 `json:"kl"`
	ClipFrac float64 `json:"clip_frac"`
	// The value head on the holdout split, before (init) and after, beside
	// the base-rate predictor (train mean outcome).
	ValueHoldoutN    int     `json:"value_holdout_n"`
	BaseRate         float64 `json:"base_rate"`
	BaseLogLoss      float64 `json:"base_log_loss"`
	InitValueLogLoss float64 `json:"init_value_log_loss"`
	ValueLogLoss     float64 `json:"value_log_loss"`
	// ValueByTurn splits the final value head's holdout quality by the
	// decision's turn (the view's per-player turn counter).
	ValueByTurn []ValueBucket `json:"value_by_turn"`
}

// ValueBucket is the value head's holdout quality over one turn bucket: log
// loss against the bucket's own base rate (the train split's mean outcome in
// the bucket, so a head that only knows "late states are decided" gains
// nothing), and the AUC of V(s) against won/lost (draws left out; 0.5 is no
// discrimination).
type ValueBucket struct {
	Turns       string  `json:"turns"`
	N           int     `json:"n"`
	LogLoss     float64 `json:"log_loss"`
	BaseRate    float64 `json:"base_rate"`
	BaseLogLoss float64 `json:"base_log_loss"`
	AUC         float64 `json:"auc"`
}

// valueTurnBuckets are the turn buckets the value readout splits by (the
// search-teacher coverage report's t1–6 / t7–12 / t13+).
var valueTurnBuckets = []struct {
	name   string
	lo, hi int32
}{{"1-6", 1, 6}, {"7-12", 7, 12}, {"13+", 13, math.MaxInt32}}

func turnBucket(turn int32) int {
	for i, b := range valueTurnBuckets {
		if turn >= b.lo && turn <= b.hi {
			return i
		}
	}
	return 0
}

// auc is the Mann-Whitney AUC of scores against binary labels (ties count
// half); 0.5 when either class is empty. Deterministic: a stable sort.
func auc(scores []float64, wins []bool) float64 {
	type sw struct {
		s float64
		w bool
	}
	xs := make([]sw, len(scores))
	pos, neg := 0, 0
	for i := range scores {
		xs[i] = sw{scores[i], wins[i]}
		if wins[i] {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return 0.5
	}
	sort.SliceStable(xs, func(i, j int) bool { return xs[i].s < xs[j].s })
	rankSum := 0.0
	for i := 0; i < len(xs); {
		j := i
		for j < len(xs) && xs[j].s == xs[i].s {
			j++
		}
		r := float64(i+j+1) / 2 // mean 1-based rank of the tie block
		for k := i; k < j; k++ {
			if xs[k].w {
				rankSum += r
			}
		}
		i = j
	}
	return (rankSum - float64(pos*(pos+1))/2) / float64(pos*neg)
}

// ppoDeviated reports whether the played answer differs from the default
// bot's (the BotPick marks) anywhere in the options.
func ppoDeviated(ex policynet.Example) bool {
	for i := range ex.Options {
		if ex.PPO.Chosen[i] != ex.Options[i].BotPick {
			return true
		}
	}
	return false
}

// TrainPPO runs one PPO round from init (mutated in place and returned).
func TrainPPO(examples []policynet.Example, init *policynet.Model, cfg PPOConfig) (*PPOResult, error) {
	switch {
	case init == nil:
		return nil, fmt.Errorf("policytrain: PPO needs an init checkpoint")
	case len(examples) == 0:
		return nil, fmt.Errorf("policytrain: empty on-policy corpus")
	case cfg.Epochs < 1 || cfg.Batch < 1:
		return nil, fmt.Errorf("policytrain: epochs %d / batch %d < 1", cfg.Epochs, cfg.Batch)
	case cfg.LR <= 0:
		return nil, fmt.Errorf("policytrain: learning rate %g <= 0", cfg.LR)
	case cfg.Holdout < 0 || cfg.Holdout >= 1:
		return nil, fmt.Errorf("policytrain: holdout fraction %g outside [0,1)", cfg.Holdout)
	case cfg.PPOClip < 0 || cfg.PPOKL < 0 || cfg.ValueWeight < 0:
		return nil, fmt.Errorf("policytrain: negative PPO clip %g / KL %g / value weight %g", cfg.PPOClip, cfg.PPOKL, cfg.ValueWeight)
	case cfg.Baseline != BaselineValue && cfg.Baseline != BaselineMean:
		return nil, fmt.Errorf("policytrain: PPO baseline %q (want %s or %s)", cfg.Baseline, BaselineValue, BaselineMean)
	case cfg.Baseline == BaselineValue && !init.HasValue():
		return nil, fmt.Errorf("policytrain: PPO baseline value needs a value head on the init checkpoint (use -ppo-baseline mean)")
	}
	m := init
	res := &PPOResult{Model: m, Baseline: cfg.Baseline}

	// Only examples with a known outcome can carry an advantage.
	usable := make([]policynet.Example, 0, len(examples))
	for i := range examples {
		if examples[i].PPO == nil {
			return nil, fmt.Errorf("policytrain: example %d has no PPO target", i)
		}
		if !examples[i].HasOutcome {
			res.NoOutcome++
			continue
		}
		usable = append(usable, examples[i])
	}
	res.Examples = len(usable)
	if len(usable) == 0 {
		return nil, fmt.Errorf("policytrain: no on-policy example has an outcome")
	}

	// π_old consistency: the init must reproduce the recorded scores.
	for i := range usable {
		ex := &usable[i]
		got := m.Score(ex.State, ex.Options)
		for k := range got {
			if got[k] != ex.PPO.OldScores[k] {
				res.ScoreMismatch++
				if d := math.Abs(float64(got[k]) - float64(ex.PPO.OldScores[k])); d > res.ScoreMaxDiff {
					res.ScoreMaxDiff = d
				}
			}
		}
	}
	if cfg.Log != nil {
		fmt.Fprintf(cfg.Log, "ppo: %d examples (%d dropped without outcome); init re-score mismatches %d options (max |diff| %g)\n",
			len(usable), res.NoOutcome, res.ScoreMismatch, res.ScoreMaxDiff)
	}

	rng := rand.New(rand.NewPCG(uint64(cfg.Seed), 0x9E3779B97F4A7C15^uint64(cfg.Seed)))
	sp := splitCorpusByGame(usable, cfg.Holdout, rng)
	if len(sp.train) == 0 {
		return nil, fmt.Errorf("policytrain: PPO holdout %g of %d examples leaves nothing to train on", cfg.Holdout, len(usable))
	}
	res.TrainN, res.HoldoutN = len(sp.train), len(sp.hold)

	if cfg.ValueWeight > 0 && !m.HasValue() {
		if cfg.ValueHidden < 1 {
			return nil, fmt.Errorf("policytrain: value weight %g on an init without a value head needs -value-hidden >= 1", cfg.ValueWeight)
		}
		m.InitValue(cfg.ValueHidden, valueRNG(cfg.Seed))
	}

	// Baselines and raw advantages (fixed for the round).
	baseRate := 0.0
	for _, ix := range sp.train {
		baseRate += usable[ix].Outcome
	}
	baseRate /= float64(len(sp.train))
	raw := make([]float64, len(usable))
	for i := range usable {
		b := baseRate
		if cfg.Baseline == BaselineValue {
			b = float64(init.Value(usable[i].State))
		}
		raw[i] = usable[i].Outcome - b
	}
	on := func(k decision.Kind) bool { return cfg.Kinds == nil || slices.Contains(cfg.Kinds, k) }
	for _, ix := range sp.train {
		usable[ix].PPO.PolicyOff = !on(usable[ix].Kind)
		if !usable[ix].PPO.PolicyOff {
			res.PolicyN++
		}
		res.MeanAdv += raw[ix]
	}
	res.MeanAdv /= float64(len(sp.train))
	for i := range usable {
		if ppoDeviated(usable[i]) {
			res.Deviated++
		}
	}

	// The value readout's holdout (before training).
	vHold := func() (n int, ll float64) {
		if !m.HasValue() {
			return 0, 0
		}
		for _, ix := range sp.hold {
			ll += m.ValueLogLoss(usable[ix].State, usable[ix].Outcome)
			n++
		}
		if n > 0 {
			ll /= float64(n)
		}
		return n, ll
	}
	res.Final.ValueHoldoutN, res.Final.InitValueLogLoss = vHold()
	res.Final.BaseRate = baseRate
	for _, ix := range sp.hold {
		res.Final.BaseLogLoss += bceProb(baseRate, usable[ix].Outcome)
	}
	if len(sp.hold) > 0 {
		res.Final.BaseLogLoss /= float64(len(sp.hold))
	}

	lc := policynet.LossConfig{Mode: policynet.LossCE, PPOClip: cfg.PPOClip, PPOKL: cfg.PPOKL, ValueWeight: cfg.ValueWeight, VDWMMargin: cfg.VDWMMargin}
	if cfg.VDWM {
		for _, ix := range sp.train {
			usable[ix].PPO.VDWM = true
		}
	}
	grads := m.NewGrads()
	order := make([]int, len(sp.train))
	for epoch := 1; epoch <= cfg.Epochs; epoch++ {
		copy(order, sp.train)
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		es := PPOEpochStat{Epoch: epoch}
		nPol, nVal := 0, 0
		for base := 0; base < len(order); base += cfg.Batch {
			end := min(base+cfg.Batch, len(order))
			batch := order[base:end]
			if cfg.VDWM {
				setVDWMWeights(usable, raw, batch)
			} else {
				setAdvantages(usable, raw, batch, cfg.AdvNorm)
			}
			grads.Reset()
			for _, ix := range batch {
				st := m.LossGrad(usable[ix], lc, grads)
				if st.PPO.On {
					nPol++
					es.Surrogate += st.Parts.Rank
					es.KL += st.PPO.KL
					es.Margin += st.PPO.Margin
					if st.PPO.MarginActive {
						es.MarginActive++
					}
					if st.PPO.Clipped {
						es.ClipFrac++
					}
				}
				if st.Parts.ValueHead != 0 {
					nVal++
					es.ValueLogLoss += st.Parts.ValueHead / cfg.ValueWeight
				}
			}
			if cfg.Clip > 0 {
				grads.Clip(cfg.Clip)
			}
			m.ApplyGrads(grads, float32(cfg.LR/float64(end-base)))
			if block, index, value, bad := firstNonFiniteParameter(m); bad {
				return nil, fmt.Errorf("policytrain: non-finite parameter after PPO update at epoch %d: %s[%d]=%g", epoch, block, index, value)
			}
		}
		if nPol > 0 {
			es.Surrogate /= float64(nPol)
			es.KL /= float64(nPol)
			es.ClipFrac /= float64(nPol)
			es.Margin /= float64(nPol)
			es.MarginActive /= float64(nPol)
		}
		if nVal > 0 {
			es.ValueLogLoss /= float64(nVal)
		}
		res.Epochs = append(res.Epochs, es)
		if cfg.Log != nil && cfg.VDWM {
			fmt.Fprintf(cfg.Log, "vdwm epoch %d/%d weighted margin %.6f active %.4f mean hinge %.4f value ll %.6f\n",
				epoch, cfg.Epochs, es.Surrogate, es.MarginActive, es.Margin, es.ValueLogLoss)
		} else if cfg.Log != nil {
			fmt.Fprintf(cfg.Log, "ppo epoch %d/%d surrogate %.6f clip %.4f kl %.6f value ll %.6f\n",
				epoch, cfg.Epochs, es.Surrogate, es.ClipFrac, es.KL, es.ValueLogLoss)
		}
	}
	_, res.Final.ValueLogLoss = vHold()
	if m.HasValue() {
		res.Final.ValueByTurn = valueByTurn(m, usable, sp)
	}

	// Final readout against π_old over every example, by kind.
	type acc struct {
		n, dev                     int
		out, adv, pold, kl         float64
		disagree, absAdv           float64
		clip, flip, flipN, admitsD float64
		admitsN                    int
	}
	byKind := map[decision.Kind]*acc{}
	nAll := 0
	for i := range usable {
		ex := usable[i]
		a := byKind[ex.Kind]
		if a == nil {
			a = &acc{}
			byKind[ex.Kind] = a
		}
		a.n++
		if ppoDeviated(ex) {
			a.dev++
		}
		a.out += ex.Outcome
		a.adv += raw[i]
		p := ex.PPO
		lpOld, _ := policynet.PPOLogProb(ex.Options, p.OldScores, p.Chosen, p.Subset)
		a.pold += math.Exp(lpOld)
		a.disagree += policynet.VDWMRowWeight(ex)
		a.absAdv += math.Abs(raw[i])
		probe := ex
		pc := *p
		pc.PolicyOff, pc.Advantage, pc.VDWM = false, 0, false
		probe.PPO = &pc
		st := m.Loss(probe, policynet.LossConfig{PPOClip: cfg.PPOClip})
		a.kl += st.PPO.KL
		res.Final.KL += st.PPO.KL
		if r := math.Exp(st.PPO.LogRatio); cfg.PPOClip > 0 && (r < 1-cfg.PPOClip || r > 1+cfg.PPOClip) {
			a.clip++
			res.Final.ClipFrac++
		}
		nAll++
		now := m.Score(ex.State, ex.Options)
		if p.Subset {
			before, after := admittedSet(p.OldScores, p.SignVote), admittedSet(now, p.SignVote)
			d := 0
			for k := range before {
				if before[k] != after[k] {
					d++
				}
			}
			a.admitsD += float64(d)
			a.admitsN++
			a.flipN++
			if d > 0 {
				a.flip++
			}
		} else {
			played := -1
			for k := range p.Chosen {
				if p.Chosen[k] {
					played = k
				}
			}
			best := -1
			for k := range now {
				if ex.Options[k].Target.Labelled && (best < 0 || now[k] > now[best]) {
					best = k
				}
			}
			a.flipN++
			if best != played {
				a.flip++
			}
		}
	}
	if nAll > 0 {
		res.Final.KL /= float64(nAll)
		res.Final.ClipFrac /= float64(nAll)
	}
	kinds := make([]decision.Kind, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	for _, k := range kinds {
		a := byKind[k]
		n := float64(a.n)
		ks := PPOKindStat{Kind: k, N: a.n, Deviated: a.dev, DeviatedPct: 100 * float64(a.dev) / n,
			WinRate: a.out / n, MeanAdv: a.adv / n, MeanPOld: a.pold / n, PolicyOn: on(k),
			MeanDisagree: a.disagree / n, MeanAbsAdv: a.absAdv / n,
			FinalKL: a.kl / n, FinalClip: a.clip / n}
		if a.flipN > 0 {
			ks.FinalFlip = a.flip / a.flipN
		}
		if a.admitsN > 0 {
			ks.FinalAdmits = a.admitsD / float64(a.admitsN)
		}
		res.ByKind = append(res.ByKind, ks)
	}
	return res, nil
}

// setAdvantages writes each batch example's advantage: the raw advantage,
// normalised over the batch's policy examples when norm is set (mean 0, sd 1;
// a batch whose advantages are all equal is only centred).
func setAdvantages(examples []policynet.Example, raw []float64, batch []int, norm bool) {
	if !norm {
		for _, ix := range batch {
			examples[ix].PPO.Advantage = raw[ix]
		}
		return
	}
	mean, n := 0.0, 0
	for _, ix := range batch {
		if !examples[ix].PPO.PolicyOff {
			mean += raw[ix]
			n++
		}
	}
	if n == 0 {
		return
	}
	mean /= float64(n)
	v := 0.0
	for _, ix := range batch {
		if !examples[ix].PPO.PolicyOff {
			v += (raw[ix] - mean) * (raw[ix] - mean)
		}
	}
	sd := math.Sqrt(v / float64(n))
	for _, ix := range batch {
		a := raw[ix] - mean
		if sd > 1e-8 {
			a /= sd
		}
		examples[ix].PPO.Advantage = a
	}
}

// writePPOStats writes the round's readout as indented JSON.
func writePPOStats(path string, res *PPOResult) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// admittedSet is the subset seats' per-option admission vote
// (seat/policynet.go admissionThreshold, restated: cmd/policytrain reads
// scores, not decisions): the calibrated boundary 0 (strict) when the scores
// straddle it, else the decision's own mean (inclusive); an exactly tied
// multi-option decision admits nothing here (the seat delegates it). It is a
// readout of how many played answers a round's update would change, before
// the seat's one-option-per-attacker and requirement repair. sign selects
// the seat's opt-in sign vote (seat.AdmissionSign): score > 0 alone.
func admittedSet(scores []float32, sign bool) []bool {
	out := make([]bool, len(scores))
	if len(scores) == 0 {
		return out
	}
	if sign {
		for i, s := range scores {
			out[i] = s > 0
		}
		return out
	}
	tied := true
	pos, neg := 0, 0
	sum := 0.0
	for _, s := range scores {
		if s != scores[0] {
			tied = false
		}
		if s > 0 {
			pos++
		} else {
			neg++
		}
		sum += float64(s)
	}
	if len(scores) == 1 || (pos > 0 && neg > 0) {
		for i, s := range scores {
			out[i] = s > 0
		}
		return out
	}
	if tied {
		return out
	}
	mean := float32(sum / float64(len(scores)))
	for i, s := range scores {
		out[i] = s >= mean
	}
	return out
}

// setVDWMWeights writes each batch example's VDWM scale: |raw advantage| ×
// n / W, W = Σ |A|·VDWMRowWeight over the batch's policy examples, so the
// batch step (ApplyGrads' lr/n) descends Σ w·h / Σ w exactly. A batch with no
// weight trains no margin term.
func setVDWMWeights(examples []policynet.Example, raw []float64, batch []int) {
	w := 0.0
	for _, ix := range batch {
		if !examples[ix].PPO.PolicyOff {
			w += math.Abs(raw[ix]) * policynet.VDWMRowWeight(examples[ix])
		}
	}
	for _, ix := range batch {
		examples[ix].PPO.Weight = 0
		if w > 0 {
			examples[ix].PPO.Weight = math.Abs(raw[ix]) * float64(len(batch)) / w
		}
	}
}

// valueByTurn is the final value head's holdout readout per turn bucket.
func valueByTurn(m *policynet.Model, examples []policynet.Example, sp split) []ValueBucket {
	nb := len(valueTurnBuckets)
	sum, cnt := make([]float64, nb), make([]int, nb)
	for _, ix := range sp.train {
		b := turnBucket(examples[ix].Turn)
		sum[b] += examples[ix].Outcome
		cnt[b]++
	}
	out := make([]ValueBucket, nb)
	scores := make([][]float64, nb)
	wins := make([][]bool, nb)
	for b := range out {
		out[b].Turns = valueTurnBuckets[b].name
		out[b].BaseRate = 0.5
		if cnt[b] > 0 {
			out[b].BaseRate = sum[b] / float64(cnt[b])
		}
	}
	for _, ix := range sp.hold {
		ex := examples[ix]
		b := turnBucket(ex.Turn)
		out[b].N++
		out[b].LogLoss += m.ValueLogLoss(ex.State, ex.Outcome)
		out[b].BaseLogLoss += bceProb(out[b].BaseRate, ex.Outcome)
		if ex.Outcome == 0 || ex.Outcome == 1 {
			scores[b] = append(scores[b], float64(m.Value(ex.State)))
			wins[b] = append(wins[b], ex.Outcome == 1)
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
