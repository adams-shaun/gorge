package policynet

import (
	"math"
	"math/rand/v2"
	"testing"
)

// The pinned encoder-contract hash (checkpoint.go's EncoderHash). Any drift
// in the hash function, a feature-string format, a vocabulary or a layout
// moves this value — and with it every existing checkpoint's loadability,
// which is exactly the point.
const pinnedEncoderHash = uint64(0xbe2c331ca2000aa6)

func TestEncoderHashPinned(t *testing.T) {
	if got := EncoderHash(); got != pinnedEncoderHash {
		t.Fatalf("EncoderHash() = %#x, want %#x — the encoder contract moved; every existing checkpoint is now stale (this is the intended tripwire, not a bug to paper over: bump CheckpointVersion consciously if the encoder really changed)", got, pinnedEncoderHash)
	}
}

// gradFixture builds a small model and one fully-labelled example touching
// every parameter block: state dense (all nonzero? several zero — the skip
// path), state sparse rows, option hashed rows, slots and option dense.
func gradFixture(rng *rand.Rand) (*Model, Example, LossConfig) {
	m := NewModel(64, 6, 5, rng)
	st := State{Dense: make([]float32, DenseWidth)}
	for i := range st.Dense {
		st.Dense[i] = float32(rng.NormFloat64()) * 0.7
	}
	// A few exact zeros so both the skip and the accumulate paths run.
	st.Dense[3] = 0
	st.Dense[40] = 0
	st.Dense[DenseWidth-1] = 0
	rows := make([]uint16, 40)
	for i := range rows {
		rows[i] = uint16(rng.IntN(64))
	}
	for i, r := range rows {
		v := float32(1)
		if i%3 == 0 {
			v = 0.5
		}
		st.Sparse = append(st.Sparse, Feature{Row: r, Value: v})
	}

	mkOpt := func(seed int64, pref bool) Option {
		or := rand.New(rand.NewPCG(uint64(seed), 2))
		o := Option{Dense: make([]float32, OptionDenseWidth)}
		for i := range o.Dense {
			o.Dense[i] = float32(or.NormFloat64()) * 0.4
		}
		o.Dense[odTapOut] = 1 // a one-hot-style dense entry
		o.Hashed = []Feature{
			{Row: uint16(or.IntN(64)), Value: 1},
			{Row: uint16(or.IntN(64)), Value: 0.75},
		}
		for _, r := range []int{0, 5, 37, 55, 71, 103} {
			o.Slots = append(o.Slots, Feature{Row: uint16(r + or.IntN(4)), Value: 1})
		}
		o.Target = OptionTarget{Labelled: true, Preferred: pref, Value: 0.3 + 0.4*or.Float64()}
		return o
	}
	ex := Example{
		Kind:          "choose",
		Margin:        0.4,
		TeacherChoice: 1,
		State:         st,
		Options:       []Option{mkOpt(11, false), mkOpt(22, true), mkOpt(33, false), mkOpt(44, false)},
	}
	avoidKinks(m, ex)
	lc := LossConfig{HuberDelta: 0.25, RankWeight: 1.0}
	return m, ex, lc
}

// avoidKinks shifts each hidden unit's bias until every labelled option's
// pre-activation clears the ReLU kink by a margin the FD interval cannot
// cross (eps·|x| ≤ 2e-3 at eps = 1e-3). A finite difference across a kink
// is not the gradient, and the random fixture otherwise lands several units
// within 0.02 of zero. Deterministic: shift up until min z ≥ 0.01 or all
// z ≤ −0.01 (a unit dead in every option is a fine outcome — it exercises
// the mask's zero side).
func avoidKinks(m *Model, ex Example) {
	const margin = 0.01
	const step = 0.013
	for h := 0; h < m.Hidden; h++ {
		for iter := 0; iter < 200; iter++ {
			zs := hiddenZs(m, ex, h)
			minZ, maxZ := zs[0], zs[0]
			for _, z := range zs[1:] {
				if z < minZ {
					minZ = z
				}
				if z > maxZ {
					maxZ = z
				}
			}
			if minZ >= margin || maxZ <= -margin {
				break
			}
			m.HidB[h] += step
		}
	}
}

// hiddenZs computes one unit's pre-activation for every labelled option.
func hiddenZs(m *Model, ex Example, h int) []float64 {
	var out []float64
	base := h * m.InW
	for i := range ex.Options {
		if !ex.Options[i].Target.Labelled {
			continue
		}
		x := m.inputVector(m.StateTrunk(ex.State), ex.Options[i])
		z := m.HidB[h]
		for ii, xi := range x {
			if xi != 0 {
				z += m.HidW[base+ii] * xi
			}
		}
		out = append(out, float64(z))
	}
	return out
}

// TestGradientFiniteDifference checks every parameter block's hand-derived
// gradient against a central finite difference of the forward-only loss.
// eps = 1e-3: small enough that no ReLU kink sits inside the difference
// interval for this fixed fixture, large enough that float32 forward
// rounding (~1e-7 relative on the loss) stays three orders below the
// differenced signal. Tolerance: 2e-3 absolute plus 2e-2 relative — the
// gradients here are O(1e-2..1), so the tolerance is loose in absolute
// terms and tight in relative ones.
func TestGradientFiniteDifference(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7+1))
	m, ex, lc := gradFixture(rng)
	const eps = 1e-3
	absTol, relTol := 2e-3, 2e-2

	check := func(block string, params []float32, grads []float32) {
		t.Helper()
		if len(params) != len(grads) {
			t.Fatalf("%s: %d params, %d grads", block, len(params), len(grads))
		}
		bad := 0
		for i := range params {
			old := params[i]
			params[i] = old + eps
			lp := m.Loss(ex, lc).Loss
			params[i] = old - eps
			lm := m.Loss(ex, lc).Loss
			params[i] = old
			num := (lp - lm) / (2 * eps)
			ana := float64(grads[i])
			d := math.Abs(num - ana)
			if d > absTol+relTol*math.Abs(ana) && d > absTol {
				if bad < 5 {
					t.Errorf("%s[%d]: numeric %.6g analytic %.6g (|diff| %.3g)", block, i, num, ana, d)
				}
				bad++
			}
		}
		if bad > 0 {
			t.Fatalf("%s: %d/%d coordinates outside tolerance", block, bad, len(params))
		}
	}

	// Analytic gradients, computed once.
	g := m.NewGrads()
	g.Zero()
	st := m.LossGrad(ex, lc, g)
	if st.Loss <= 0 {
		t.Fatalf("fixture loss %.6g must be positive for the difference to mean anything", st.Loss)
	}

	check("table", m.Table, g.Table)
	check("state dense projection", m.StateW, g.StateW)
	check("state projection bias", m.StateB, g.StateB)
	check("hidden", m.HidW, g.HidW)
	check("hidden bias", m.HidB, g.HidB)
	check("output", m.OutW, g.OutW)
	// The output bias is a scalar; check it through a one-element wrapper.
	oldB := m.OutB
	m.OutB = oldB + eps
	lp := m.Loss(ex, lc).Loss
	m.OutB = oldB - eps
	lm := m.Loss(ex, lc).Loss
	m.OutB = oldB
	num := (lp - lm) / (2 * eps)
	if d := math.Abs(num - float64(g.OutB)); d > absTol {
		t.Fatalf("output bias: numeric %.6g analytic %.6g (|diff| %.3g)", num, g.OutB, d)
	}
}

// TestScoreRunsAndIsDeterministic smoke-tests the inference path: scores
// come out one per option, in order, bit-identical across calls.
func TestScoreRunsAndIsDeterministic(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 11+1))
	m, ex, _ := gradFixture(rng)
	a := m.Score(ex.State, ex.Options)
	b := m.Score(ex.State, ex.Options)
	if len(a) != len(ex.Options) {
		t.Fatalf("Score returned %d scores for %d options", len(a), len(ex.Options))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("option %d: score %g then %g — the inference path is not deterministic", i, a[i], b[i])
		}
	}
}

// TestGradientAccumulatesAcrossOptions pins the accumulate contract the
// trainer's batch loop relies on: two LossGrad calls on DIFFERENT examples
// SUM (not overwrite), and Reset/Zero clears. The comparison carries a
// few-ulp tolerance because the fused multiply-add the compiler emits for
// `g += d·x` rounds a running accumulation differently than a separate
// sum-of-parts — float32 gradient arithmetic is not associative, and the
// trainer's contract is the running accumulation order.
func TestGradientAccumulatesAcrossOptions(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 9+1))
	m, ex, lc := gradFixture(rng)
	ex2 := ex
	ex2.Margin = 0 // kills the rank term; different loss, different grads

	g1 := m.NewGrads()
	g1.Zero()
	m.LossGrad(ex, lc, g1)
	m.LossGrad(ex2, lc, g1)

	g2 := m.NewGrads()
	g2.Zero()
	m.LossGrad(ex, lc, g2)
	g3 := m.NewGrads()
	g3.Zero()
	m.LossGrad(ex2, lc, g3)

	for i := range g1.HidW {
		a, b, c := g1.HidW[i], g2.HidW[i], g3.HidW[i]
		if math.Abs(float64(a-(b+c))) > 1e-6*(1+math.Abs(float64(b))+math.Abs(float64(c))) {
			t.Fatalf("hidden grad [%d]: accumulated %g, separate sum %g+%g", i, a, b, c)
		}
	}
	g1.Reset()
	if g1.HidB[0] != 0 {
		t.Fatalf("Reset left HidB[0] = %g", g1.HidB[0])
	}
	if g1.Table[0] != 0 {
		t.Fatalf("Reset left table row 0 = %g", g1.Table[0])
	}
}

// TestLossShapes pins the loss-shape edge cases the trainer relies on:
// an unlabelled option is excluded from BOTH terms exactly as if it were
// absent from the list, an example with no preferred option contributes
// value only, and a zero margin kills the rank term.
func TestLossShapes(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 13+1))
	m, ex, lc := gradFixture(rng)

	// Marking option 3 unlabelled must give the same loss AND the same
	// gradients as dropping it from the offered list entirely.
	exA := ex
	exA.Options[3].Target = OptionTarget{} // unlabelled
	exC := ex
	exC.Options = exC.Options[:3]
	lA := m.Loss(exA, lc)
	lC := m.Loss(exC, lc)
	if lA.Parts.Value != lC.Parts.Value || lA.Parts.Rank != lC.Parts.Rank || lA.Loss != lC.Loss {
		t.Fatalf("unlabelled option not excluded: loss %v vs %v", lA.Parts, lC.Parts)
	}
	gA, gC := m.NewGrads(), m.NewGrads()
	gA.Zero()
	m.LossGrad(exA, lc, gA)
	gC.Zero()
	m.LossGrad(exC, lc, gC)
	for i := range gA.HidW {
		if gA.HidW[i] != gC.HidW[i] {
			t.Fatalf("unlabelled option moved hidden grad [%d]: %g vs %g", i, gA.HidW[i], gC.HidW[i])
		}
	}
	if gA.OutB != gC.OutB {
		t.Fatalf("unlabelled option moved output-bias grad: %g vs %g", gA.OutB, gC.OutB)
	}

	// No preferred option → rank term zero even with a margin.
	exD := ex
	for i := range exD.Options {
		exD.Options[i].Target.Preferred = false
	}
	if got := m.Loss(exD, lc); got.Parts.Rank != 0 {
		t.Fatalf("no-preferred example carries rank loss %g", got.Parts.Rank)
	}

	// Zero margin → rank term zero even with a preferred option.
	exE := ex
	exE.Margin = 0
	if got := m.Loss(exE, lc); got.Parts.Rank != 0 {
		t.Fatalf("zero-margin example carries rank loss %g", got.Parts.Rank)
	}

	// An example with NO labelled options scores zero and never touches the
	// gradient buffer.
	exF := ex
	for i := range exF.Options {
		exF.Options[i].Target = OptionTarget{}
	}
	if got := m.Loss(exF, lc); got.Loss != 0 || got.Eligible {
		t.Fatalf("no-labelled example: loss %g eligible %v", got.Loss, got.Eligible)
	}
	gF := m.NewGrads()
	gF.Zero()
	m.LossGrad(exF, lc, gF)
	if gF.OutB != 0 {
		t.Fatalf("no-labelled example moved the output-bias grad: %g", gF.OutB)
	}
}
