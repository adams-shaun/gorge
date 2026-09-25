package policynet

// The value-only gradient (ticket pn17): the value head's BCE gradient on a
// single (state, outcome) pair with NO offered options. It is exactly the
// value-head tail of LossGrad (net.go), extracted so the state/outcome dump
// path can descend the same arithmetic without a policy example: forward
// the state trunk, BCE on the logit, backValue into the value blocks and —
// through the trunk gradient ds — into the trunk's own blocks (StateB,
// StateW and the sparse table rows the state touched). The policy-only
// blocks (HidW, HidB, OutW, OutB) receive no gradient here, which is the
// point: a value-only run trains the value head and the shared trunk and
// never the policy read.
//
// Determinism: the summation orders are the valueForward/backValue orders
// LossGrad already pins, and the trunk backward is LossGrad's tail in the
// same order (StateB, then StateW row-major, then the sparse rows through
// addTable, which notes every touched row for Reset/ApplyGrads/Clip).

// ValueGrad evaluates one (state, target) pair's value BCE AND accumulates
// its gradients into g (g is NOT reset first — the caller accumulates
// batches, exactly like LossGrad). The model must carry a value head and
// must not carry an entity encoder (an entity trunk's backward needs the
// entity blocks' pass the value-only trainer does not exercise; the trainer
// refuses such a model before the first step).
func (m *Model) ValueGrad(st State, target float64, g *Grads) (loss float64) {
	if !m.HasValue() {
		panic("policynet.ValueGrad on a model without a value head")
	}
	if m.EntK != 0 {
		panic("policynet.ValueGrad on an entity model (refuse it in the trainer)")
	}
	s := m.StateTrunk(st)
	z := m.valueForward(s, g.va)
	var dz float64
	loss, dz = valueBCE(1, z, target)
	for j := range g.ds {
		g.ds[j] = 0
	}
	m.backValue(float32(dz), s, g)
	for j := 0; j < m.H; j++ {
		g.StateB[j] += g.ds[j]
	}
	for d := 0; d < DenseWidth && d < len(st.Dense); d++ {
		v := st.Dense[d]
		if v == 0 {
			continue
		}
		base := d * m.H
		for j := 0; j < m.H; j++ {
			g.StateW[base+j] += v * g.ds[j]
		}
	}
	for _, f := range st.Sparse {
		g.addTable(int(f.Row), g.ds, f.Value)
	}
	return loss
}
