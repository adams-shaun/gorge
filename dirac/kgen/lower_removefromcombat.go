package kgen

// removeFromCombat lowers api:RemoveFromCombat (gorge effRemoveFromCombat,
// engine/effects/combatfx.go: each resolved object on the battlefield gets
// one EndCombatReset) to OpExt ExtRemoveFromCombat over one object: a
// single target slot, the source, the trigger object (TriggeredAttacker /
// TriggeredBlocker and their LKICopy spellings) or the source's host.
// RememberRemovedFromCombat$ (Illusionist's Gambit) and
// UnblockCreaturesBlockedOnlyBy$ (Balduvian Warlord, False Orders) refuse;
// a several-object Defined$ refuses.
func (lc *lowerCtx) removeFromCombat(ps *params) error {
	for _, k := range []string{"RememberRemovedFromCombat", "UnblockCreaturesBlockedOnlyBy"} {
		if _, ok := ps.get(k); ok {
			return refuse("RemoveFromCombat %s$", k)
		}
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rSelf, rTriggered, rEquipped:
	case rTarget:
		t := lc.ab.Targets[obj.N]
		if !t.Max.isConst() || t.Max.N != 1 || t.F == nil || t.F.Players != 0 {
			return refuse("RemoveFromCombat of a target slot other than one object")
		}
	default:
		return refuse("RemoveFromCombat of %s", obj)
	}
	lc.emit(lInstr{Op: "RemoveFromCombat", Obj: obj})
	return nil
}
