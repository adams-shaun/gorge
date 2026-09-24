package policynet

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// ppoFixture is valueFixture turned into a PPO example: π_old is the
// model's own current scores nudged by a small deterministic perturbation (so
// the ratio is near, not at, 1 and the KL is non-zero), option 1 was played,
// and the advantage is set. Unlabelled option 3 sits outside the action space.
func ppoFixture(t *testing.T, subset bool, seed uint64) (*Model, Example, LossConfig) {
	t.Helper()
	m, ex, lc := valueFixture(rand.New(rand.NewPCG(seed, seed+1)))
	ex.Options[3].Target.Labelled = false
	for i := range ex.Options {
		ex.Options[i].Target.Preferred = false
	}
	now := m.Score(ex.State, ex.Options)
	old := make([]float32, len(now))
	for i := range now {
		old[i] = now[i] + 0.03*float32(i%3-1) + 0.01
	}
	chosen := []bool{false, true, false, false}
	if subset {
		chosen = []bool{true, true, false, false}
	}
	ex.PPO = &PPOTarget{Subset: subset, OldScores: old, Chosen: chosen, Advantage: 0.8}
	lc.PPOClip, lc.PPOKL = 0.2, 0.35
	lc.ValueBlend = 0 // the on-policy corpus has outcomes only
	return m, ex, lc
}

func fdAll(t *testing.T, m *Model, ex Example, lc LossConfig, g *Grads) {
	t.Helper()
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

// TestPPOGradientFiniteDifference pins the hand-derived PPO gradient (the
// unclipped surrogate, the exact KL anchor and the joint value term) against
// a central finite difference of the forward-only loss, for both action
// distributions, on every parameter block.
func TestPPOGradientFiniteDifference(t *testing.T) {
	for _, subset := range []bool{false, true} {
		m, ex, lc := ppoFixture(t, subset, 31)
		g := m.NewGrads()
		g.Zero()
		st := m.LossGrad(ex, lc, g)
		if !st.PPO.On || st.PPO.Clipped || st.PPO.KL <= 0 || st.Parts.ValueHead <= 0 {
			t.Fatalf("subset=%v: fixture must be unclipped with a positive KL and value term: %+v", subset, st)
		}
		if r := math.Exp(st.PPO.LogRatio); math.Abs(r-1) > 0.15 {
			t.Fatalf("subset=%v: ratio %g too close to the clip for a clean FD", subset, r)
		}
		if lst := m.Loss(ex, lc); lst.Loss != st.Loss || lst.Parts != st.Parts || lst.PPO != st.PPO {
			t.Fatalf("Loss %+v and LossGrad %+v disagree", lst, st)
		}
		fdAll(t, m, ex, lc, g)
	}
}

// TestPPOClippedBranchCarriesOnlyTheKL: a ratio outside the clip on the side
// the advantage favours takes the clipped (constant) branch, so the only
// policy gradient left is the KL anchor's; with β 0 there is none at all.
func TestPPOClippedBranchCarriesOnlyTheKL(t *testing.T) {
	m, ex, lc := ppoFixture(t, false, 41)
	lc.ValueWeight = 0
	now := m.Score(ex.State, ex.Options)
	// Push π_old's chosen score DOWN so π_new(chosen) ≫ π_old: r > 1+ε with A > 0.
	for i := range ex.PPO.OldScores {
		ex.PPO.OldScores[i] = now[i]
	}
	ex.PPO.OldScores[1] -= 1.5
	g := m.NewGrads()
	g.Zero()
	st := m.LossGrad(ex, lc, g)
	if !st.PPO.Clipped || math.Exp(st.PPO.LogRatio) <= 1.2 {
		t.Fatalf("want the clipped branch: %+v", st.PPO)
	}
	fdAll2 := func() { fdCheck(t, m, ex, lc, "hidden", m.HidW, g.HidW) }
	fdAll2()
	lc.PPOKL = 0
	g.Zero()
	m.LossGrad(ex, lc, g)
	if g.Norm() != 0 {
		t.Fatalf("clipped branch with β 0 must carry no gradient, norm %g", g.Norm())
	}
}

// TestPPOAtRatioOneIsThePolicyGradient: when π_new = π_old the surrogate is
// −A, the KL 0, and the logit gradient is −A·∂log π — exactly the vanilla
// policy gradient. Checked through the loss value on both shapes.
func TestPPOAtRatioOneIsThePolicyGradient(t *testing.T) {
	for _, subset := range []bool{false, true} {
		m, ex, lc := ppoFixture(t, subset, 51)
		lc.ValueWeight = 0
		now := m.Score(ex.State, ex.Options)
		copy(ex.PPO.OldScores, now)
		st := m.Loss(ex, lc)
		if st.PPO.LogRatio != 0 || st.PPO.KL != 0 || st.Loss != -ex.PPO.Advantage {
			t.Fatalf("subset=%v: at ratio 1 want loss −A=%g, KL 0; got %+v", subset, -ex.PPO.Advantage, st)
		}
		lp, ok := PPOLogProb(ex.Options, now, ex.PPO.Chosen, subset)
		if !ok || math.Abs(lp-st.PPO.LogPOld) > 1e-12 {
			t.Fatalf("subset=%v: PPOLogProb %g vs loss's log π_old %g", subset, lp, st.PPO.LogPOld)
		}
	}
}

// TestPPOLogProbKnownAnswer pins both shapes on hand-computed numbers, and
// that unlabelled options are outside the action space.
func TestPPOLogProbKnownAnswer(t *testing.T) {
	opts := []Option{{Target: OptionTarget{Labelled: true}}, {Target: OptionTarget{Labelled: true}}, {}}
	scores := []float32{0, float32(math.Log(3)), 100}
	lp, ok := PPOLogProb(opts, scores, []bool{false, true, false}, false)
	if !ok || math.Abs(lp-math.Log(0.75)) > 1e-7 {
		t.Fatalf("softmax log π = %g, want log 0.75", lp)
	}
	if _, ok := PPOLogProb(opts, scores, []bool{true, true, false}, false); ok {
		t.Fatal("a softmax answer with two chosen options has no probability")
	}
	lp, ok = PPOLogProb(opts, scores, []bool{true, false, false}, true)
	// σ(0)·(1−σ(log 3)) = 0.5 · 0.25
	if !ok || math.Abs(lp-math.Log(0.125)) > 1e-7 {
		t.Fatalf("subset log π = %g, want log 0.125", lp)
	}
}

// TestPPOPolicyOffTrainsOnlyTheValueHead: an excluded kind's example has no
// policy term and its gradient is exactly the value term's.
func TestPPOPolicyOffTrainsOnlyTheValueHead(t *testing.T) {
	m, ex, lc := ppoFixture(t, true, 61)
	ex.PPO.PolicyOff = true
	g := m.NewGrads()
	g.Zero()
	st := m.LossGrad(ex, lc, g)
	if st.PPO.On || st.Parts.Rank != 0 || st.Parts.ValueHead <= 0 {
		t.Fatalf("PolicyOff: %+v", st)
	}
	for _, blk := range [][]float32{g.HidW, g.HidB, g.OutW} {
		for _, v := range blk {
			if v != 0 {
				t.Fatal("PolicyOff moved the policy head")
			}
		}
	}
	fdCheck(t, m, ex, lc, "table", m.Table, g.Table)
}

// TestOnPolicyRecordRoundTrip: an encoded state and options written through
// the corpus record's JSON and decoded back score bit-identically — the
// property the trainer's π_old re-score check relies on.
func TestOnPolicyRecordRoundTrip(t *testing.T) {
	m, ex, _ := ppoFixture(t, true, 71)
	ex.Options[2].BotPick = true
	scores := m.Score(ex.State, ex.Options)
	rec := OnPolicyRecord{RecordType: OnPolicyRecordType, SchemaVersion: OnPolicySchemaVersion, Kind: "attackers",
		Subset: true, State: EncodeOnPolicyState(ex.State), Scores: scores, Chosen: []int{0, 2}, Outcome: 1, OutcomeKnown: true}
	for i, o := range ex.Options {
		rec.Options = append(rec.Options, EncodeOnPolicyOption(o, o.Target.Labelled || i == 3))
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&rec); err != nil {
		t.Fatal(err)
	}
	var back OnPolicyRecord
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	got, err := back.Example()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Score(got.State, got.Options), scores) {
		t.Fatal("decoded example scores differently from the recorded scores")
	}
	if !slices.Equal(got.PPO.OldScores, scores) || !slices.Equal(got.PPO.Chosen, []bool{true, false, true, false}) ||
		!got.Options[2].BotPick || !got.Options[3].Target.Labelled || !got.PPO.Subset || !got.HasOutcome || got.Outcome != 1 {
		t.Fatalf("decoded PPO target %+v / options wrong", got.PPO)
	}
}

// TestVDWMGradientFiniteDifference pins the VDWM margin term's gradient
// (plus the joint value term) against a central finite difference on every
// block, for both shapes, with the hinge active.
func TestVDWMGradientFiniteDifference(t *testing.T) {
	for _, subset := range []bool{false, true} {
		m, ex, lc := ppoFixture(t, subset, 81)
		ex.PPO.VDWM, ex.PPO.Weight = true, 1.7
		lc.VDWMMargin = 5 // wide enough that every hinge is active, far from its kink
		g := m.NewGrads()
		g.Zero()
		st := m.LossGrad(ex, lc, g)
		if !st.PPO.On || !st.PPO.MarginActive || st.Parts.Rank <= 0 || st.Parts.ValueHead <= 0 {
			t.Fatalf("subset=%v: fixture must have an active margin and value term: %+v", subset, st)
		}
		if lst := m.Loss(ex, lc); lst.Loss != st.Loss || lst.PPO != st.PPO {
			t.Fatalf("Loss %+v and LossGrad %+v disagree", lst, st)
		}
		fdAll(t, m, ex, lc, g)
	}
}

// TestVDWMKnownAnswer pins the loss on hand-computed numbers: the softmax
// hinge m − z_y + max_{j≠y} z_j times (1 − p_old(y))·Weight, zero once the
// margin clears; the subset per-option hinges with their own (1 − q) factors.
func TestVDWMKnownAnswer(t *testing.T) {
	lab := OptionTarget{Labelled: true}
	opts := []Option{{Target: lab}, {Target: lab}, {Target: lab}}
	ex := Example{Options: opts, PPO: &PPOTarget{VDWM: true, Weight: 2,
		OldScores: []float32{0, float32(math.Log(3)), 0}, Chosen: []bool{false, true, false}}}
	lc := LossConfig{VDWMMargin: 1}
	// p_old(y) = 3/5 → factor 0.4; hinge 1 − 0.5 + 0.2 = 0.7.
	parts, dys, st := lossVDWM(lc, ex, []int{0, 1, 2}, []float64{0.2, 0.5, -1})
	if math.Abs(parts.Rank-2*0.4*0.7) > 1e-6 || math.Abs(dys[1]+0.8) > 1e-6 || math.Abs(dys[0]-0.8) > 1e-6 || dys[2] != 0 || !st.MarginActive {
		t.Fatalf("softmax: parts %+v dys %v", parts, dys)
	}
	if parts, _, st := lossVDWM(lc, ex, []int{0, 1, 2}, []float64{0.2, 1.3, -1}); parts.Rank != 0 || st.MarginActive {
		t.Fatalf("a cleared margin must carry no loss: %+v", parts)
	}
	if w := VDWMRowWeight(ex); math.Abs(w-0.4) > 1e-6 {
		t.Fatalf("row weight %g, want 0.4", w)
	}
	ex.PPO.Subset = true
	ex.PPO.OldScores = []float32{0, 0, 0}
	ex.PPO.Chosen = []bool{true, false, false}
	// q = ½ everywhere → factor ½. Hinges: option 0 (chosen, t=+1) 1 − 0.2 =
	// 0.8; option 1 (t=−1) 1 + 0.5 = 1.5; option 2 (t=−1) 1 − 1 = 0, cleared.
	parts, dys, _ = lossVDWM(lc, ex, []int{0, 1, 2}, []float64{0.2, 0.5, -1})
	want := 2 * 0.5 * (0.8 + 1.5)
	if math.Abs(parts.Rank-want) > 1e-12 || dys[0] != -1 || dys[1] != 1 || dys[2] != 0 {
		t.Fatalf("subset: parts %+v dys %v (want %g)", parts, dys, want)
	}
}
