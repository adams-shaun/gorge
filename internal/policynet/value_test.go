package policynet

import (
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
)

// valueFixture is gradFixture plus a value head and a blended target, with
// the value head's hidden units shifted clear of their ReLU kinks.
func valueFixture(rng *rand.Rand) (*Model, Example, LossConfig) {
	m, ex, lc := gradFixture(rng)
	m.InitValue(7, rand.New(rand.NewPCG(99, 100)))
	for i := range m.VHidB {
		m.VHidB[i] = float32(rng.NormFloat64()) * 0.1
	}
	m.VOutB = 0.2
	avoidValueKinks(m, ex.State)
	ex.Outcome, ex.HasOutcome = 1, true
	ex.TeacherValue, ex.HasTeacherValue = 0.35, true
	lc.ValueWeight, lc.ValueBlend = 1.3, 0.25
	return m, ex, lc
}

// avoidValueKinks pushes every value hidden unit's pre-activation at least
// 0.05 from zero (the FD interval at eps 1e-3 over ~40 summed rows moves it
// by far less), keeping some units dead so the mask's zero side runs too.
func avoidValueKinks(m *Model, st State) {
	s := m.StateTrunk(st)
	for k := 0; k < m.ValueHidden; k++ {
		u := float64(m.VHidB[k])
		for j := range s {
			u += float64(m.VHidW[k*m.H+j]) * float64(s[j])
		}
		switch {
		case u >= 0 && u < 0.05:
			m.VHidB[k] += float32(0.05 - u)
		case u < 0 && u > -0.05:
			m.VHidB[k] -= float32(0.05 + u)
		}
	}
}

// fdCheck compares one block's analytic gradient to a central difference of
// the forward-only loss (the tolerances TestGradientFiniteDifference uses).
func fdCheck(t *testing.T, m *Model, ex Example, lc LossConfig, block string, params, grads []float32) {
	t.Helper()
	const eps = 1e-3
	absTol, relTol := 2e-3, 2e-2
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
		if d := math.Abs(num - ana); d > absTol+relTol*math.Abs(ana) && d > absTol {
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

// scalarFD is fdCheck for one scalar parameter.
func scalarFD(t *testing.T, m *Model, ex Example, lc LossConfig, block string, p *float32, grad float32) {
	t.Helper()
	const eps = 1e-3
	old := *p
	*p = old + eps
	lp := m.Loss(ex, lc).Loss
	*p = old - eps
	lm := m.Loss(ex, lc).Loss
	*p = old
	num := (lp - lm) / (2 * eps)
	ana := float64(grad)
	if d := math.Abs(num - ana); d > 2e-3+2e-2*math.Abs(ana) && d > 2e-3 {
		t.Fatalf("%s: numeric %.6g analytic %.6g", block, num, ana)
	}
}

// TestValueGradientFiniteDifference extends the finite-difference pin to
// the value head: the joint loss (policy + ValueWeight·BCE) is checked
// against every block, including the four value blocks and the shared trunk
// (Table, StateW, StateB), which the value term now also feeds.
func TestValueGradientFiniteDifference(t *testing.T) {
	m, ex, lc := valueFixture(rand.New(rand.NewPCG(7, 8)))
	g := m.NewGrads()
	g.Zero()
	st := m.LossGrad(ex, lc, g)
	if st.Parts.ValueHead <= 0 {
		t.Fatalf("value term %g must be positive for the check to mean anything", st.Parts.ValueHead)
	}
	if lst := m.Loss(ex, lc); lst.Loss != st.Loss || lst.Parts != st.Parts {
		t.Fatalf("Loss %+v and LossGrad %+v disagree", lst.Parts, st.Parts)
	}
	fdCheck(t, m, ex, lc, "table", m.Table, g.Table)
	fdCheck(t, m, ex, lc, "state dense projection", m.StateW, g.StateW)
	fdCheck(t, m, ex, lc, "state projection bias", m.StateB, g.StateB)
	fdCheck(t, m, ex, lc, "hidden", m.HidW, g.HidW)
	fdCheck(t, m, ex, lc, "hidden bias", m.HidB, g.HidB)
	fdCheck(t, m, ex, lc, "output", m.OutW, g.OutW)
	scalarFD(t, m, ex, lc, "output bias", &m.OutB, g.OutB)
	fdCheck(t, m, ex, lc, "value hidden", m.VHidW, g.VHidW)
	fdCheck(t, m, ex, lc, "value hidden bias", m.VHidB, g.VHidB)
	fdCheck(t, m, ex, lc, "value output", m.VOutW, g.VOutW)
	scalarFD(t, m, ex, lc, "value output bias", &m.VOutB, g.VOutB)
}

// TestValueGradientReachesTrunk isolates the value term's trunk
// contribution: with the policy term silenced (hybrid mode, no preferred
// option, every labelled value tied — both policy terms are skipped), the
// whole gradient is the value head's, and it must be non-zero on the trunk
// blocks and match the finite difference there.
func TestValueGradientReachesTrunk(t *testing.T) {
	m, ex, lc := valueFixture(rand.New(rand.NewPCG(17, 18)))
	lc.Mode = LossHybrid
	for i := range ex.Options {
		ex.Options[i].Target.Preferred = false
		ex.Options[i].Target.Value = 0.5
	}
	g := m.NewGrads()
	g.Zero()
	st := m.LossGrad(ex, lc, g)
	if st.Parts.Value != 0 || st.Parts.Rank != 0 || st.Parts.ValueHead <= 0 {
		t.Fatalf("fixture parts %+v: want only the value-head term", st.Parts)
	}
	for _, blk := range []struct {
		name string
		g    []float32
		zero bool
	}{{"hidden", g.HidW, true}, {"output", g.OutW, true}, {"state bias", g.StateB, false}, {"state dense", g.StateW, false}, {"table", g.Table, false}} {
		n := 0.0
		for _, v := range blk.g {
			n += float64(v) * float64(v)
		}
		if (n == 0) != blk.zero {
			t.Fatalf("%s gradient norm² %g: want zero=%v", blk.name, n, blk.zero)
		}
	}
	fdCheck(t, m, ex, lc, "table", m.Table, g.Table)
	fdCheck(t, m, ex, lc, "state dense projection", m.StateW, g.StateW)
	fdCheck(t, m, ex, lc, "state projection bias", m.StateB, g.StateB)
}

// TestValueWeightZeroIsTodaysLoss: ValueWeight 0 on a model WITH a value head
// gives exactly the loss and policy gradients of the same model without one.
func TestValueWeightZeroIsTodaysLoss(t *testing.T) {
	m, ex, lc := valueFixture(rand.New(rand.NewPCG(27, 28)))
	lc.ValueWeight = 0
	bare := *m
	bare.InitValue(0, nil)

	ga, gb := m.NewGrads(), bare.NewGrads()
	ga.Zero()
	gb.Zero()
	sa, sb := m.LossGrad(ex, lc, ga), bare.LossGrad(ex, lc, gb)
	if sa != sb {
		t.Fatalf("stats differ: %+v vs %+v", sa, sb)
	}
	if !slices.Equal(ga.Table, gb.Table) || !slices.Equal(ga.StateW, gb.StateW) || !slices.Equal(ga.StateB, gb.StateB) ||
		!slices.Equal(ga.HidW, gb.HidW) || !slices.Equal(ga.OutW, gb.OutW) || ga.OutB != gb.OutB || ga.Norm() != gb.Norm() {
		t.Fatal("ValueWeight 0 moved a policy gradient")
	}
	for _, v := range append(append(append([]float32{}, ga.VHidW...), ga.VHidB...), ga.VOutW...) {
		if v != 0 {
			t.Fatal("ValueWeight 0 wrote a value gradient")
		}
	}
}

// TestValueTargetBlend pins the target blend and the no-target skips.
func TestValueTargetBlend(t *testing.T) {
	ex := Example{Outcome: 1, HasOutcome: true, TeacherValue: 0.4, HasTeacherValue: true}
	for _, c := range []struct {
		b    float64
		want float64
	}{{0, 1}, {1, 0.4}, {0.5, 0.7}, {0.25, 0.85}} {
		got, ok := LossConfig{ValueBlend: c.b}.ValueTarget(ex)
		if !ok || math.Abs(got-c.want) > 1e-12 {
			t.Fatalf("blend %g: target %g ok %v, want %g", c.b, got, ok, c.want)
		}
	}
	noOutcome := ex
	noOutcome.HasOutcome = false
	if _, ok := (LossConfig{ValueBlend: 0.5}).ValueTarget(noOutcome); ok {
		t.Fatal("blend 0.5 without an outcome produced a target")
	}
	if got, ok := (LossConfig{ValueBlend: 1}).ValueTarget(noOutcome); !ok || got != 0.4 {
		t.Fatalf("blend 1 without an outcome: %g %v, want 0.4 true", got, ok)
	}
	noTeacher := ex
	noTeacher.HasTeacherValue = false
	if _, ok := (LossConfig{ValueBlend: 0.5}).ValueTarget(noTeacher); ok {
		t.Fatal("blend 0.5 without a teacher value produced a target")
	}
	if got, ok := (LossConfig{}).ValueTarget(noTeacher); !ok || got != 1 {
		t.Fatalf("blend 0 without a teacher value: %g %v, want 1 true", got, ok)
	}

	// No target ⇒ no value term.
	m, fex, lc := valueFixture(rand.New(rand.NewPCG(37, 38)))
	fex.HasOutcome = false
	if st := m.Loss(fex, lc); st.Parts.ValueHead != 0 {
		t.Fatalf("value term %g on an example without its target", st.Parts.ValueHead)
	}
}

// TestInitValueLeavesPolicyInit: adding a value head from its own rng moves
// no policy block.
func TestInitValueLeavesPolicyInit(t *testing.T) {
	a := NewModel(64, 6, 5, rand.New(rand.NewPCG(1, 2)))
	b := NewModel(64, 6, 5, rand.New(rand.NewPCG(1, 2)))
	b.InitValue(4, rand.New(rand.NewPCG(1, 0x76616c7565)))
	if !b.HasValue() || a.HasValue() {
		t.Fatal("HasValue wrong")
	}
	if !slices.Equal(a.Table, b.Table) || !slices.Equal(a.StateW, b.StateW) || !slices.Equal(a.HidW, b.HidW) || !slices.Equal(a.OutW, b.OutW) {
		t.Fatal("InitValue moved a policy block")
	}
	if len(b.VHidW) != 4*6 || len(b.VHidB) != 4 || len(b.VOutW) != 4 {
		t.Fatalf("value geometry %d/%d/%d", len(b.VHidW), len(b.VHidB), len(b.VOutW))
	}
}

// TestScorerValueMatchesModelValue: Scorer.Value is bit-identical to
// Model.Value, Model.Value is safe to call concurrently on one shared model,
// V is in (0,1), and a model without a value head answers 0.5.
func TestScorerValueMatchesModelValue(t *testing.T) {
	m := NewModel(TableRows, 8, 4, rand.New(rand.NewPCG(41, 42)))
	m.InitValue(5, rand.New(rand.NewPCG(43, 44)))
	rng := rand.New(rand.NewPCG(45, 46))
	sc := NewScorer(m)
	states := make([]State, 16)
	want := make([]float32, len(states))
	for i := range states {
		states[i] = randomState(rng, TableRows)
		want[i] = m.Value(states[i])
		if !(want[i] > 0 && want[i] < 1) {
			t.Fatalf("state %d: V %g outside (0,1)", i, want[i])
		}
		if got := sc.Value(states[i]); got != want[i] {
			t.Fatalf("state %d: Scorer.Value %g, Model.Value %g", i, got, want[i])
		}
	}
	var wg sync.WaitGroup
	errs := make([]bool, 4)
	for w := range errs {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for rep := 0; rep < 20; rep++ {
				for i := range states {
					if m.Value(states[i]) != want[i] {
						errs[w] = true
					}
				}
			}
		}(w)
	}
	wg.Wait()
	for w, bad := range errs {
		if bad {
			t.Fatalf("worker %d read a different value concurrently", w)
		}
	}
	bare := NewModel(TableRows, 8, 4, rand.New(rand.NewPCG(41, 42)))
	if v := bare.Value(states[0]); v != 0.5 || NewScorer(bare).Value(states[0]) != 0.5 {
		t.Fatalf("no value head: V %g, want 0.5", v)
	}
}
