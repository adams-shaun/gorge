package kgen

// api Endure (CR 701.63; gorge effects/endure.go effEndure)
// as OpExt ExtEndure. Num$ (default 1) is read once; the Defined$ object
// (default Self) is the permanent that endures. The Spirit arm mints the
// corpus's w_x_x_spirit and sets it to N/N with a permanent override whose
// value form (ValPT) carries N: the kernel reads N from that override's PA,
// so the instruction does not store it twice. Any other parameter stays
// unread and refuses through the shared unread-params gate.
func (lc *lowerCtx) endure(ps *params) error {
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	const spirit = "w_x_x_spirit"
	if _, ok := lc.p.reg.Token(spirit); !ok {
		return refuse("token script %q not in the corpus", spirit)
	}
	// always the value form, even for a constant N: the kernel reads N
	// from PA (gorge's layer-7b SetPower$/SetToughness$ per mint)
	o := &lOverride{HasSetPT: true, PVal: n, TVal: n}
	lc.out.overrides = append(lc.out.overrides, o)
	lc.out.Tokens = append(lc.out.Tokens, spirit)
	lc.emit(lInstr{Op: "Endure", Obj: obj, A: n, C: len(lc.out.overrides) - 1, CS: spirit})
	return nil
}
