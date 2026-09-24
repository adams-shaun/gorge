package policynet

import (
	"io"
)

// Scorer is the inference half of the learned per-option policy (plan L9c):
// a trained Model plus reusable scratch buffers, scoring one encoded option
// at a time for a seat-side policy. The contract:
//
//   - the weights come from a checkpoint and ONLY from a checkpoint-shaped
//     Model — LoadScorer/LoadScorerFile run the full checkpoint gate (magic,
//     schema version, encoder hash, geometry), so a checkpoint trained
//     against a drifted encoder is refused before it can score anything;
//   - single-threaded: a Scorer owns its scratch, so concurrent Score calls
//     on ONE Scorer race. A seat that needs two scorers builds two (a bench
//     worker builds one per seat per game over the shared read-only Model);
//   - float32 arithmetic in the EXACT summation order Model.StateTrunk /
//     inputVector / forwardHead use, so Scorer.Score and Model.Score return
//     bit-identical scores for the same (state, options) — pinned by
//     TestScorerMatchesModelScore;
//   - NO allocation per option beyond the scratch buffers: ScoreOption only
//     writes sc.s/sc.x/sc.a. Score allocates one result slice per call (per
//     decision, not per option);
//   - bit-identical results for the same (checkpoint, state, option) on
//     repeated calls — the scratch is zeroed before every reuse, so no call
//     can read another's residue. Pinned by TestScorerRepeatedCallsAreBitIdentical.
type Scorer struct {
	m *Model
	// s is the state trunk (H), x the hidden-layer input (InW), a the hidden
	// activations (Hidden). All owned by this Scorer, never shared.
	s []float32
	x []float32
	a []float32
	// va is the value head's hidden activations (ValueHidden; empty when the
	// model has no value head).
	va []float32
}

// NewScorer wraps a trained (or zero) Model for inference. The Model is
// treated as read-only from here on.
func NewScorer(m *Model) *Scorer {
	if m == nil || m.H < 1 || m.Hidden < 1 || m.InW < 1 {
		panic("policynet: NewScorer on a nil or degenerate model")
	}
	return &Scorer{
		m:  m,
		s:  make([]float32, m.H),
		x:  make([]float32, m.InW),
		a:  make([]float32, m.Hidden),
		va: make([]float32, m.ValueHidden),
	}
}

// HasValue reports whether the wrapped model carries a value head.
func (sc *Scorer) HasValue() bool { return sc.m.HasValue() }

// Value is the scratch variant of Model.Value for single-threaded callers:
// it loads st as the scorer's state (exactly SetState, so a following
// ScoreOption scores against it) and returns V(s) ∈ (0,1), bit-identical to
// Model.Value (TestScorerValueMatchesModelValue). 0.5 on a model without a
// value head.
func (sc *Scorer) Value(st State) float32 {
	sc.SetState(st)
	if !sc.m.HasValue() {
		return 0.5
	}
	return float32(sigmoidFloat(float64(sc.m.valueForward(sc.s, sc.va))))
}

// ResidualWeight reports the model's fixed bot-prior residual weight
// (Model.ResidualW, the train Config.ResidualInit the model was trained
// with). The seat-side bot reads it to decide whether the residual prior is
// ACTIVE at inference: positive means the wrapped default bot's own answer
// is marked (Option.BotPick) so the scorer scores under the contract it
// trained under, and is a precondition for the seat scoring KPriority at
// all; zero — the weight a checkpoint trained without -residual-init loads —
// makes the mark a no-op and keeps priority delegated to the default bot.
func (sc *Scorer) ResidualWeight() float32 {
	return sc.m.ResidualW
}

// LoadScorer reads a checkpoint (the full LoadCheckpoint gate: magic, schema
// version, encoder hash, geometry, exact body length) and wraps it for
// inference.
func LoadScorer(r io.Reader) (*Scorer, error) {
	m, err := LoadCheckpoint(r)
	if err != nil {
		return nil, err
	}
	return NewScorer(m), nil
}

// LoadScorerFile loads a checkpoint from a path.
func LoadScorerFile(path string) (*Scorer, error) {
	m, err := LoadCheckpointFile(path)
	if err != nil {
		return nil, err
	}
	return NewScorer(m), nil
}

// SetState computes the state trunk s ∈ R^H once per decision. Every
// ScoreOption call after it scores against THIS state, so the trunk's cost
// (the sparse bag's sum-pool) is paid once per decision, not per option.
// The summation order is Model.StateTrunk's exactly.
func (sc *Scorer) SetState(st State) {
	m := sc.m
	s := sc.s
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
}

// ScoreOption scores one encoded option against the state SetState (or the
// last Score) loaded. It allocates nothing: every intermediate lives in the
// Scorer's scratch, zeroed before reuse so repeated calls are bit-identical.
func (sc *Scorer) ScoreOption(o Option) float32 {
	m := sc.m
	x := sc.x
	copy(x[:m.H], sc.s)
	zero(x[m.H:])

	// The option half of Model.inputVector, on the scratch.
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
	return sc.scoreHead(x) + m.residual(o)
}

// scoreHead runs the hidden layer + output scalar on one input vector, into
// the scratch activations — the same arithmetic Model.forwardHead runs, with
// the zero-skipping reads kept so the summation order cannot drift.
func (sc *Scorer) scoreHead(x []float32) float32 {
	m := sc.m
	a := sc.a
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
		} else {
			a[h] = 0
		}
	}
	y := m.OutB
	for h, ah := range a {
		if ah != 0 {
			y += m.OutW[h] * ah
		}
	}
	return y
}

// Score scores every offered option: the state trunk once, then one
// scratch-forward per option. The returned slice parallels opts.
func (sc *Scorer) Score(st State, opts []Option) []float32 {
	sc.SetState(st)
	out := make([]float32, len(opts))
	for i := range opts {
		out[i] = sc.ScoreOption(opts[i])
	}
	return out
}
