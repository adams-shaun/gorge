package kgen

// copyParentSpell lowers CopySpellAbility | Defined$ Parent (Chain
// Lightning; gorge effCopySpellAbility's Parent arm): a copy of the
// resolving instant or sorcery itself, put on the stack below it. C&1 is
// MayChooseTarget$ True; C&4 is Controller$ TargetedOrController (gorge
// copyControllerFor: target 0's player, or its object's controller), else
// the resolving controller owns the copy.
func (lc *lowerCtx) copyParentSpell(ps *params) error {
	if lc.ab.Kind != akSpell || lc.out.Types&^(tInstant|tSorcery) != 0 {
		return refuse("CopySpellAbility Defined$ Parent outside an instant or sorcery spell")
	}
	if _, ok := ps.get("Optional"); ok {
		return refuse("CopySpellAbility Defined$ Parent Optional$")
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "CopySpell", Obj: lRef{Kind: rSelf}, A: n}
	if ps.is("MayChooseTarget", "True") {
		in.C = 1
	}
	if c, ok := ps.get("Controller"); ok {
		switch c {
		case "You":
		case "TargetedOrController", "TargetedController", "TargetedPlayer", "Targeted":
			if len(lc.ab.Targets) != 1 {
				return refuse("CopySpellAbility Controller$ %q over %d target slots", c, len(lc.ab.Targets))
			}
			in.C |= 4
		default:
			return refuse("CopySpellAbility Controller$ %q", c)
		}
	}
	lc.emit(in)
	return nil
}
