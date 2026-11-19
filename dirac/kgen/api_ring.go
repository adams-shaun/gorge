package kgen

// api_ring.go lowers api RingTemptsYou (gorge engine/effects/ring.go
// effRingTemptsYou): the Ring tempts the resolving controller (CR 701.54a).
// The corpus carries no Defined$ and no params, so the lowering reads none;
// the kernel's half is dirac/act_ring.go.
func init() {
	apiPS("RingTemptsYou", (*lowerCtx).ringTempts)
}

func (lc *lowerCtx) ringTempts(ps *params) error {
	lc.emit(lInstr{Op: "RingTemptsYou", Obj: lRef{Kind: rYou}, Note: "the Ring tempts the controller (CR 701.54a; dirac/act_ring.go)"})
	return nil
}
