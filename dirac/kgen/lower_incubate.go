package kgen

// incubate lowers api:Incubate (gorge effIncubate, CR 701.53a-b): the
// controller (or, with Defined$ TargetedController, the targeted
// permanent's controller) creates the Incubator token Amount$ times and
// puts Amount$ +1/+1 counters on each mint. The token is the corpus's one
// incubator script; TokenScript$ would override it and no carrier does, so
// it refuses. Times$ is read exactly, like Amount$.
func (lc *lowerCtx) incubate(ps *params) error {
	if _, ok := ps.get("TokenScript"); ok {
		return refuse("Incubate TokenScript$")
	}
	owner := lRef{Kind: rYou}
	if d, ok := ps.get("Defined"); ok {
		if d != "TargetedController" {
			return refuse("Incubate Defined$ %q", d)
		}
		var err error
		if owner, err = lc.defined(d); err != nil {
			return err
		}
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	times, err := lc.amount(ps, "Times", 1)
	if err != nil {
		return err
	}
	const key = "incubator_c_0_0_a_phyrexian"
	if _, ok := lc.p.reg.Token(key); !ok {
		return refuse("token script %q not in the corpus", key)
	}
	// The token's front is a pool token entry and its back face rides
	// beside it (lower_faces.go); lowerPool collects only the CARDS' token
	// lists, so both keys are appended here.
	lc.out.Tokens = append(lc.out.Tokens, key, faceKey(key, 1))
	lc.emit(lInstr{Op: "Incubate", Obj: owner, A: n, B: times, CS: key})
	return nil
}
