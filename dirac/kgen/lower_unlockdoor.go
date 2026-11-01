package kgen

// DB$ UnlockDoor (CR 709.5f-g; gorge
// effects/unlockdoor.go effUnlockDoor). The effect takes exactly one target
// slot, ValidTgts$ Room.YouCtrl, with TargetMin$ 0 or 1 and TargetMax$ 1
// (Ghostly Keybearer's "up to one target Room you control"; Keys to the House
// and Marina Vendrell's single target). Mode$ Unlock (0) unlocks a locked
// door; Mode$ LockOrUnlock (1) first asks the controller whether to lock or
// unlock (gorge's "Lock or unlock a door?", option 0 = unlock). Every other
// Mode$ refuses by name. The kernel swaps the Room permanent's door state
// without paying (the effect is not the unlock special action).
func (lc *lowerCtx) unlockDoor(ps *params) error {
	mode := ps.str("Mode")
	c := 0
	switch mode {
	case "Unlock":
	case "LockOrUnlock":
		c = 1
	default:
		return refuse("UnlockDoor Mode %q", mode)
	}
	if vt := ps.str("ValidTgts"); vt != "Room.YouCtrl" {
		return refuse("UnlockDoor ValidTgts %q", vt)
	}
	if _, ok := ps.get("Defined"); ok {
		return refuse("UnlockDoor Defined$")
	}
	ps.consume("TgtPrompt")
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return err
	}
	if obj.Kind != rTarget {
		return refuse("UnlockDoor without a target slot")
	}
	t := &lc.ab.Targets[obj.N]
	if t.Zone != zBattlefield {
		return refuse("UnlockDoor target zone %s", zoneLabel[t.Zone])
	}
	if t.Min.Kind != vConst || t.Max.Kind != vConst || t.Max.N != 1 || (t.Min.N != 0 && t.Min.N != 1) {
		return refuse("UnlockDoor target count %s..%s", t.Min, t.Max)
	}
	// Every other target rider would change which Room (or how many) the
	// effect sees; only the plain 0..1 / 1..1 slot is read.
	if t.Unique || t.SameCtrl || t.DiffCtrl || t.SharedCrType || t.DistinctName || t.MaxMV != 0 ||
		t.Divided != 0 || t.LKIDiv || t.ParentPlus1 != 0 || t.PermOrSpell || t.Spell || t.StackAb || t.AbOnly || t.TrigOnly || t.VTgt != nil || t.CopyAb != 0 || t.SingleTgt || t.AtRes || t.ChgSub {
		return refuse("UnlockDoor target rider on the Room slot")
	}
	lc.emit(lInstr{Op: "UnlockDoor", Obj: obj, C: c})
	return nil
}
