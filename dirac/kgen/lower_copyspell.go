package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// copyParentSpell lowers CopySpellAbility | Defined$ Parent (Chain
// Lightning; gorge effCopySpellAbility's Parent arm): a copy of the
// resolving instant or sorcery itself, put on the stack below it. C&1 is
// MayChooseTarget$ True; C&4 is Controller$ TargetedOrController (gorge
// copyControllerFor: target 0's player, or its object's controller), else
// the resolving controller owns the copy.
//
// Optional$ True (Sevinne's Reclamation's "you may copy this spell") is
// gorge's copy_optional yes/no, asked of the copy's controller once the copy
// is possible: an OpMayAsk under MayGuardCopyParent over the copy. Beside
// Controller$ the asked player is another one, and that shape refuses.
func (lc *lowerCtx) copyParentSpell(ps *params) error {
	if lc.ab.Kind != akSpell || lc.out.Types&^(tInstant|tSorcery) != 0 {
		return refuse("CopySpellAbility Defined$ Parent outside an instant or sorcery spell")
	}
	optional := false
	if v, ok := ps.get("Optional"); ok {
		if _, hasCtrl := ps.get("Controller"); hasCtrl || strings.TrimSpace(v) != "True" {
			return refuse("CopySpellAbility Defined$ Parent Optional$")
		}
		optional = true
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
	if optional {
		lc.emit(lInstr{Op: "MayAsk", C: kop.MayGuardCopyParent, Skip: 1, Note: "you may copy"})
	}
	lc.emit(in)
	return nil
}
