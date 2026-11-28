package neteval

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
)

// scoreRef is the full per-option forward pass score replaces: every hidden
// sum starts at the bias and runs over the whole input vector, the trunk
// state included.
func scoreRef(p *pnScratch, o *policynet.Option) float32 {
	m := p.m
	x := make([]float32, m.InW)
	copy(x, p.s)
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
	a := make([]float32, m.Hidden)
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

// mzModel is policynetCheckpoint's model before the entity upgrade: the mz
// feature set. LoadPolicynet refuses it, so the test builds its scratch
// directly.
func mzModel() *policynet.Model {
	rng := rand.New(rand.NewPCG(7, 23))
	m := policynet.NewModel(policynet.TableRows, 8, 16, rng)
	m.Features = policynet.FeaturesMZ
	for _, blk := range [][]float32{m.HidW, m.StateB, m.HidB} {
		for i := range blk {
			if blk[i] == 0 {
				blk[i] = float32(rng.NormFloat64()) * 0.2
			}
		}
	}
	m.OutB, m.ResidualW = -0.2, 0.75
	return m
}

// score, which starts each hidden sum at the per-decision trunk partial, is
// bit-identical to the full forward pass (scoreRef) on both feature sets.
func TestScorePartialIsExact(t *testing.T) {
	const states = 200
	_, entRef := policynetNet(t, 0)
	_, entKRef := policynetNet(t, 3)
	ps := positions(4 * states)
	enc := feat.NewEncoder(vanillaPool())
	for _, arm := range []struct {
		name string
		m    *policynet.Model
	}{{"mz", mzModel()}, {"entity", entRef}, {"entity-valueK", entKRef}} {
		t.Run(arm.name, func(t *testing.T) {
			m := arm.m
			entity := m.Features == policynet.FeaturesEntity
			if !entity && (m.Features != policynet.FeaturesMZ || m.EntK != 0) {
				t.Fatalf("arm built on %s with EntK %d", m.Features, m.EntK)
			}
			p := newPNScratch(m, pnMaxCards)
			var d dirac.Decision
			checked, options, entRefs, picked, trunkMoved, partRead := 0, 0, 0, 0, 0, 0
			distinct := map[uint32]bool{}
			for i, s := range ps {
				if checked == states {
					break
				}
				s.StepInto(&d)
				if d.N == 0 {
					continue
				}
				bot := make([]bool, d.N)
				bot[i%int(d.N)] = true
				st, opts := gorgeInputs(enc, s, &d, d.Player, bot)
				if !entity {
					st.Cards = nil
				}
				// one scratch across states: a partial left over from the
				// previous decision differs from scoreRef
				p.trunk(&st)
				for j := range opts {
					got, want := p.score(&opts[j]), scoreRef(p, &opts[j])
					if math.Float32bits(got) != math.Float32bits(want) {
						t.Fatalf("state %d option %d: score %v (%08x), full pass %v (%08x)", checked, j,
							got, math.Float32bits(got), want, math.Float32bits(want))
					}
					distinct[math.Float32bits(got)] = true
					options++
					if opts[j].BotPick {
						picked++
					}
					if p.nCards > 0 && (opts[j].EntA > 0 || opts[j].EntB > 0) {
						entRefs++
					}
				}
				for h := range p.part {
					if p.part[h] != m.HidB[h] {
						trunkMoved++
						break
					}
				}
				// score reads the partial: moving it moves the score
				before := p.score(&opts[0])
				for h := range p.part {
					p.part[h] += 1000
				}
				if p.score(&opts[0]) != before {
					partRead++
				}
				checked++
			}
			if checked != states || options <= states || picked == 0 || len(distinct) < states {
				t.Fatalf("%d states, %d options, %d bot picks, %d distinct scores", checked, options, picked, len(distinct))
			}
			if trunkMoved != states || partRead != states {
				t.Fatalf("of %d states: the trunk moved the partial in %d, score read it in %d", states, trunkMoved, partRead)
			}
			if entity != (entRefs > 0) {
				t.Fatalf("entity %v: %d entity references", entity, entRefs)
			}
		})
	}
}
