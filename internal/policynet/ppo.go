package policynet

import "math"

// This file is the on-policy PPO objective (ticket pn13): the clipped
// surrogate plus a KL anchor, trained on the DEPLOYED seat's own games
// (seat.PolicyNetBot recording, cmd/botbench -onpolicy-corpus). It is the
// gorge port of the one multi-round recipe that compounded in mtgbld
// ("self-distillation PPO"): corpus = the argmax policy's own decisions,
// π_old = the probability the policy gave the answer it actually played,
// advantage = outcome − V(s) normalised per batch, small steps.
//
// A PPO example carries its target in Example.PPO; the supervised path
// (lossFromScores) never sees one, so a corpus without PPO examples trains
// exactly as before (cmd/policytrain's TestSupervisedIsBitIdenticalToPrePPOTrainer).
//
// The action distribution is defined over the example's LABELLED options
// (the loader marks the decision's action space Labelled) and takes one of
// two shapes, matching the two scored inference paths in seat/policynet.go:
//
//   - softmax (priority, target): π(a|s) = softmax(z)_a over the action
//     space; the seat plays its argmax.
//   - subset (attackers, blockers): π(C|s) = Π_i σ(z_i)^[i∈C]·(1−σ(z_i))^[i∉C],
//     independent per-option inclusion (the per-option BCE contract the
//     subset heads are trained under); the seat plays its admission vote.
//
// z is the FULL score the seat reads: the head output plus the residual
// bot-prior (Model.ResidualW on BotPick options). The residual is a fixed
// offset, so it enters π_old and π_new identically and only the head moves.
//
// The loss for one example with advantage A, log-ratio ρ = log π_new(C) −
// log π_old(C) and r = exp(ρ):
//
//	L = −min(r·A, clip(r, 1−ε, 1+ε)·A) + β·KL(π_old ‖ π_new)
//
// The KL is exact, not a sample estimate: over the softmax's action space,
// Σ_k p_old,k·(log p_old,k − log p_new,k), with gradient p_new − p_old in the
// logits; for the subset shape, the sum of the per-option Bernoulli KLs, with
// gradient σ(z_new) − σ(z_old) per option. ρ is clamped to ±PPOLogRatioClamp
// before exp (a clamped ratio carries no gradient, like the clip).

// PPOLogRatioClamp bounds the log-ratio before exponentiation (ratio in
// [e^-3, e^3] ≈ [0.05, 20], the mtgbld trainer's ratio clamp).
const PPOLogRatioClamp = 3.0

// PPOTarget is one on-policy decision's PPO target: what the deployed policy
// scored and played, and the advantage the trainer assigns it.
type PPOTarget struct {
	// Subset selects the per-option Bernoulli distribution (attackers,
	// blockers); false is the softmax over the action space (priority,
	// target).
	Subset bool
	// OldScores parallels Example.Options: the full scores (head + residual)
	// π_old gave every option when the seat decided. Only the labelled
	// (action-space) entries are read.
	OldScores []float32
	// Chosen parallels Example.Options: the options the seat's answer
	// contained. A softmax example needs exactly one chosen labelled option,
	// or it trains no policy term.
	Chosen []bool
	// Advantage is the example's (normalised) advantage, set by the trainer
	// before the example is used.
	Advantage float64
	// PolicyOff silences the policy term (surrogate and KL) for this example:
	// a kind the run excludes from the PPO update still trains the value head
	// on its outcome.
	PolicyOff bool
	// SignVote records that the seat answered this subset decision with the
	// sign admission vote (seat.AdmissionSign) rather than the default auto
	// vote. It changes no loss term — the subset π is the per-option
	// Bernoulli either way — only the trainer's answer-change readout.
	SignVote bool
	// VDWM switches the example from the PPO objective to the
	// value-disagreement-weighted margin loss (vdwm.go); Weight is then its
	// per-example scale, |outcome − V_old(s)| times the trainer's batch
	// normalisation.
	VDWM   bool
	Weight float64
}

// PPOStep is one PPO example's readout (zero for a supervised example).
type PPOStep struct {
	On       bool    // the example trained a PPO policy term
	LogRatio float64 // log π_new(C) − log π_old(C), unclamped
	Clipped  bool    // the clipped branch of the surrogate was the min
	KL       float64 // exact KL(π_old ‖ π_new) over the action space
	LogPOld  float64 // log π_old(C)
	// VDWM readouts (vdwm.go): the unweighted hinge total and whether any
	// hinge was active (the margin not yet cleared).
	Margin       float64
	MarginActive bool
}

// logSigmoid is log σ(x) = −softplus(−x), stable for any finite x.
func logSigmoid(x float64) float64 { return -softplus(-x) }

// PPOLogProb returns log π(C|s) for scores parallel to the options, reading
// only the labelled options: the softmax shape's log-probability of the one
// chosen option, or the subset shape's summed per-option Bernoulli
// log-likelihood. ok is false for a softmax decision that does not have
// exactly one chosen labelled option (its probability is undefined here).
// Shared by the loss, the corpus writer's π_old and the tests.
func PPOLogProb(opts []Option, scores []float32, chosen []bool, subset bool) (lp float64, ok bool) {
	var z []float64
	a, nChosen := -1, 0
	var c []bool
	for i := range opts {
		if !opts[i].Target.Labelled {
			continue
		}
		if chosen[i] {
			a = len(z)
			nChosen++
		}
		z = append(z, clampScore(float64(scores[i])))
		c = append(c, chosen[i])
	}
	if len(z) == 0 {
		return 0, false
	}
	if subset {
		for k := range z {
			if c[k] {
				lp += logSigmoid(z[k])
			} else {
				lp += logSigmoid(-z[k])
			}
		}
		return lp, true
	}
	if nChosen != 1 {
		return 0, false
	}
	return z[a] - logSumExp(z), true
}

// lossPPO is the PPO objective for one example (see the file comment). ys
// are the current full scores of the labelled options (forwardExample's).
func lossPPO(lc LossConfig, ex Example, labelled []int, ys []float64) (parts LossParts, dys []float64, st PPOStep) {
	n := len(labelled)
	dys = make([]float64, n)
	p := ex.PPO
	if p.PolicyOff {
		return parts, dys, st
	}
	if p.VDWM {
		return lossVDWM(lc, ex, labelled, ys)
	}
	z := make([]float64, n)
	zo := make([]float64, n)
	c := make([]bool, n)
	a, nChosen := -1, 0
	for k, i := range labelled {
		z[k] = clampScore(ys[k])
		zo[k] = clampScore(float64(p.OldScores[i]))
		c[k] = p.Chosen[i]
		if c[k] {
			a = k
			nChosen++
		}
	}
	dlp := make([]float64, n) // ∂ log π_new(C) / ∂z
	dkl := make([]float64, n) // ∂ KL(π_old ‖ π_new) / ∂z
	var lpNew, lpOld, kl float64
	if p.Subset {
		for k := range z {
			sn, so := sigmoidFloat(z[k]), sigmoidFloat(zo[k])
			if c[k] {
				lpNew += logSigmoid(z[k])
				lpOld += logSigmoid(zo[k])
				dlp[k] = 1 - sn
			} else {
				lpNew += logSigmoid(-z[k])
				lpOld += logSigmoid(-zo[k])
				dlp[k] = -sn
			}
			kl += so*(logSigmoid(zo[k])-logSigmoid(z[k])) + (1-so)*(logSigmoid(-zo[k])-logSigmoid(-z[k]))
			dkl[k] = sn - so
		}
	} else {
		if nChosen != 1 || n < 1 {
			return parts, dys, st
		}
		lseN, lseO := logSumExp(z), logSumExp(zo)
		lpNew, lpOld = z[a]-lseN, zo[a]-lseO
		for k := range z {
			pn := math.Exp(z[k] - lseN)
			po := math.Exp(zo[k] - lseO)
			kl += po * ((zo[k] - lseO) - (z[k] - lseN))
			dkl[k] = pn - po
			dlp[k] = -pn
		}
		dlp[a] += 1
	}
	st.On = true
	st.LogPOld = lpOld
	st.LogRatio = lpNew - lpOld
	st.KL = kl

	rho := st.LogRatio
	rhoGrad := 1.0
	if rho > PPOLogRatioClamp {
		rho, rhoGrad = PPOLogRatioClamp, 0
	} else if rho < -PPOLogRatioClamp {
		rho, rhoGrad = -PPOLogRatioClamp, 0
	}
	r := math.Exp(rho)
	adv := p.Advantage
	s1 := r * adv
	s2 := s1
	if eps := lc.PPOClip; eps > 0 {
		s2 = math.Min(math.Max(r, 1-eps), 1+eps) * adv
	}
	var pg, dpg float64 // surrogate loss and its derivative in log π_new
	if s1 <= s2 {
		pg = -s1
		dpg = -s1 * rhoGrad // d(−r·A)/dρ = −r·A
	} else {
		pg = -s2
		st.Clipped = true
	}
	for k := range dys {
		dys[k] = dpg*dlp[k] + lc.PPOKL*dkl[k]
	}
	parts.Rank = pg + lc.PPOKL*kl
	parts.Total = parts.Rank
	return parts, dys, st
}
