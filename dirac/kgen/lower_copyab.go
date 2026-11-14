package kgen

import "strings"

// copyAbility lowers "copy target activated or triggered ability you control;
// you may choose new targets for the copy" (gorge
// effCopySpellAbility's targeted arm with TargetType$
// Activated.YouCtrl,Triggered.YouCtrl or Triggered.YouCtrl): one copy of the
// first object target, an ability on the stack its chooser controls. The
// ability's types are its source's (CR 113.7a; gorge matchesTargetSpec binds
// the source's derived types to the wrapper), so ValidTgts$ Card / Card,Emblem
// name every source and a bare card type (Card.Creature, Artifact) filters by
// the source's types; anything else refuses. MayChooseTarget$ must be True.
// Amount$ is the copy count (default 1; Gogo's X, Mister Fantastic's 2).
func (lc *lowerCtx) copyAbility(ps *params) error {
	var kind uint8
	switch tt := ps.str("TargetType"); tt {
	case "Activated.YouCtrl,Triggered.YouCtrl":
		kind = 1
	case "Triggered.YouCtrl":
		kind = 2
	default:
		return refuse("CopySpellAbility TargetType %q", tt)
	}
	for _, k := range []string{"Defined", "Controller", "CopyForEachCanTarget", "TargetMin", "TargetMax"} {
		if _, ok := ps.get(k); ok {
			return refuse("CopySpellAbility of an ability with %s$", k)
		}
	}
	if !ps.is("MayChooseTarget", "True") {
		return refuse("CopySpellAbility of an ability without MayChooseTarget$ True")
	}
	vt := ps.str("ValidTgts")
	if vt == "Card,Emblem" {
		vt = "Card"
	}
	f, err := lc.filter(vt)
	if err != nil {
		return err
	}
	if f == nil || f.Players != 0 || len(f.Clauses) != 1 || f.Clauses[0] != (lClause{Types: f.Clauses[0].Types}) {
		return refuse("CopySpellAbility of an ability whose ValidTgts$ %q reads more than the source's types", vt)
	}
	t := lc.addTarget(lTarget{F: f, Zone: zStack, Spell: true, StackAb: true, AbOnly: true, CopyAb: kind})
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "CopyAbility", Obj: lRef{Kind: rTarget, N: t}, A: n, C: 1})
	return nil
}

// copyTriggeringAbility lowers an AbilityCast trigger's "copy that ability"
// (Ertha Jo, Frontier Mentor: Defined$ TriggeredSpellAbility, gorge
// effCopySpellAbility): Amount$ copies of the activated ability whose
// activation fired the trigger, as an ExtCopyAbility whose ability is the one
// the trigger recorded (kop.CopyAbTriggering). MayChooseTarget$ True lets the
// copy's controller choose new targets as the copy is about to resolve
// (CR 707.10c); anything else refuses.
func (lc *lowerCtx) copyTriggeringAbility(ps *params) error {
	if !ps.is("MayChooseTarget", "True") {
		return refuse("CopySpellAbility of the triggering ability without MayChooseTarget$ True")
	}
	if v, ok := ps.m["Controller"]; ok && strings.TrimSpace(v) == "You" {
		ps.consume("Controller")
	}
	n, err := lc.amount(ps, "Amount", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "CopyAbility", Obj: lRef{Kind: rTriggered}, A: n, C: 1})
	return nil
}
