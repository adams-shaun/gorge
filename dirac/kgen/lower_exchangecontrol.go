package kgen

// DB$ ExchangeControl (CR 701.12; gorge effects/exchange_control.go
// effExchangeControl). The pair it swaps is one of two shapes, both lowered
// to the target slots it reads at resolution (kop.ExtExchangeControl):
//
//   - one slot of exactly two battlefield targets (ValidTgts$ with TargetMin$
//     2 and TargetMax$ 2, optionally TargetsWithDifferentControllers$);
//   - an earlier link's single battlefield target (Defined$ ParentTarget) and
//     this link's own single battlefield target;
//   - the source permanent (Defined$ Self) and this link's own single
//     battlefield target (exchangeControlSelf).
//
// Every other parameter (TargetsAtRandom$, RememberExchanged$, ...) stays
// unread, so the node refuses by name.
func (lc *lowerCtx) exchangeControl(ps *params) error {
	if _, ok := ps.get("ValidTgts"); !ok {
		return refuse("ExchangeControl without ValidTgts$")
	}
	if d, ok := ps.get("Defined"); ok && d == "Self" {
		return lc.exchangeControlSelf(ps)
	}
	if d, ok := ps.get("Defined"); ok && d != "ParentTarget" {
		return refuse("ExchangeControl Defined$ %q", d)
	}
	_, defined := ps.get("Defined")
	ref, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	own := lc.lastTarget()
	if own < 0 || ref.Kind != rTarget {
		return refuse("ExchangeControl without target slots")
	}
	slots := []int{own}
	if defined {
		slots = []int{ref.N, own}
	}
	for _, n := range slots {
		t := lc.ab.Targets[n]
		if t.Zone != zBattlefield || t.F == nil || t.F.Players != 0 || len(t.F.Clauses) == 0 {
			return refuse("ExchangeControl target slot %d is not a battlefield object", n)
		}
		want := 1
		if !defined {
			want = 2
		}
		if !isConstN(t.Min, want) || !isConstN(t.Max, want) {
			return refuse("ExchangeControl target count %s..%s", t.Min, t.Max)
		}
	}
	if defined && lc.ab.Targets[own].DiffCtrl {
		return refuse("ExchangeControl TargetsWithDifferentControllers$ on a one-object slot")
	}
	mask := 0
	for _, n := range slots {
		mask |= 1 << n
	}
	lc.emit(lInstr{Op: "ExchangeControl", C: mask})
	return nil
}

// exchangeControlSelf is Defined$ Self beside ValidTgts$ (Avarice Totem,
// Phyrexian Infiltrator: "exchange control of CARDNAME and target ..."). gorge
// effExchangeControl resolves Defined$ without the ValidTgts$ shortcut, so
// the pair is the source then the one target: Obj RefSelf puts the source
// first. The source must still be the same object on the battlefield, and
// a source that targeted itself is a pair of one, which changes nothing.
func (lc *lowerCtx) exchangeControlSelf(ps *params) error {
	// the target slot is read without Defined$ (objOrTarget would let
	// Defined$ win as the subject), as phases' Self & Targeted does
	m := make(map[string]string, len(ps.m))
	for k, v := range ps.m {
		if k != "Defined" {
			m[k] = v
		}
	}
	ref, err := lc.objOrTarget(&params{m: m, used: ps.used}, "", zBattlefield)
	if err != nil {
		return err
	}
	if ref.Kind != rTarget {
		return refuse("ExchangeControl Defined$ Self without a target slot")
	}
	t := lc.ab.Targets[ref.N]
	if t.Zone != zBattlefield || t.F == nil || t.F.Players != 0 || len(t.F.Clauses) == 0 {
		return refuse("ExchangeControl Defined$ Self: target slot %d is not a battlefield object", ref.N)
	}
	if !isConstN(t.Min, 1) || !isConstN(t.Max, 1) {
		return refuse("ExchangeControl Defined$ Self: target count %s..%s", t.Min, t.Max)
	}
	if t.DiffCtrl || t.SameCtrl {
		return refuse("ExchangeControl Defined$ Self beside a target-controller rider")
	}
	lc.emit(lInstr{Op: "ExchangeControl", Obj: lRef{Kind: rSelf}, C: 1 << ref.N})
	return nil
}
