package policynet

import "math"

// VDWM — the value-disagreement-weighted margin loss (mtgbld's
// "novel synthesis", the pn13 addendum's second arm), trained on the same
// on-policy corpus as PPO. The label y is the RECORDED answer (the deployed
// seat's own play); the loss is a margin (hinge) on the logits instead of a
// probability ratio, weighted towards the rows where the old policy was
// uncertain and the outcome surprised the value head:
//
//	softmax kinds:  h = max(0, m − z_y + max_{j≠y} z_j),   w = (1 − p_old(y))·|A|
//	subset kinds:   h_k = max(0, m − t_k·z_k) per option,  w_k = (1 − q_k)·|A|
//	                t_k = +1 chosen / −1 not, q_k = σ(t_k·z_old,k)
//	loss = Σ w·h / Σ w over the batch
//
// A = outcome − V_old(s) enters UNSIGNED, so a row pulls toward its own
// recorded answer whether the game was won or lost: with y the seat's own
// argmax the margin term sharpens the current policy on the surprising,
// uncertain rows rather than steering it (see the ticket report). The
// trainer supplies |A| and the batch normalisation in PPOTarget.Weight
// (VDWMRowWeight is the Σ_k (1 − q_k) factor it normalises with); there is no
// KL anchor and no ratio. m ≤ 0 means DefaultVDWMMargin.

// DefaultVDWMMargin is mtgbld's margin m = 1.
const DefaultVDWMMargin = 1.0

// vdwmFactors returns, per labelled option (parallel to labelled), the
// (1 − q) disagreement factor of the recorded answer under π_old: for a
// softmax example one factor on the chosen option (1 − p_old(y)) and zero
// elsewhere; for a subset example 1 − σ(t_k·z_old,k) on every option. ok is
// false for a softmax example without exactly one chosen labelled option.
func vdwmFactors(ex Example, labelled []int) (f []float64, y int, ok bool) {
	p := ex.PPO
	f = make([]float64, len(labelled))
	y = -1
	if p.Subset {
		for k, i := range labelled {
			t := -1.0
			if p.Chosen[i] {
				t = 1
			}
			f[k] = 1 - sigmoidFloat(t*clampScore(float64(p.OldScores[i])))
		}
		return f, -1, true
	}
	zo := make([]float64, len(labelled))
	n := 0
	for k, i := range labelled {
		zo[k] = clampScore(float64(p.OldScores[i]))
		if p.Chosen[i] {
			y = k
			n++
		}
	}
	if n != 1 {
		return f, -1, false
	}
	f[y] = 1 - math.Exp(zo[y]-logSumExp(zo))
	return f, y, true
}

// VDWMRowWeight is Σ (1 − q) over the example's action space: the
// disagreement mass the trainer multiplies by |A| and normalises the batch
// with. 0 for a non-PPO example or an undefined softmax answer.
func VDWMRowWeight(ex Example) float64 {
	if ex.PPO == nil {
		return 0
	}
	var labelled []int
	for i := range ex.Options {
		if ex.Options[i].Target.Labelled {
			labelled = append(labelled, i)
		}
	}
	f, _, ok := vdwmFactors(ex, labelled)
	if !ok {
		return 0
	}
	s := 0.0
	for _, v := range f {
		s += v
	}
	return s
}

// lossVDWM is the VDWM term for one example, scaled by PPOTarget.Weight.
func lossVDWM(lc LossConfig, ex Example, labelled []int, ys []float64) (parts LossParts, dys []float64, st PPOStep) {
	n := len(labelled)
	dys = make([]float64, n)
	m := lc.VDWMMargin
	if m <= 0 {
		m = DefaultVDWMMargin
	}
	f, y, ok := vdwmFactors(ex, labelled)
	if !ok {
		return parts, dys, st
	}
	p := ex.PPO
	st.On = true
	w := p.Weight
	z := make([]float64, n)
	for k := range ys {
		z[k] = clampScore(ys[k])
	}
	if p.Subset {
		for k, i := range labelled {
			t := -1.0
			if p.Chosen[i] {
				t = 1
			}
			if h := m - t*z[k]; h > 0 {
				st.Margin += h
				st.MarginActive = true
				parts.Rank += w * f[k] * h
				dys[k] = -w * f[k] * t
			}
		}
	} else if n > 1 {
		best := -1
		for k := range z {
			if k != y && (best < 0 || z[k] > z[best]) {
				best = k
			}
		}
		if h := m - z[y] + z[best]; h > 0 {
			st.Margin = h
			st.MarginActive = true
			parts.Rank = w * f[y] * h
			dys[y] = -w * f[y]
			dys[best] = w * f[y]
		}
	}
	parts.Total = parts.Rank
	return parts, dys, st
}
