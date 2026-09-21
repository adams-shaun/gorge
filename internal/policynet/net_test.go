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

// tiedExample builds a fully-labelled example whose options all share the
// same teacher value (the real corpus's 42%-tied decision shape) and whose
// teacher kept the bot (Margin 0, so the rank term is off). The value term
// must contribute NOTHING to such a decision: regressing a score on a
// constant is minimised by flattening every option's score, which is exactly
// how option-discriminating features get erased.
func tiedExample(rng *rand.Rand) (*Model, Example) {
	m := NewModel(64, 8, 8, rng)
	st := State{Dense: make([]float32, DenseWidth)}
	st.Dense[0] = 0.7
	ex := Example{Kind: "attackers", Margin: 0, TeacherChoice: 0, BotIndex: 0, State: st}
	for j := 0; j < 3; j++ {
		o := Option{Dense: make([]float32, OptionDenseWidth)}
		o.Hashed = []Feature{{Row: uint16(10 + j), Value: 1}}
		o.Target = OptionTarget{Labelled: true, Value: 0.5, Preferred: j == 0}
		ex.Options = append(ex.Options, o)
	}
	return m, ex
}

// TestValueTermSkipsTiedDecisions pins the anti-collapse contract at the loss
// level: a decision whose labelled values are all tied contributes zero value
// loss and zero gradient, so training it cannot drive option-discriminating
// features toward zero. A raw-target value term (the pre-fix behaviour) gives
// this example a nonzero loss and a nonzero output-bias gradient, which is
// precisely the collapse the trainer fix removes.
func TestValueTermSkipsTiedDecisions(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 31+1))
	m, ex := tiedExample(rng)
	lc := LossConfig{HuberDelta: 0.1, RankWeight: 5}

	st := m.Loss(ex, lc)
	if st.Parts.Value != 0 || st.Parts.Total != 0 || st.Loss != 0 {
		t.Fatalf("tied decision carries value loss %g (total %g): the value term is trying to flatten the options",
			st.Parts.Value, st.Parts.Total)
	}

	g := m.NewGrads()
	g.Zero()
	m.LossGrad(ex, lc, g)
	if g.OutB != 0 {
		t.Fatalf("tied decision moved the output-bias gradient: %g", g.OutB)
	}
	for i := range g.OutW {
		if g.OutW[i] != 0 {
			t.Fatalf("tied decision moved the output weight gradient [%d]: %g", i, g.OutW[i])
		}
	}
	for i := range g.HidB {
		if g.HidB[i] != 0 {
			t.Fatalf("tied decision moved the hidden-bias gradient [%d]: %g", i, g.HidB[i])
		}
	}
}

// TestOverrideWeightScalesExample pins the signal-weighting contract: an
// example whose label records a teacher override (TeacherChoice != BotIndex)
// is scaled by LossConfig.OverrideWeight in BOTH its loss and its gradients,
// while an agreeing example is untouched. That is the lever that stops the
// 75% of decisions where the teacher merely agrees with the bot from drowning
// the 25% that carry the information.
func TestOverrideWeightScalesExample(t *testing.T) {
	rng := rand.New(rand.NewPCG(41, 41+1))
	m, base := tiedExample(rng)
	// Give the example real signal so loss and gradient are nonzero.
	for i := range base.Options {
		base.Options[i].Target.Value = 0.5 + 0.2*float64(i)
	}
	base.Margin = 0.3

	agree := base
	agree.TeacherChoice = 0
	agree.BotIndex = 0
	override := base
	override.TeacherChoice = 1
	override.BotIndex = 0
	if !override.Override() {
		t.Fatal("override example not recognised as an override")
	}

	lc := LossConfig{HuberDelta: 0.1, RankWeight: 1, OverrideWeight: 4}
	agreeStat := m.Loss(agree, lc)
	overStat := m.Loss(override, lc)
	if want := agreeStat.Parts.Total * 4; math.Abs(overStat.Parts.Total-want) > 1e-9 {
		t.Fatalf("override loss %.6g, want 4x the agreeing loss %.6g", overStat.Parts.Total, want)
	}

	gA, gO := m.NewGrads(), m.NewGrads()
	gA.Zero()
	m.LossGrad(agree, lc, gA)
	gO.Zero()
	m.LossGrad(override, lc, gO)
	for i := range gA.OutW {
		if math.Abs(float64(gO.OutW[i]-4*gA.OutW[i])) > 1e-6 {
			t.Fatalf("override output-weight grad [%d] = %g, want 4x agreeing %g", i, gO.OutW[i], gA.OutW[i])
		}
	}
	if math.Abs(float64(gO.OutB-4*gA.OutB)) > 1e-6 {
		t.Fatalf("override output-bias grad = %g, want 4x agreeing %g", gO.OutB, gA.OutB)
	}
}

// TestLossModeSwitchPinsTermMix pins the L9b-fix2 mode contract at the loss
// level: CE contributes ONLY the rank term (Parts.Value exactly 0) and its
// per-example weight ignores the margin (pure imitation: a one-world margin
// must not silence the CE signal the way it silenced the margin-weighted rank
// term); value contributes ONLY the Huber term (Parts.Rank exactly 0); and
// the zero Mode ("", every pre-existing caller) is byte-equal to the explicit
// hybrid.
func TestLossModeSwitchPinsTermMix(t *testing.T) {
	rng := rand.New(rand.NewPCG(43, 43+1))
	m, ex := tiedExample(rng)
	// Untie the values so the value term is active in every mode, and keep a
	// preferred option so the rank term is active too.
	for i := range ex.Options {
		ex.Options[i].Target.Value = 0.5 + 0.1*float64(i)
	}
	ex.Margin = 0.001 // the noise-dominated scale: a hundredth of one world

	ceLo := m.Loss(ex, LossConfig{Mode: LossCE, HuberDelta: 0.1, RankWeight: 1})
	exHi := ex
	exHi.Margin = 10
	ceHi := m.Loss(exHi, LossConfig{Mode: LossCE, HuberDelta: 0.1, RankWeight: 1})
	if ceLo.Parts.Value != 0 {
		t.Fatalf("CE carries a value term: %g", ceLo.Parts.Value)
	}
	if ceLo.Parts.Rank <= 0 {
		t.Fatalf("CE carries no rank term: %g", ceLo.Parts.Rank)
	}
	if ceLo.Parts.Total != ceHi.Parts.Total {
		t.Fatalf("CE loss follows the margin (%g at margin 0.001 vs %g at 10) — the CE weight must not be margin-scaled",
			ceLo.Parts.Total, ceHi.Parts.Total)
	}

	val := m.Loss(ex, LossConfig{Mode: LossValue, HuberDelta: 0.1, RankWeight: 1})
	if val.Parts.Rank != 0 {
		t.Fatalf("value mode carries a rank term: %g", val.Parts.Rank)
	}
	if val.Parts.Value <= 0 {
		t.Fatalf("value mode carries no value term: %g", val.Parts.Value)
	}

	hybZero := m.Loss(ex, LossConfig{HuberDelta: 0.1, RankWeight: 1})
	hybNamed := m.Loss(ex, LossConfig{Mode: LossHybrid, HuberDelta: 0.1, RankWeight: 1})
	if hybZero.Parts != hybNamed.Parts {
		t.Fatalf("zero Mode is not hybrid: %+v vs %+v", hybZero.Parts, hybNamed.Parts)
	}
	if hybNamed.Parts.Value <= 0 || hybNamed.Parts.Rank <= 0 {
		t.Fatalf("hybrid lost one of its terms: %+v", hybNamed.Parts)
	}
}

// TestResidualIsAFixedBotPrior pins the L9b-fix2 item-3 residual head: an
// option in the bot's own answer (Option.BotPick) scores exactly Model.ResidualW
// higher and nothing else moves; ResidualW is a constant prior, never a
// trained parameter (no Grads field touches it); and ResidualW 0 makes the
// residual a no-op, so pure CE is byte-identical to the pre-residual build.
func TestResidualIsAFixedBotPrior(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	m := NewModel(256, 8, 8, rng)
	st := State{Dense: make([]float32, DenseWidth)}
	opts := []Option{
		{Dense: make([]float32, OptionDenseWidth), BotPick: true},
		{Dense: make([]float32, OptionDenseWidth)},
	}
	base := m.Score(st, opts)
	if base[0] != base[1] {
		t.Fatalf("identical options scored differently: %g vs %g", base[0], base[1])
	}
	m.ResidualW = 3
	got := m.Score(st, opts)
	if got[1] != base[1] {
		t.Fatalf("non-bot option moved under the residual: %g -> %g", base[1], got[1])
	}
	if d := float64(got[0] - base[0]); d < 2.999 || d > 3.001 {
		t.Fatalf("bot option residual = %g, want +3", d)
	}

	// The prior is not a gradient block: a forward/backward that reads the
	// residual must leave ResidualW untouched (it lives on the Model).
	for i := range opts {
		opts[i].Target = OptionTarget{Labelled: true, Value: 0.5, Preferred: i == 1}
	}
	ex := Example{Kind: "attackers", State: st, Options: opts}
	before := m.ResidualW
	g := m.NewGrads()
	m.LossGrad(ex, LossConfig{Mode: LossCE, RankWeight: 1}, g)
	g.Reset()
	if m.ResidualW != before {
		t.Fatalf("ResidualW changed during a loss step: %g -> %g", before, m.ResidualW)
	}
}

// TestLossFiniteOnDivergedScores pins the numerical-stability contract: the
// loss and its score gradient stay finite even when the forward pass hands
// back ±Inf/NaN scores (a diverged run). The pre-fix loss fed the raw scores
// to logSumExp, so an Inf made `LSE(labelled) − LSE(preferred)` compute
// Inf − Inf = NaN, which then poisoned every weight; the score clamp plus the
// max-subtracting log-sum-exp keep the whole term finite.
func TestLossFiniteOnDivergedScores(t *testing.T) {
	ex := Example{Kind: "attackers", Margin: 0.3, TeacherChoice: 1, BotIndex: 0,
		State: State{Dense: make([]float32, DenseWidth)}}
	for j := 0; j < 3; j++ {
		o := Option{Dense: make([]float32, OptionDenseWidth)}
		o.Target = OptionTarget{Labelled: true, Value: 0.2 + 0.3*float64(j), Preferred: j == 1}
		ex.Options = append(ex.Options, o)
	}
	labelled := []int{0, 1, 2}
	lc := LossConfig{HuberDelta: 0.1, RankWeight: 50}
	for _, name := range []string{"+Inf", "-Inf", "NaN"} {
		ys := []float64{1, 2, 3}
		switch name {
		case "+Inf":
			ys[2] = math.Inf(1)
		case "-Inf":
			ys[1] = math.Inf(-1)
		case "NaN":
			ys[0] = math.NaN()
		}
		parts, dys := lossFromScores(lc, ex, labelled, ys)
		if math.IsNaN(parts.Total) || math.IsInf(parts.Total, 0) ||
			math.IsNaN(parts.Value) || math.IsInf(parts.Value, 0) ||
			math.IsNaN(parts.Rank) || math.IsInf(parts.Rank, 0) {
			t.Fatalf("%s scores: non-finite loss %+v", name, parts)
		}
		for k, d := range dys {
			if math.IsNaN(d) || math.IsInf(d, 0) {
				t.Fatalf("%s scores: non-finite score gradient [%d] = %g", name, k, d)
			}
		}
	}
}
