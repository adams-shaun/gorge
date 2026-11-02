package kgen

// regenerate lowers api:Regenerate (gorge effRegenerate: one this-turn
// shield on each Defined$ object, default Self, or the target). The
// shields stack and each destruction spends one (CR 701.19a; kernel
// addRegenShield / regenerate).
func (lc *lowerCtx) regenerate(ps *params) error {
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Regenerate", Obj: obj})
	return nil
}

// noRegen reads Destroy/DestroyAll's NoRegen$: gorge compares it against
// "True" (effDestroy / effDestroyAll skip ReplaceDestruction), so an
// explicit False permits regeneration. Anything else refuses.
func noRegen(ps *params) (int, error) {
	v, ok := ps.get("NoRegen")
	if !ok {
		return 0, nil
	}
	switch v {
	case "True":
		return 1, nil
	case "False":
		return 0, nil
	}
	return 0, refuse("NoRegen %q", v)
}
