package policynet

import (
	"math"
	"math/rand/v2"
)

// This file is the model half of the learned per-option scorer (plan L9b,
// R1 §1.4/§3.2/§3.4): a shared hashed embedding table over the encoder's
// sparse rows, a dense projection of the state's dense vector, and a
// one-hidden-layer ReLU MLP that scores [state ‖ option]. Forward and
// backward are HAND-DERIVED — there is no autodiff (R1 §3.4); the gradients
// are pinned by a finite-difference test (net_test.go).
//
// Shape (all widths fixed by the encoder contract in policynet.go/option.go):
//
//	state trunk  s ∈ R^H   = Σ_(r,v)∈state.sparse E[r]·v + W_sd·state.dense + b_s
//	option vec   o ∈ R^(H+152) = [Σ_(r,v)∈opt.hashed E[r]·v ‖ opt.slots ‖ opt.dense]
//	input        x = [s ‖ o] ∈ R^(2H+152)
//	hidden       a = relu(W_h·x + b_h) ∈ R^hidden
//	score        y = w_o·a + b_o  (scalar — one per offered option)
//
// The option's one-hot slots and 24 dense scalars enter RAW (concatenated),
// not embedded: they are already a fixed-width learned-input space (R1 §1.4's
// 106 categorical slots + 24 dense), and giving each its own embedding row
// would duplicate them. The hashed ids share the STATE's table, so a card
// seen in the state bag and the same card offered as an option land on the
// same embedding row — that is the R1 §1.4 sharing.
//
// Everything is float32; the LOSS arithmetic is float64 (softmax/LSE
// stability), which does not affect weight determinism. No map iteration
// reaches any output; the option list order is the record's order.

// DefaultHuberDelta and DefaultRankWeight are the trainer's default loss
// weights (see LossConfig).
const (
	DefaultHuberDelta = 0.1
	DefaultRankWeight = 1.0
)

// Model is the per-option scorer. The parameter blocks are exactly the four
// the finite-difference test exercises: the embedding table, the state dense
// projection, the hidden layer and the output layer.
type Model struct {
	Rows   int // embedding table rows (TableRows for a checkpoint-rounded model)
	H      int // embedding width
	Hidden int // hidden layer width
	InW    int // hidden input width = 2H + OptionSlotWidth + OptionDenseWidth

	Table  []float32 // Rows*H, row-major: Table[r*H+j]
	StateW []float32 // DenseWidth*H, row-major: StateW[d*H+j]
	StateB []float32 // H
	HidW   []float32 // Hidden*InW, row-major: HidW[h*InW+i]
	HidB   []float32 // Hidden
	OutW   []float32 // Hidden
	OutB   float32
}

// LossConfig carries the loss weights. HuberDelta is the Huber loss's
// transition point in value units (the teacher's candidate means live in
// [0,1]); RankWeight scales the margin-weighted ranking term against the
// value term. The ranking term's per-example weight is
// RankWeight · Example.Margin — a decision the teacher covered with a large
// margin pulls harder. Defaults are DefaultHuberDelta / DefaultRankWeight.
type LossConfig struct {
	HuberDelta float64
	RankWeight float64
}

// DefaultLossConfig returns the default loss weights.
func DefaultLossConfig() LossConfig {
	return LossConfig{HuberDelta: DefaultHuberDelta, RankWeight: DefaultRankWeight}
}

// LossParts is one example's loss broken into its two terms. Total is the
// quantity the trainer descends: Value + Rank.
type LossParts struct {
	Value float64 // mean Huber over the labelled options
	Rank  float64 // RankWeight·margin·(LSE over labelled − LSE over preferred)
	Total float64
}

// StepStat is one example's training-relevant readout: the loss and whether
// the argmax over the LABELLED options landed in the teacher-preferred set
// (top-1 agreement with the teacher; Eligible false when the example has no
// preferred option and so cannot agree or disagree).
type StepStat struct {
	Loss     float64
	Parts    LossParts
	Agree    bool
	Eligible bool
}

// NewModel builds a model with the given geometry, initialised from a
// caller-seeded rng (deterministic when the seed is; math/rand/v2 with an
// explicit source — the repo forbids the v1 package). Uniform Glorot-style
// limits per block: the table small (a sum-pool of ~200 rows must start near
// zero), the projections at sqrt(6/(fan_in+fan_out)).
func NewModel(rows, h, hidden int, rng *rand.Rand) *Model {
	m := &Model{
		Rows:   rows,
		H:      h,
		Hidden: hidden,
		InW:    2*h + OptionSlotWidth + OptionDenseWidth,
	}
	m.Table = make([]float32, rows*h)
	m.StateW = make([]float32, DenseWidth*h)
	m.StateB = make([]float32, h)
	m.HidW = make([]float32, hidden*m.InW)
	m.HidB = make([]float32, hidden)
	m.OutW = make([]float32, hidden)
	fillUniform(m.Table, 0.05, rng)
	fillUniform(m.StateW, glorot(DenseWidth, h), rng)
	fillUniform(m.HidW, glorot(m.InW, hidden), rng)
	fillUniform(m.OutW, glorot(hidden, 1), rng)
	return m
}

func glorot(fanIn, fanOut int) float32 {
	return float32(6 / (float64(fanIn) + float64(fanOut)))
}

func fillUniform(w []float32, lim float32, rng *rand.Rand) {
	for i := range w {
		w[i] = (2*rng.Float32() - 1) * lim
	}
}

// StateTrunk computes the state embedding s ∈ R^H: sum-pool of the state's
// sparse hashed rows plus the dense projection. Zero-valued dense inputs are
// skipped (exact: adding w·0 is a no-op for finite weights).
func (m *Model) StateTrunk(st State) []float32 {
	s := make([]float32, m.H)
	copy(s, m.StateB)
	for _, f := range st.Sparse {
		base := int(f.Row) * m.H
		v := f.Value
		row := m.Table[base : base+m.H]
		for j := range s {
			s[j] += row[j] * v
		}
	}
	for d := 0; d < DenseWidth && d < len(st.Dense); d++ {
		v := st.Dense[d]
		if v == 0 {
			continue
		}
		base := d * m.H
		for j := range s {
			s[j] += m.StateW[base+j] * v
		}
	}
	return s
}

// inputVector builds one option's hidden-layer input x = [s ‖ hashed-embed ‖
// slots ‖ dense]. Slots whose Row falls outside OptionSlotWidth are dropped
// (the encoder never emits one).
func (m *Model) inputVector(s []float32, o Option) []float32 {
	x := make([]float32, m.InW)
	copy(x, s)
	off := m.H
	for _, f := range o.Hashed {
		base := int(f.Row) * m.H
		v := f.Value
		for j := 0; j < m.H; j++ {
			x[off+j] += m.Table[base+j] * v
		}
	}
	off += m.H
	for _, f := range o.Slots {
		if r := int(f.Row); r >= 0 && r < OptionSlotWidth {
			x[off+r] += f.Value
		}
	}
	off += OptionSlotWidth
	copy(x[off:], o.Dense)
	return x
}

// forwardHead runs the hidden layer + output scalar on one input vector.
func (m *Model) forwardHead(x []float32) (y float32, a []float32) {
	a = make([]float32, m.Hidden)
	for h := 0; h < m.Hidden; h++ {
		sum := m.HidB[h]
		base := h * m.InW
		for i, xi := range x {
			if xi != 0 {
				sum += m.HidW[base+i] * xi
			}
		}
		if sum > 0 {
			a[h] = sum
		}
	}
	y = m.OutB
	for h, ah := range a {
		if ah != 0 {
			y += m.OutW[h] * ah
		}
	}
	return y, a
}

// Score scores every offered option (the inference path, L9c's entry point):
// the state trunk is computed once, then each option gets its own head
// forward. The returned slice parallels opts.
func (m *Model) Score(st State, opts []Option) []float32 {
	s := m.StateTrunk(st)
	out := make([]float32, len(opts))
	for i := range opts {
		x := m.inputVector(s, opts[i])
		y, _ := m.forwardHead(x)
		out[i] = y
	}
	return out
}

// Grads holds one accumulated gradient per parameter block. Table rows are
// tracked (touched) so Reset between batches costs O(touched) rather than a
// full 2.1M-element memset, and ApplyGrads only walks touched rows.
type Grads struct {
	m *Model

	Table  []float32
	StateW []float32
	StateB []float32
	HidW   []float32
	HidB   []float32
	OutW   []float32
	OutB   float32

	noted   []byte    // Rows: 1 when the row is in touched
	touched []int32   // table rows written since the last Reset
	dx      []float32 // scratch: hidden-input gradient, one option at a time
	da      []float32 // scratch: hidden activation gradient
	dz      []float32 // scratch: hidden pre-activation gradient
	ds      []float32 // scratch: accumulated state-trunk gradient
}

// NewGrads allocates a zeroed gradient buffer for m.
func (m *Model) NewGrads() *Grads {
	g := &Grads{
		m:      m,
		Table:  make([]float32, len(m.Table)),
		StateW: make([]float32, len(m.StateW)),
		StateB: make([]float32, len(m.StateB)),
		HidW:   make([]float32, len(m.HidW)),
		HidB:   make([]float32, len(m.HidB)),
		OutW:   make([]float32, len(m.OutW)),
		noted:  make([]byte, m.Rows),
		dx:     make([]float32, m.InW),
		da:     make([]float32, m.Hidden),
		dz:     make([]float32, m.Hidden),
		ds:     make([]float32, m.H),
	}
	return g
}

// Zero clears every block (a full reset — used by the gradient tests).
func (g *Grads) Zero() {
	zero(g.Table)
	zero(g.StateW)
	zero(g.StateB)
	zero(g.HidW)
	zero(g.HidB)
	zero(g.OutW)
	g.OutB = 0
	g.touched = g.touched[:0]
	for i := range g.noted {
		g.noted[i] = 0
	}
}

// Reset clears only the table rows touched since the last Reset plus the
// always-dense blocks. Between batches this is the cheap path.
func (g *Grads) Reset() {
	for _, r := range g.touched {
		base := int(r) * g.m.H
		zero(g.Table[base : base+g.m.H])
		g.noted[r] = 0
	}
	g.touched = g.touched[:0]
	zero(g.StateW)
	zero(g.StateB)
	zero(g.HidW)
	zero(g.HidB)
	zero(g.OutW)
	g.OutB = 0
}

// ApplyGrads descends one step: every parameter p ← p − scale·∇p. scale is
// the caller's lr/batchSize.
func (m *Model) ApplyGrads(g *Grads, scale float32) {
	for _, r := range g.touched {
		base := int(r) * m.H
		for j := 0; j < m.H; j++ {
			m.Table[base+j] -= scale * g.Table[base+j]
		}
	}
	for i := range m.StateW {
		m.StateW[i] -= scale * g.StateW[i]
	}
	for i := range m.StateB {
		m.StateB[i] -= scale * g.StateB[i]
	}
	for i := range m.HidW {
		m.HidW[i] -= scale * g.HidW[i]
	}
	for i := range m.HidB {
		m.HidB[i] -= scale * g.HidB[i]
	}
	for i := range m.OutW {
		m.OutW[i] -= scale * g.OutW[i]
	}
	m.OutB -= scale * g.OutB
}

func zero(w []float32) {
	for i := range w {
		w[i] = 0
	}
}

// addTable accumulates vec·scale into gradient row r, noting the row.
func (g *Grads) addTable(r int, vec []float32, scale float32) {
	if g.noted[r] == 0 {
		g.noted[r] = 1
		g.touched = append(g.touched, int32(r))
	}
	base := r * g.m.H
	for j := range vec {
		g.Table[base+j] += scale * vec[j]
	}
}

// forwardExample computes the state trunk and every labelled option's input
// vector, hidden activations and score. The returned indices parallel ys/xs/as.
func (m *Model) forwardExample(ex Example) (labelled []int, trunk []float32, xs, as [][]float32, ys []float64) {
	s := m.StateTrunk(ex.State)
	for i := range ex.Options {
		if !ex.Options[i].Target.Labelled {
			continue
		}
		x := m.inputVector(s, ex.Options[i])
		y, a := m.forwardHead(x)
		labelled = append(labelled, i)
		xs = append(xs, x)
		as = append(as, a)
		ys = append(ys, float64(y))
	}
	return labelled, s, xs, as, ys
}

// lossFromScores computes the combined loss and the per-labelled-option
// score gradient, given the forward pass. The ranking term is a softmax
// cross-entropy over the LABELLED options toward the teacher-preferred set
// (uniform over that set via the log-sum-exp form
// LSE(labelled) − LSE(preferred)), weighted by RankWeight·margin. An example
// with no preferred option, or a single labelled option, contributes only
// the value term (a one-option softmax has zero gradient — the only legal
// answer is the only answer).
func lossFromScores(lc LossConfig, ex Example, labelled []int, ys []float64) (parts LossParts, dys []float64) {
	n := len(labelled)
	parts.Value = 0
	for k, i := range labelled {
		e := ys[k] - ex.Options[i].Target.Value
		parts.Value += huber(e, lc.HuberDelta)
	}
	parts.Value /= float64(n)

	dys = make([]float64, n)
	for k := range dys {
		e := ys[k] - ex.Options[labelled[k]].Target.Value
		dys[k] += huberGrad(e, lc.HuberDelta) / float64(n)
	}

	pref := make([]int, 0, 2)
	for k, i := range labelled {
		if ex.Options[i].Target.Preferred {
			pref = append(pref, k)
		}
	}
	if len(pref) == 0 || n <= 1 {
		parts.Total = parts.Value
		return parts, dys
	}
	w := lc.RankWeight * ex.Margin
	if w == 0 {
		parts.Total = parts.Value
		return parts, dys
	}
	lseAll := logSumExp(ys)
	prefYs := make([]float64, len(pref))
	for i, k := range pref {
		prefYs[i] = ys[k]
	}
	lsePref := logSumExp(prefYs)
	parts.Rank = w * (lseAll - lsePref)
	for k := range dys {
		dys[k] += w * math.Exp(ys[k]-lseAll)
	}
	for _, k := range pref {
		dys[k] -= w
	}
	parts.Total = parts.Value + parts.Rank
	return parts, dys
}

// Loss evaluates the example's loss (forward only — no gradients).
func (m *Model) Loss(ex Example, lc LossConfig) StepStat {
	labelled, _, _, _, ys := m.forwardExample(ex)
	if len(labelled) == 0 {
		return StepStat{}
	}
	parts, _ := lossFromScores(lc, ex, labelled, ys)
	st := StepStat{Loss: parts.Total, Parts: parts}
	st.Eligible, st.Agree = agreement(ex, labelled, ys)
	return st
}

// LossGrad evaluates the example's loss AND accumulates its parameter
// gradients into g (g is NOT reset first — the trainer accumulates batches).
func (m *Model) LossGrad(ex Example, lc LossConfig, g *Grads) StepStat {
	labelled, _, xs, as, ys := m.forwardExample(ex)
	if len(labelled) == 0 {
		return StepStat{}
	}
	parts, dys := lossFromScores(lc, ex, labelled, ys)
	st := StepStat{Loss: parts.Total, Parts: parts}
	st.Eligible, st.Agree = agreement(ex, labelled, ys)

	// Backward through the head, option by option; the state-trunk gradient
	// accumulates across options and is applied once at the end.
	for j := range g.ds {
		g.ds[j] = 0
	}
	for k, i := range labelled {
		dy := float32(dys[k])
		if dy == 0 {
			continue
		}
		x, a := xs[k], as[k]
		m.backHead(dy, x, a, g, g.dx)
		// dx[:H] is the state trunk's gradient contribution; dx[H:2H] is the
		// option hashed-embed's; the slots/dense region is raw input.
		for j := 0; j < m.H; j++ {
			g.ds[j] += g.dx[j]
		}
		for _, f := range ex.Options[i].Hashed {
			g.addTable(int(f.Row), g.dx[m.H:m.H+m.H], f.Value)
		}
	}
	// State trunk backward: dense projection + sparse table rows.
	for j := 0; j < m.H; j++ {
		g.StateB[j] += g.ds[j]
	}
	for d := 0; d < DenseWidth && d < len(ex.State.Dense); d++ {
		v := ex.State.Dense[d]
		if v == 0 {
			continue
		}
		base := d * m.H
		for j := 0; j < m.H; j++ {
			g.StateW[base+j] += v * g.ds[j]
		}
	}
	for _, f := range ex.State.Sparse {
		g.addTable(int(f.Row), g.ds, f.Value)
	}
	return st
}

// backHead propagates one option's score gradient through the output layer
// and the ReLU hidden layer into g, writing the hidden-input gradient into dx.
func (m *Model) backHead(dy float32, x, a []float32, g *Grads, dx []float32) {
	for i := range dx {
		dx[i] = 0
	}
	g.OutB += dy
	for h := 0; h < m.Hidden; h++ {
		g.da[h] = dy * m.OutW[h]
	}
	for h := 0; h < m.Hidden; h++ {
		d := g.da[h]
		if d == 0 || a[h] <= 0 {
			g.dz[h] = 0
			continue
		}
		g.dz[h] = d
		g.OutW[h] += dy * a[h] // dL/dOutW[h] = dy·a[h] (the missing block would have trained a headless net)
		g.HidB[h] += d
		base := h * m.InW
		for i, xi := range x {
			if xi != 0 {
				g.HidW[base+i] += d * xi
			}
		}
	}
	for h := 0; h < m.Hidden; h++ {
		d := g.dz[h]
		if d == 0 {
			continue
		}
		base := h * m.InW
		for i, xi := range x {
			if xi != 0 {
				dx[i] += m.HidW[base+i] * d
			}
		}
	}
}

// agreement reports whether the argmax over the labelled options lands in
// the teacher-preferred set. Ties go to the FIRST argmax (the lowest option
// index in the record's order), matching the engine's tie rule.
func agreement(ex Example, labelled []int, ys []float64) (eligible, agree bool) {
	hasPref := false
	for _, i := range labelled {
		if ex.Options[i].Target.Preferred {
			hasPref = true
			break
		}
	}
	if !hasPref {
		return false, false
	}
	best := 0
	for k := 1; k < len(ys); k++ {
		if ys[k] > ys[best] {
			best = k
		}
	}
	return true, ex.Options[labelled[best]].Target.Preferred
}

func huber(e, delta float64) float64 {
	ae := e
	if ae < 0 {
		ae = -ae
	}
	if ae <= delta {
		return 0.5 * e * e
	}
	return delta * (ae - 0.5*delta)
}

func huberGrad(e, delta float64) float64 {
	if e <= -delta {
		return -delta
	}
	if e >= delta {
		return delta
	}
	return e
}

func logSumExp(v []float64) float64 {
	max := v[0]
	for _, x := range v[1:] {
		if x > max {
			max = x
		}
	}
	sum := 0.0
	for _, x := range v {
		sum += math.Exp(x - max)
	}
	return max + math.Log(sum)
}
