package neteval

import (
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
)

// The policynet backend: a bots/policynet checkpoint (the L9c "GPOL"
// binary policytrain writes and azmcts's policynet leaf reads) run on the
// materialized FeaturesEntity encoding. The weights are policynet.Model's,
// read through policynet.LoadCheckpointFile (its full gate: magic, schema
// version, encoder hash, geometry); the forward pass is Model.Score's and
// Model.Value's arithmetic in the same summation order, on per-Net
// scratch, so the priors and the value are float-identical to the Model's
// (TestPolicynetEvalMatchesGorge) and an evaluation allocates nothing
// (TestPolicynetEvalZeroAllocs).

// pnEntSumScale is policynet's entSumScale (entity.go): the sum half of
// an entity group's pool is scaled by it. A drift fails
// TestPolicynetEvalMatchesGorge.
const pnEntSumScale = 0.25

// LoadPolicynet reads a bots/policynet checkpoint as Weights. Only the
// FeaturesEntity set loads: it is the encoding dirac/feat writes. A model
// with an External scorer or per-option extra inputs is not a checkpoint
// and is refused. A K-way value head answers Value with its component 0
// (policynet Model.Value's P(me)); the Net reports no value vector.
func LoadPolicynet(path string) (*Weights, error) {
	m, err := policynet.LoadCheckpointFile(path)
	if err != nil {
		return nil, fmt.Errorf("neteval %s: %w", path, err)
	}
	if m.Features != policynet.FeaturesEntity {
		return nil, fmt.Errorf("neteval %s: feature set %s, this encoder writes %s", path, m.Features, policynet.FeaturesEntity)
	}
	if m.EntK < 1 || m.Ext != nil || m.ExtraW != 0 {
		return nil, fmt.Errorf("neteval %s: not an entity checkpoint model (EntK %d, ExtraW %d)", path, m.EntK, m.ExtraW)
	}
	if raw := m.Features.EntityRawW(); raw != feat.EntityRawWidth {
		return nil, fmt.Errorf("neteval %s: net reads entity width %d, this encoder writes %d", path, raw, feat.EntityRawWidth)
	}
	if m.StateDenseW() > feat.DenseWidth || m.OptDenseW() > feat.OptionDenseWidth {
		return nil, fmt.Errorf("neteval %s: net reads dense widths %d/%d, this encoder writes %d/%d", path, m.StateDenseW(), m.OptDenseW(), feat.DenseWidth, feat.OptionDenseWidth)
	}
	return &Weights{pn: m}, nil
}

// pnScratch is one Net's policynet forward-pass scratch, sized once for
// the most entity cards an encoding holds.
type pnScratch struct {
	m      *policynet.Model
	s      []float32 // the state trunk (H)
	part   []float32 // per hidden unit: its sum over the bias and the trunk (Hidden)
	x      []float32 // one option's hidden input (InW)
	a      []float32 // hidden activations (Hidden)
	va     []float32 // value hidden activations (ValueHidden)
	vz     []float32 // K-way value logits (ValueK)
	u      []float32 // per card: the entity encoder input (entInW)
	h      []float32 // per card: the entity encoding (EntK)
	pool   []float32 // the pooled entity vector
	set    []bool    // per max slot of pool: a card has been pooled into it
	nCards int
}

func newPNScratch(m *policynet.Model, maxCards int) *pnScratch {
	inW := m.Features.EntityRawW() + m.H
	poolW := m.Features.EntityGroupsN() * 2 * m.EntK
	return &pnScratch{
		m: m,
		s: make([]float32, m.H), part: make([]float32, m.Hidden),
		x: make([]float32, m.InW), a: make([]float32, m.Hidden),
		va: make([]float32, m.ValueHidden), vz: make([]float32, m.ValueK),
		u: make([]float32, maxCards*inW), h: make([]float32, maxCards*m.EntK),
		pool: make([]float32, poolW), set: make([]bool, poolW),
	}
}

// trunk computes the state trunk of st into p.s, the entity encodings into
// p.h (policynet Model.stateTrunkEnt's arithmetic, in its order) and each
// hidden unit's partial sum over the trunk into p.part.
func (p *pnScratch) trunk(st *policynet.State) {
	m, s := p.m, p.s
	copy(s, m.StateB)
	for _, f := range st.Sparse {
		base := int(f.Row) * m.H
		v := f.Value
		row := m.Table[base : base+m.H]
		for j := range s {
			s[j] += row[j] * v
		}
	}
	for d := 0; d < m.StateDenseW() && d < len(st.Dense); d++ {
		v := st.Dense[d]
		if v == 0 {
			continue
		}
		base := d * m.H
		for j := range s {
			s[j] += m.StateW[base+j] * v
		}
	}
	p.entForward(st)
	for q, v := range p.pool {
		if v == 0 {
			continue
		}
		row := m.EntP[q*m.H : (q+1)*m.H]
		for j := range s {
			s[j] += row[j] * v
		}
	}
	// Every option's hidden input starts with s, so the first H terms of
	// each hidden sum are the same for all of them: sum them once, in
	// forwardHead's order and with its zero skip, and score continues from
	// there.
	for h := range p.part {
		sum := m.HidB[h]
		base := h * m.InW
		for i, si := range s {
			if si != 0 {
				sum += m.HidW[base+i] * si
			}
		}
		p.part[h] = sum
	}
}

// entForward is policynet Model.entForward on the scratch.
func (p *pnScratch) entForward(st *policynet.State) {
	m := p.m
	k, rawW, groups := m.EntK, m.Features.EntityRawW(), m.Features.EntityGroupsN()
	inW := rawW + m.H
	clear(p.pool)
	clear(p.set)
	p.nCards = len(st.Cards)
	for ci := range st.Cards {
		card := &st.Cards[ci]
		u := p.u[ci*inW : (ci+1)*inW]
		clear(u)
		copy(u, card.Raw)
		for _, f := range card.Rows {
			row := m.Table[int(f.Row)*m.H : int(f.Row)*m.H+m.H]
			for j := range row {
				u[rawW+j] += row[j] * f.Value
			}
		}
		h := p.h[ci*k : (ci+1)*k]
		for q := 0; q < k; q++ {
			sum := m.EntB[q]
			w := m.EntW[q*inW : (q+1)*inW]
			for i, ui := range u {
				if ui != 0 {
					sum += w[i] * ui
				}
			}
			if sum > 0 {
				h[q] = sum
			} else {
				h[q] = 0
			}
		}
		g := int(card.Group)
		if g >= groups {
			continue
		}
		base := g * 2 * k
		for q := 0; q < k; q++ {
			p.pool[base+q] += pnEntSumScale * h[q]
			mi := base + k + q
			if !p.set[mi] || h[q] > p.pool[mi] {
				p.pool[mi] = h[q]
				p.set[mi] = true
			}
		}
	}
}

// score is policynet Model.Score's per-option arithmetic (inputVector,
// forwardHead, the bot-prior residual) against the loaded trunk. The
// trunk's inputs x[:H] are already summed in p.part, so score builds and
// reads only x[H:].
func (p *pnScratch) score(o *policynet.Option) float32 {
	m, x := p.m, p.x
	clear(x[m.H:])
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
		if r := int(f.Row); r >= 0 && r < policynet.OptionSlotWidth {
			x[off+r] += f.Value
		}
	}
	off += policynet.OptionSlotWidth
	copy(x[off:off+m.OptDenseW()], o.Dense)
	off += m.OptDenseW()
	for slot, ref := range [2]int32{o.EntA, o.EntB} {
		if ref <= 0 || int(ref) > p.nCards {
			continue
		}
		copy(x[off+slot*m.EntK:off+(slot+1)*m.EntK], p.h[int(ref-1)*m.EntK:int(ref)*m.EntK])
	}
	a := p.a
	for h := 0; h < m.Hidden; h++ {
		sum := p.part[h]
		base := h*m.InW + m.H
		for i, xi := range x[m.H:] {
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
	if o.BotPick {
		y += m.ResidualW
	}
	return y
}

// value is policynet Model.Value on the loaded trunk: the scalar head
// through the float64 sigmoid, or the K-way head's softmax component 0;
// 0.5 without a value head.
func (p *pnScratch) value() float32 {
	m := p.m
	if m.ValueHidden <= 0 {
		return 0.5
	}
	for k := 0; k < m.ValueHidden; k++ {
		sum := m.VHidB[k]
		row := m.VHidW[k*m.H : (k+1)*m.H]
		for j, sj := range p.s {
			sum += row[j] * sj
		}
		if sum > 0 {
			p.va[k] = sum
		} else {
			p.va[k] = 0
		}
	}
	if m.ValueK > 0 {
		for c := range p.vz {
			sum := m.VOutBK[c]
			base := c * m.ValueHidden
			for k, ak := range p.va {
				if ak != 0 {
					sum += m.VOutWK[base+k] * ak
				}
			}
			p.vz[c] = sum
		}
		return softmax0(p.vz)
	}
	z := m.VOutB
	for k, ak := range p.va {
		if ak != 0 {
			z += m.VOutW[k] * ak
		}
	}
	return float32(sigmoid64(float64(z)))
}

// eval runs the forward pass on st and opts, writing the option logits
// into out, and returns the value probability.
func (p *pnScratch) eval(st *policynet.State, opts []policynet.Option, out []float32) float32 {
	p.trunk(st)
	for i := range opts {
		out[i] = p.score(&opts[i])
	}
	return p.value()
}

// sigmoid64 is policynet's sigmoidFloat.
func sigmoid64(z float64) float64 {
	if z >= 0 {
		return 1 / (1 + math.Exp(-z))
	}
	e := math.Exp(z)
	return e / (1 + e)
}

// clampScore64 is policynet's clampScore.
func clampScore64(y float64) float64 {
	const lim = 1e6
	switch {
	case math.IsNaN(y):
		return 0
	case y > lim:
		return lim
	case y < -lim:
		return -lim
	}
	return y
}

// softmax0 is component 0 of policynet's softmaxVec over z, in its
// arithmetic (the float32 rounding of each exponential included).
func softmax0(z []float32) float32 {
	mx := math.Inf(-1)
	for _, x := range z {
		if f := clampScore64(float64(x)); f > mx {
			mx = f
		}
	}
	sum := 0.0
	var p0 float32
	for i, x := range z {
		e := math.Exp(clampScore64(float64(x)) - mx)
		if i == 0 {
			p0 = float32(e)
		}
		sum += e
	}
	return float32(float64(p0) / sum)
}

// pnMaxCards bounds the entity cards one encoding holds.
const pnMaxCards = dirac.MaxObj + dirac.MaxStack
