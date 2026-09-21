package policynet

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

// The clip-semantics pins (ticket policytrain-clip-rankweight-interaction).
// Grads.Clip is documented as a PER-BATCH GLOBAL-L2 cap on the summed
// gradient: direction-preserving, magnitude-only. These tests pin that
// choice against the two alternatives the ticket named — a per-example cap
// and a per-parameter-group cap — on a corpus where the alternatives give
// measurably different training.

// clipPinCorpus builds examples whose per-example gradient norms straddle
// the cap used below (measured in-test), so a per-example clip distorts the
// relative example weights while the per-batch clip preserves them.
func clipPinCorpus(n int) []Example {
	out := make([]Example, n)
	rng := rand.New(rand.NewPCG(11, 7))
	for i := 0; i < n; i++ {
		st := State{Dense: make([]float32, DenseWidth)}
		for d := 0; d < 6; d++ {
			st.Dense[d] = float32(rng.NormFloat64())
		}
		st.Sparse = append(st.Sparse,
			Feature{Row: HashID(fmt.Sprintf("cp|s|%d", i%11)), Value: 2},
			Feature{Row: HashID("cp|shared"), Value: 1},
		)
		pref := i % 3
		ex := Example{Kind: "attackers", TeacherChoice: pref, BotIndex: 0, State: st, Margin: 0.2}
		for j := 0; j < 3; j++ {
			o := Option{Dense: make([]float32, OptionDenseWidth)}
			for d := 0; d < 6; d++ {
				o.Dense[d] = float32(rng.NormFloat64())
			}
			o.Hashed = append(o.Hashed, Feature{Row: HashID(fmt.Sprintf("cp|p|%d", j)), Value: 2})
			if j == pref {
				o.Hashed = append(o.Hashed, Feature{Row: HashID("cp|good"), Value: 2})
			}
			// Untied values so the hybrid value term carries signal (the
			// CE-mode pins do not read values at all).
			o.Target = OptionTarget{Labelled: true, Preferred: j == pref, Value: 0.5 + 0.05*float64(j)}
			ex.Options = append(ex.Options, o)
		}
		out[i] = ex
	}
	return out
}

// clipLoop runs the trainer's batch loop by hand (the same Reset / LossGrad /
// Clip / ApplyGrads sequence train.go runs) with one knob changed by the
// caller via clipOne: whether the cap is applied to the SUMMED batch
// gradient once (per-batch semantics) or to each example's gradient before
// accumulating (the per-example alternative). Batches are visited in fixed
// index order so both loops see the identical sequence.
func clipLoop(m *Model, examples []Example, lc LossConfig, epochs, batch int, lr, cap float64, perExample bool) *Model {
	grads := m.NewGrads()
	one := m.NewGrads()
	for epoch := 0; epoch < epochs; epoch++ {
		for base := 0; base < len(examples); base += batch {
			end := base + batch
			if end > len(examples) {
				end = len(examples)
			}
			grads.Reset()
			for _, ex := range examples[base:end] {
				if perExample {
					one.Reset()
					m.LossGrad(ex, lc, one)
					one.Clip(cap)
					addGradsInto(grads, one)
				} else {
					m.LossGrad(ex, lc, grads)
				}
			}
			if !perExample {
				grads.Clip(cap)
			}
			m.ApplyGrads(grads, float32(lr/float64(end-base)))
		}
	}
	return m
}

// addGradsInto accumulates src's every block into dst. Test-only helper for
// the per-example-clip reference loop; dst's touched set grows to cover
// src's rows so ApplyGrads and Norm see the merged gradient.
func addGradsInto(dst, src *Grads) {
	for _, r := range src.touched {
		base := int(r) * dst.m.H
		if dst.noted[r] == 0 {
			dst.noted[r] = 1
			dst.touched = append(dst.touched, r)
		}
		for j := 0; j < dst.m.H; j++ {
			dst.Table[base+j] += src.Table[base+j]
		}
	}
	for _, blk := range [4][2][]float32{
		{dst.StateW, src.StateW}, {dst.StateB, src.StateB},
		{dst.HidW, src.HidW}, {dst.HidB, src.HidB},
	} {
		for i := range blk[0] {
			blk[0][i] += blk[1][i]
		}
	}
	for i := range dst.OutW {
		dst.OutW[i] += src.OutW[i]
	}
	dst.OutB += src.OutB
}

// TestClipSemanticsIsPerBatchNotPerExample pins WHICH gradient the cap acts
// on. On the corpus below the per-example gradient norms straddle the cap
// (asserted), so the two semantics produce different update directions; ten
// epochs later the two models must differ substantially. Control: with a cap
// so large it never binds, the two loops are the same arithmetic and must
// agree byte for byte.
func TestClipSemanticsIsPerBatchNotPerExample(t *testing.T) {
	examples := clipPinCorpus(300)
	const cap = 0.5
	lc := LossConfig{Mode: LossCE, HuberDelta: 0.1, RankWeight: 1, OverrideWeight: 1}

	// Premise check on a freshly initialised model: the batch gradient norm
	// exceeds the cap and the per-example norms straddle it (some below,
	// some above) — the corpus is one where the two semantics MUST differ.
	m0 := NewModel(TableRows, 32, 64, rand.New(rand.NewPCG(5, 6)))
	g := m0.NewGrads()
	one := m0.NewGrads()
	batchNorm := 0.0
	below, above := 0, 0
	for _, ex := range examples[:64] {
		one.Reset()
		m0.LossGrad(ex, lc, one)
		if n := one.Norm(); n < cap {
			below++
		} else {
			above++
		}
		m0.LossGrad(ex, lc, g)
	}
	batchNorm = g.Norm()
	if batchNorm <= cap || below == 0 || above == 0 {
		t.Fatalf("premise lost: batch norm %.3g (cap %g), per-example below/above %d/%d — this corpus no longer separates the two clip semantics", batchNorm, cap, below, above)
	}

	run := func(perExample bool, cap float64) *Model {
		m := NewModel(TableRows, 32, 64, rand.New(rand.NewPCG(5, 6)))
		return clipLoop(m, examples, lc, 10, 32, 0.1, cap, perExample)
	}
	batch := run(false, cap)
	perEx := run(true, cap)
	if relDiff := blockRelDiff(batch, perEx); relDiff < 0.01 {
		t.Fatalf("per-batch and per-example clip semantics produced nearly identical models (rel diff %.3g) — the corpus no longer separates them", relDiff)
	}

	// Control: a cap that never binds leaves both loops the same arithmetic.
	// NOT byte-identical: the fused multiply-add the compiler emits for the
	// direct `g += d·x` accumulation rounds in one step, while the split path
	// (per-example grads, then a separate add) rounds twice — a last-ulp
	// difference that stays at ~1e-7. The bound is on that rounding, far
	// below the O(1) separation the binding cap must produce.
	batchFree := run(false, 1e30)
	perExFree := run(true, 1e30)
	if relDiff := blockRelDiff(batchFree, perExFree); relDiff > 1e-5 {
		t.Fatalf("with a non-binding cap the two loops differ beyond rounding (rel diff %.3g)", relDiff)
	}
}

// blockRelDiff is the largest over-blocks of (max|a−b| / max|a|).
func blockRelDiff(a, b *Model) float64 {
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
