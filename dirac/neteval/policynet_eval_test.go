package neteval

import (
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
)

// policynetCheckpoint writes a tiny random-weight FeaturesEntity checkpoint
// (an L9c policytrain output's layout) under t's temp directory: an entity
// encoder, a value head (valueK > 0: the K-way one) and a nonzero bot-prior
// residual, every block nonzero so no arithmetic path is skipped.
func policynetCheckpoint(t testing.TB, valueK int) (string, *policynet.Model) {
	t.Helper()
	rng := rand.New(rand.NewPCG(7, uint64(11+valueK)))
	m := policynet.NewModel(policynet.TableRows, 8, 16, rng)
	m.Features = policynet.FeaturesMZ
	if err := policynet.UpgradeEntity(m, 4, rng); err != nil {
		t.Fatal(err)
	}
	for _, blk := range [][]float32{m.EntP, m.HidW, m.EntB, m.StateB, m.HidB} {
		for i := range blk {
			if blk[i] == 0 {
				blk[i] = float32(rng.NormFloat64()) * 0.2
			}
		}
	}
	m.InitValue(12, rng)
	for i := range m.VHidB {
		m.VHidB[i] = float32(rng.NormFloat64()) * 0.1
	}
	m.VOutB, m.OutB, m.ResidualW = 0.3, -0.2, 0.75
	if valueK > 0 {
		m.ValueK = valueK
		m.VOutWK = make([]float32, valueK*m.ValueHidden)
		m.VOutBK = make([]float32, valueK)
		for i := range m.VOutWK {
			m.VOutWK[i] = float32(rng.NormFloat64()) * 0.5
		}
		for i := range m.VOutBK {
			m.VOutBK[i] = float32(rng.NormFloat64()) * 0.1
		}
	}
	p := filepath.Join(t.TempDir(), "policy.ckpt")
	if err := m.SaveCheckpoint(p); err != nil {
		t.Fatal(err)
	}
	return p, m
}

func policynetNet(t testing.TB, valueK int) (*Net, *policynet.Model) {
	t.Helper()
	p, _ := policynetCheckpoint(t, valueK)
	w, err := LoadPolicynet(p)
	if err != nil {
		t.Fatal(err)
	}
	if w.Backend() != "policynet" {
		t.Fatalf("loaded through %s, want policynet", w.Backend())
	}
	ref, err := policynet.LoadCheckpointFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return New(w, vanillaPool()), ref
}

// gorgeInputs is policynet's FeaturesEntity encoding of s at d for seat,
// copied out of a fresh feat encoder (dirac/feat writes gorge's encoding:
// dirac/oracle's FeatCheck holds that), with bot's options marked BotPick
// as azmcts marks them.
func gorgeInputs(enc *feat.Encoder, s *dirac.State, d *dirac.Decision, seat uint8, bot []bool) (policynet.State, []policynet.Option) {
	fs := enc.EncodeStateFor(s, d, seat)
	st := policynet.State{Dense: append([]float32(nil), fs.Dense[:]...)}
	for _, f := range fs.Sparse {
		st.Sparse = append(st.Sparse, policynet.Feature{Row: f.Row, Value: f.Value})
	}
	for i := range fs.Cards {
		c := &fs.Cards[i]
		ec := policynet.EntityCard{Group: c.Group, Raw: append([]float32(nil), c.Raw[:]...)}
		for _, f := range c.Rows[:c.NRows] {
			ec.Rows = append(ec.Rows, policynet.Feature{Row: f.Row, Value: f.Value})
		}
		st.Cards = append(st.Cards, ec)
	}
	if bot == nil {
		return st, nil
	}
	botAt := -1
	for i := 0; d.Kind == dirac.DkPriority && i < int(d.N) && i < len(bot); i++ {
		if bot[i] {
			botAt = i
			break
		}
	}
	fo := enc.EncodeOptionsBot(d, nil, botAt)
	opts := make([]policynet.Option, d.N)
	for i := range opts {
		f := &fo[i]
		o := policynet.Option{Dense: append([]float32(nil), f.Dense[:]...), EntA: f.EntA, EntB: f.EntB, BotPick: i < len(bot) && bot[i]}
		for _, x := range f.Slots[:f.NSlots] {
			o.Slots = append(o.Slots, policynet.Feature{Row: x.Row, Value: x.Value})
		}
		for _, x := range f.Hashed[:f.NHashed] {
			o.Hashed = append(o.Hashed, policynet.Feature{Row: x.Row, Value: x.Value})
		}
		opts[i] = o
	}
	return st, opts
}

// The policynet backend's priors and value on dirac positions are
// float-identical to bots/policynet's own forward pass (Model.Score,
// Model.Value: azmcts's prior and leaf) on the same FeaturesEntity inputs.
func TestPolicynetEvalMatchesGorge(t *testing.T) {
	for _, k := range []int{0, 3} {
		n, ref := policynetNet(t, k)
		enc := feat.NewEncoder(n.enc.Pool())
		var d dirac.Decision
		var out [dirac.MaxOptions]float32
		checked, picked, entRefs, cards := 0, 0, 0, 0
		for i, s := range positions(60) {
			s.StepInto(&d)
			if d.N == 0 {
				continue
			}
			bot := make([]bool, d.N)
			bot[i%int(d.N)] = true
			n.NewWorld()
			n.Priors(s, &d, bot, out[:d.N])
			st, opts := gorgeInputs(enc, s, &d, d.Player, bot)
			want := ref.Score(st, opts)
			for j := range want {
				if out[j] != want[j] {
					t.Fatalf("valueK %d position %d option %d: logit %v, policynet %v", k, i, j, out[j], want[j])
				}
				if opts[j].BotPick {
					picked++
				}
				if opts[j].EntA > 0 || opts[j].EntB > 0 {
					entRefs++
				}
			}
			cards += len(st.Cards)
			for _, seat := range []uint8{d.Player, 1 - d.Player} {
				n.NewWorld()
				got := n.Value(s, &d, seat)
				vst, _ := gorgeInputs(enc, s, &d, seat, nil)
				if wv := ref.Value(vst); got != wv || wv == 0.5 {
					t.Fatalf("valueK %d position %d seat %d: value %v, policynet %v", k, i, seat, got, wv)
				}
			}
			checked++
		}
		// the comparison reached every path it claims: options marked for
		// the residual, entity references, entity cards pooled
		if checked < 40 || picked == 0 || entRefs == 0 || cards == 0 {
			t.Fatalf("valueK %d: %d positions, %d bot picks, %d entity refs, %d cards", k, checked, picked, entRefs, cards)
		}
	}
}

// The policynet backend allocates nothing per evaluation once its scratch
// has grown (the evaluation cache off: every call runs the forward pass).
func TestPolicynetEvalZeroAllocs(t *testing.T) {
	for _, k := range []int{0, 3} {
		n, _ := policynetNet(t, k)
		n.cache = nil
		ps := positions(40)
		var d dirac.Decision
		var logits [dirac.MaxOptions]float32
		bot := make([]bool, dirac.MaxOptions)
		bot[0] = true
		run := func() {
			for i := range ps {
				ps[i].StepInto(&d)
				n.NewWorld()
				n.Priors(ps[i], &d, bot[:d.N], logits[:d.N])
				n.Value(ps[i], &d, 1-d.Player)
			}
		}
		run()
		evals := n.Evals
		if a := testing.AllocsPerRun(5, run); a != 0 {
			t.Fatalf("valueK %d: allocs per pass over %d positions = %v", k, len(ps), a)
		}
		if n.Evals == evals {
			t.Fatalf("valueK %d: no forward pass ran", k)
		}
	}
}
